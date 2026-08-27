package reason

import "context"

func summarize(ctx context.Context, llm Completer, in Input, eventID string) (Summary, int, error) {
	return runModel(ctx, llm, eventID, "summarize", summarizePrompt(in), ParseSummary, nil)
}
