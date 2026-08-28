package reason

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type scripted struct {
	replies []string
	n       int
}

func (s *scripted) Complete(_ context.Context, prompt string) (string, error) {
	i := s.n
	s.n++
	if i >= len(s.replies) {
		return "not json", nil
	}
	if strings.HasPrefix(s.replies[i], "ERR:") {
		return "", fmt.Errorf("%s", strings.TrimPrefix(s.replies[i], "ERR:"))
	}
	_ = prompt
	return s.replies[i], nil
}

func TestClassify_retriesThenSucceeds(t *testing.T) {
	llm := &scripted{replies: []string{
		"here is nothing useful",
		"still not json",
		`{"affected_area":"cli","summary":"adds a flag"}`,
		`{"category":"feature","confidence":0.8,"rationale":"new CLI flag"}`,
	}}
	got := Classify(context.Background(), llm, sampleInput(), "evt-1", 0.6)
	if got.Source != "model" || got.Category != "feature" {
		t.Fatalf("%+v", got)
	}
	if got.SummarizeRetries != 2 || got.ClassifyRetries != 0 {
		t.Fatalf("retries summarize=%d classify=%d", got.SummarizeRetries, got.ClassifyRetries)
	}
	if got.Summary != "adds a flag" || got.AffectedArea != "cli" {
		t.Fatalf("summary %+v", got)
	}
	if llm.n != 4 {
		t.Fatalf("calls=%d want 4", llm.n)
	}
	if got.Confidence == nil || *got.Confidence != 0.8 {
		t.Fatalf("confidence %+v", got.Confidence)
	}
}

func TestClassify_summarizeCapStopsWithoutClassify(t *testing.T) {
	llm := &scripted{replies: []string{"nope", "nope", "nope", `{"category":"feature","confidence":1,"rationale":"should not run"}`}}
	got := Classify(context.Background(), llm, sampleInput(), "evt-2", 0.6)
	if got.Source != "fallback" || got.Category != "unknown" {
		t.Fatalf("%+v", got)
	}
	if llm.n != MaxAttempts {
		t.Fatalf("calls=%d want %d (must not spin)", llm.n, MaxAttempts)
	}
	if got.SummarizeRetries != MaxAttempts-1 {
		t.Fatalf("summarize retries=%d", got.SummarizeRetries)
	}
}

func TestClassify_invalidCategoryRepairThenOK(t *testing.T) {
	rec := &recording{}
	rec.replies = []string{
		`{"affected_area":"auth","summary":"middleware"}`,
		`{"category":"not-a-label","confidence":0.9,"rationale":"x"}`,
		`{"category":"security","confidence":0.7,"rationale":"auth check"}`,
	}
	got := Classify(context.Background(), rec, sampleInput(), "evt-3", 0.6)
	if got.Category != "security" || got.Source != "model" {
		t.Fatalf("%+v", got)
	}
	if got.ClassifyRetries != 1 {
		t.Fatalf("classify retries=%d", got.ClassifyRetries)
	}
	if len(rec.prompts) < 3 || !strings.Contains(rec.prompts[2], "must be one of") {
		t.Fatalf("repair prompt missing: %#v", rec.prompts)
	}
}

