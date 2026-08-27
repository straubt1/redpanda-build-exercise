package reason

import (
	"context"
	"fmt"
	"strings"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
)

// MaxAttempts is first try plus 2 retries (decisions.md).
const MaxAttempts = 3

const maxBodyChars = 8000

type Completer interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type FileInput struct {
	Filename string
	Status   string
	Patch    string
}

type Input struct {
	Title string
	Body  string
	Files []FileInput
}

type Outcome struct {
	Category        string
	Confidence      *float64
	Rationale       string
	AffectedArea    string
	Source          string
	Error           string
	ExtractRetries  int
	ClassifyRetries int
}

func fallback(err error, extractRetries, classifyRetries int, area string) Outcome {
	msg := "llm failed"
	if err != nil {
		msg = err.Error()
	}
	return Outcome{
		Category:        "unknown",
		Source:          "fallback",
		Rationale:       "llm parse failed after retries",
		AffectedArea:    area,
		Error:           msg,
		ExtractRetries:  extractRetries,
		ClassifyRetries: classifyRetries,
	}
}

func ClassifyPR(ctx context.Context, llm Completer, in Input, eventID string) Outcome {
	// Call LLM to extract the affected area - updates Outcome.AffectedArea
	ex, extractRetries, err := stepExtract(ctx, llm, in, eventID)
	if err != nil {
		return fallback(err, extractRetries, 0, "")
	}

	// Call LLM to classify the PR - updates Outcome.Category, Outcome.Confidence, Outcome.Rationale
	cl, classifyRetries, err := stepClassify(ctx, llm, in, ex, eventID)
	if err != nil {
		return fallback(err, extractRetries, classifyRetries, ex.AffectedArea)
	}

	conf := cl.Confidence
	return Outcome{
		Category:        cl.Category,
		Confidence:      &conf,
		Rationale:       cl.Rationale,
		AffectedArea:    ex.AffectedArea,
		Source:          "llm",
		ExtractRetries:  extractRetries,
		ClassifyRetries: classifyRetries,
	}
}

func stepExtract(ctx context.Context, llm Completer, in Input, eventID string) (Extraction, int, error) {
	prompt := extractPrompt(in)
	var last error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		applog.Info.Printf("llm step=extract event_id=%s attempt=%d/%d", eventID, attempt, MaxAttempts)
		raw, err := llm.Complete(ctx, prompt)
		if err != nil {
			last = err
			continue
		}
		ex, err := ParseExtraction(raw)
		if err != nil {
			last = err
			continue
		}
		return ex, attempt - 1, nil
	}
	return Extraction{}, MaxAttempts - 1, last
}

func stepClassify(ctx context.Context, llm Completer, in Input, ex Extraction, eventID string) (Classification, int, error) {
	prompt := classifyPrompt(in, ex)
	var last error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		applog.Info.Printf("llm step=classify event_id=%s attempt=%d/%d", eventID, attempt, MaxAttempts)
		use := prompt
		if attempt > 1 && last != nil {
			use = prompt + "\n\n" + repairSuffix(last)
		}
		raw, err := llm.Complete(ctx, use)
		if err != nil {
			last = err
			continue
		}
		cl, err := ParseClassification(raw)
		if err != nil {
			last = err
			continue
		}
		return cl, attempt - 1, nil
	}
	return Classification{}, MaxAttempts - 1, last
}

func repairSuffix(err error) string {
	return fmt.Sprintf(
		"Your previous output was invalid (%v). %s",
		err,
		strings.TrimSpace(classifyRepairInstructions),
	)
}

func extractPrompt(in Input) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(extractInstructions, "\n"))
	b.WriteString("\n\n")
	writeBodyFiles(&b, in)
	writeTitle(&b, in.Title)
	return b.String()
}

func classifyPrompt(in Input, ex Extraction) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(classifyInstructions, "\n"))
	b.WriteString("\n\n")
	b.WriteString("Extraction:\n")
	b.WriteString(fmt.Sprintf("affected_area: %s\nchange_summary: %s\n\n", ex.AffectedArea, ex.ChangeSummary))
	writeBodyFiles(&b, in)
	writeTitle(&b, in.Title)
	return b.String()
}

func writeBodyFiles(b *strings.Builder, in Input) {
	b.WriteString("Changed files:\n")
	if len(in.Files) == 0 {
		b.WriteString("(none)\n")
	}
	for _, f := range in.Files {
		b.WriteString(fmt.Sprintf("### %s (%s)\n", f.Filename, f.Status))
		if f.Patch != "" {
			b.WriteString(f.Patch)
			b.WriteString("\n")
		}
	}
	b.WriteString("\nPR body:\n")
	body := in.Body
	if len(body) > maxBodyChars {
		body = body[:maxBodyChars]
	}
	if strings.TrimSpace(body) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(body)
		b.WriteString("\n")
	}
}

func writeTitle(b *strings.Builder, title string) {
	b.WriteString("\nTitle (optional context only; do not decide from this alone): ")
	if strings.TrimSpace(title) == "" {
		b.WriteString("(none)\n")
		return
	}
	b.WriteString(title)
	b.WriteString("\n")
}
