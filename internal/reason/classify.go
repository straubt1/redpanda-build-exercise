package reason

import (
	"context"
	"fmt"
	"strings"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/debugdump"
)

type Completer interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type FileInput struct {
	Filename  string
	Status    string
	Patch     string
	Additions int
	Deletions int
	Changes   int
	Truncated bool
}

type Input struct {
	Title string
	Body  string
	Files []FileInput
}

func (in Input) empty() bool {
	return strings.TrimSpace(in.Body) == "" && len(in.Files) == 0
}

type Outcome struct {
	Category         string
	Confidence       *float64
	Rationale        string
	AffectedArea     string
	Summary          string
	Source           string
	Error            string
	SummarizeRetries int
	ClassifyRetries  int
}

func Classify(ctx context.Context, llm Completer, in Input, eventID string, minConfidence float64) Outcome {
	if in.empty() {
		return Outcome{Category: "unknown", Source: "fallback", Rationale: "empty body and no files"}
	}
	if out, ok := matchRules(in); ok {
		return out
	}
	return fromModels(ctx, llm, in, eventID, minConfidence)
}

func fromModels(ctx context.Context, llm Completer, in Input, eventID string, minConfidence float64) Outcome {
	sum, summarizeRetries, err := summarize(ctx, llm, in, eventID)
	if err != nil {
		return modelFallback(err, summarizeRetries, 0, Summary{})
	}
	cl, classifyRetries, err := classifyWithModel(ctx, llm, in, sum, eventID, minConfidence)
	if err != nil {
		return modelFallback(err, summarizeRetries, classifyRetries, sum)
	}
	conf := cl.Confidence
	return Outcome{
		Category:         cl.Category,
		Confidence:       &conf,
		Rationale:        cl.Rationale,
		AffectedArea:     sum.AffectedArea,
		Summary:          sum.Summary,
		Source:           "model",
		SummarizeRetries: summarizeRetries,
		ClassifyRetries:  classifyRetries,
	}
}

func classifyWithModel(ctx context.Context, llm Completer, in Input, sum Summary, eventID string, minConfidence float64) (Classification, int, error) {
	cl, retries, err := runModel(ctx, llm, eventID, "classify", classifyPrompt(in, sum), ParseClassification, repairSuffix)
	if err != nil {
		return cl, retries, err
	}
	if cl.Confidence >= minConfidence {
		return cl, retries, nil
	}
	return retryLowConfidence(ctx, llm, in, sum, eventID, cl, retries, minConfidence)
}

func retryLowConfidence(ctx context.Context, llm Completer, in Input, sum Summary, eventID string, first Classification, retries int, minConfidence float64) (Classification, int, error) {
	attempt := retries + 2
	retries++
	use := classifyPrompt(in, sum) + "\n\n" + lowConfidenceSuffix(first.Confidence, minConfidence)
	applog.Info.Printf("model step=classify event_id=%s attempt=%d confidence=%g below threshold=%g", eventID, attempt, first.Confidence, minConfidence)
	debugdump.Write(eventID, fmt.Sprintf("prompts/classify_attempt%d.md", attempt), []byte(use))
	raw, err := llm.Complete(ctx, use)
	if err != nil {
		debugdump.Write(eventID, fmt.Sprintf("prompts/classify_attempt%d_response.md", attempt), []byte(err.Error()))
		return first, retries, nil
	}
	debugdump.Write(eventID, fmt.Sprintf("prompts/classify_attempt%d_response.md", attempt), []byte(raw))
	cl, err := ParseClassification(raw)
	if err != nil {
		return first, retries, nil
	}
	return cl, retries, nil
}

func lowConfidenceSuffix(confidence, threshold float64) string {
	return fmt.Sprintf(
		"Your previous classification had confidence %g, which is below the %g threshold. Classify again from the same Summary and Input. Recalculate category and confidence from the evidence. A low confidence is valid; do not inflate the number to meet the threshold.",
		confidence, threshold,
	)
}

func modelFallback(err error, summarizeRetries, classifyRetries int, sum Summary) Outcome {
	msg := "model failed"
	if err != nil {
		msg = err.Error()
	}
	return Outcome{
		Category:         "unknown",
		Source:           "fallback",
		Rationale:        "model parse failed after retries",
		AffectedArea:     sum.AffectedArea,
		Summary:          sum.Summary,
		Error:            msg,
		SummarizeRetries: summarizeRetries,
		ClassifyRetries:  classifyRetries,
	}
}