func TestClassify_promptPutsTitleThenBodyThenFiles(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"x","summary":"y"}`,
		`{"category":"refactor","confidence":0.4,"rationale":"rename"}`,
	}}
	in := Input{Title: "UNIQUE_TITLE_TOKEN", Body: "UNIQUE_BODY_TOKEN", Files: []FileInput{{Filename: "a.go", Status: "modified", Patch: "UNIQUE_PATCH_TOKEN"}}}
	got := Classify(context.Background(), rec, in, "evt-4", 0)
	if got.Category != "refactor" {
		t.Fatalf("%+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.4 {
		t.Fatalf("low confidence must still persist, got %+v", got.Confidence)
	}
	p := rec.prompts[0]
	ti, bi := strings.Index(p, "UNIQUE_TITLE_TOKEN"), strings.Index(p, "UNIQUE_BODY_TOKEN")
	if bi < 0 || ti < 0 || !(ti < bi) {
		t.Fatalf("summarize Input must be title, then body")
	}
	if strings.Contains(p, "UNIQUE_PATCH_TOKEN") {
		t.Fatalf("summarize Input must not include patches")
	}
	if len(rec.prompts) < 2 {
		t.Fatalf("expected classify prompt, got %d calls", len(rec.prompts))
	}
	cp := rec.prompts[1]
	if strings.Index(cp, "## Summary\n") < 0 || strings.Index(cp, "## Summary\n") > strings.Index(cp, "## Input\n") {
		t.Fatalf("classify must put ## Summary before ## Input")
	}
	cpi := strings.Index(cp, "UNIQUE_PATCH_TOKEN")
	if cpi < 0 || cpi < strings.Index(cp, "UNIQUE_BODY_TOKEN") {
		t.Fatalf("classify Input must include patches after body")
	}
}

func TestClassify_ruleSkipsModel(t *testing.T) {
	llm := &scripted{replies: []string{`{"category":"feature","confidence":1,"rationale":"should not run"}`}}
	in := Input{
		Title: "docs",
		Body:  "update readme",
		Files: []FileInput{{Filename: "README.md", Status: "modified"}},
	}
	got := Classify(context.Background(), llm, in, "evt-rule", 0.6)
	if got.Source != "rule" || got.Category != "docs" {
		t.Fatalf("%+v", got)
	}
	if llm.n != 0 {
		t.Fatalf("model called %d times", llm.n)
	}
}

func TestClassify_emptyContentSkipsModel(t *testing.T) {
	llm := &scripted{replies: []string{`{"category":"feature","confidence":1,"rationale":"should not run"}`}}
	got := Classify(context.Background(), llm, Input{Title: "only a title"}, "evt-empty", 0.6)
	if got.Source != "fallback" || got.Category != "unknown" {
		t.Fatalf("%+v", got)
	}
	if llm.n != 0 {
		t.Fatalf("model called %d times", llm.n)
	}
}

func TestClassify_retriesWhenConfidenceBelowThreshold(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"cli","summary":"adds a flag"}`,
		`{"category":"feature","confidence":0.4,"rationale":"maybe a flag"}`,
		`{"category":"feature","confidence":0.8,"rationale":"new CLI flag in main.go"}`,
	}}
	got := Classify(context.Background(), rec, sampleInput(), "evt-conf-retry", 0.6)
	if got.Source != "model" || got.Category != "feature" {
		t.Fatalf("%+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.8 {
		t.Fatalf("want second-attempt confidence 0.8, got %+v", got.Confidence)
	}
	if got.ClassifyRetries != 1 {
		t.Fatalf("classify retries=%d", got.ClassifyRetries)
	}
	if rec.n != 3 {
		t.Fatalf("calls=%d want 3", rec.n)
	}
	if len(rec.prompts) < 3 || !strings.Contains(rec.prompts[2], "below the 0.6 threshold") {
		t.Fatalf("low-confidence suffix missing: %#v", rec.prompts)
	}
	if strings.Contains(rec.prompts[1], "below the 0.6 threshold") {
		t.Fatalf("first classify prompt must not include the threshold")
	}
}

func TestClassify_doesNotRetryWhenConfidenceMeetsThreshold(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"cli","summary":"adds a flag"}`,
		`{"category":"feature","confidence":0.6,"rationale":"new CLI flag"}`,
		`{"category":"security","confidence":0.9,"rationale":"should not run"}`,
	}}
	got := Classify(context.Background(), rec, sampleInput(), "evt-conf-ok", 0.6)
	if got.Category != "feature" || got.Source != "model" {
		t.Fatalf("%+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.6 {
		t.Fatalf("confidence %+v", got.Confidence)
	}
	if rec.n != 2 {
		t.Fatalf("calls=%d want 2 (must not retry at threshold)", rec.n)
	}
	if got.ClassifyRetries != 0 {
		t.Fatalf("classify retries=%d", got.ClassifyRetries)
	}
}

func TestClassify_keepsFirstWhenLowConfidenceRetryFailsParse(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"x","summary":"y"}`,
		`{"category":"refactor","confidence":0.4,"rationale":"rename"}`,
		`not json`,
	}}
	got := Classify(context.Background(), rec, sampleInput(), "evt-conf-keep", 0.6)
	if got.Source != "model" || got.Category != "refactor" {
		t.Fatalf("must keep first valid classify, got %+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.4 {
		t.Fatalf("confidence %+v", got.Confidence)
	}
	if rec.n != 3 {
		t.Fatalf("calls=%d want 3", rec.n)
	}
}

func TestClassify_persistsStillLowAfterRetry(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"x","summary":"y"}`,
		`{"category":"unknown","confidence":0.3,"rationale":"mixed"}`,
		`{"category":"unknown","confidence":0.2,"rationale":"still mixed"}`,
	}}
	got := Classify(context.Background(), rec, sampleInput(), "evt-conf-still-low", 0.6)
	if got.Source != "model" || got.Category != "unknown" {
		t.Fatalf("low confidence must still persist as model, got %+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.2 {
		t.Fatalf("want last valid confidence 0.2, got %+v", got.Confidence)
	}
}

type recording struct {
	replies []string
	prompts []string
	n       int
}

func (r *recording) Complete(_ context.Context, prompt string) (string, error) {
	r.prompts = append(r.prompts, prompt)
	i := r.n
	r.n++
	if i >= len(r.replies) {
		return "not json", nil
	}
	return r.replies[i], nil
}

func sampleInput() Input {
	return Input{
		Title: "ignore me",
		Body:  "adds a helper",
		Files: []FileInput{{Filename: "main.go", Status: "modified", Patch: "+func Foo() {}"}},
	}
}
