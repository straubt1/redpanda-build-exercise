package reason

import (
	"context"
	"fmt"
	"strings"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
)

// MaxAttempts is first try plus 2 retries (decisions.md).
const MaxAttempts = 3

func runModel[T any](
	ctx context.Context,
	llm Completer,
	eventID, step, prompt string,
	parse func(string) (T, error),
	repair func(error) string,
) (T, int, error) {
	var zero T
	var last error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		applog.Info.Printf("model step=%s event_id=%s attempt=%d/%d", step, eventID, attempt, MaxAttempts)
		use := prompt
		if attempt > 1 && last != nil && repair != nil {
			use = prompt + "\n\n" + repair(last)
		}
		raw, err := llm.Complete(ctx, use)
		if err != nil {
			last = err
			continue
		}
		got, err := parse(raw)
		if err != nil {
			last = err
			continue
		}
		return got, attempt - 1, nil
	}
	return zero, MaxAttempts - 1, last
}

func repairSuffix(err error) string {
	return fmt.Sprintf(
		"Your previous output was invalid (%v). %s",
		err,
		strings.TrimSpace(classifyRepairInstructions),
	)
}
