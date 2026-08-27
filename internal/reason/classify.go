package reason

import (
	"context"
	"strings"
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

func Classify(ctx context.Context, llm Completer, in Input, eventID string) Outcome {
	if in.empty() {
		return Outcome{Category: "unknown", Source: "fallback", Rationale: "empty body and no files"}
	}
	if out, ok := matchRules(in); ok {
		return out
	}
	return fromModels(ctx, llm, in, eventID)
}

func fromModels(ctx context.Context, llm Completer, in Input, eventID string) Outcome {
	sum, summarizeRetries, err := summarize(ctx, llm, in, eventID)
	if err != nil {
		return modelFallback(err, summarizeRetries, 0, Summary{})
	}
	cl, classifyRetries, err := classifyWithModel(ctx, llm, in, sum, eventID)
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

func classifyWithModel(ctx context.Context, llm Completer, in Input, sum Summary, eventID string) (Classification, int, error) {
	return runModel(ctx, llm, eventID, "classify", classifyPrompt(in, sum), ParseClassification, repairSuffix)
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
