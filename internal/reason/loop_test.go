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

func TestClassifyPR_retriesThenSucceeds(t *testing.T) {
	llm := &scripted{replies: []string{
		"here is nothing useful",
		"still not json",
		`{"affected_area":"cli","change_summary":"adds a flag"}`,
		`{"category":"feature","confidence":0.8,"rationale":"new CLI flag"}`,
	}}
	got := ClassifyPR(context.Background(), llm, sampleInput(), "evt-1")
	if got.Source != "llm" || got.Category != "feature" {
		t.Fatalf("%+v", got)
	}
	if got.ExtractRetries != 2 || got.ClassifyRetries != 0 {
		t.Fatalf("retries extract=%d classify=%d", got.ExtractRetries, got.ClassifyRetries)
	}
	if llm.n != 4 {
		t.Fatalf("calls=%d want 4", llm.n)
	}
	if got.Confidence == nil || *got.Confidence != 0.8 {
		t.Fatalf("confidence %+v", got.Confidence)
	}
}

func TestClassifyPR_extractCapStopsWithoutClassify(t *testing.T) {
	llm := &scripted{replies: []string{"nope", "nope", "nope", `{"category":"feature","confidence":1,"rationale":"should not run"}`}}
	got := ClassifyPR(context.Background(), llm, sampleInput(), "evt-2")
	if got.Source != "fallback" || got.Category != "unknown" {
		t.Fatalf("%+v", got)
	}
	if llm.n != MaxAttempts {
		t.Fatalf("calls=%d want %d (must not spin)", llm.n, MaxAttempts)
	}
	if got.ExtractRetries != MaxAttempts-1 {
		t.Fatalf("extract retries=%d", got.ExtractRetries)
	}
}

func TestClassifyPR_invalidCategoryRepairThenOK(t *testing.T) {
	rec := &recording{}
	rec.replies = []string{
		`{"affected_area":"auth","change_summary":"middleware"}`,
		`{"category":"not-a-label","confidence":0.9,"rationale":"x"}`,
		`{"category":"security","confidence":0.7,"rationale":"auth check"}`,
	}
	got := ClassifyPR(context.Background(), rec, sampleInput(), "evt-3")
	if got.Category != "security" || got.Source != "llm" {
		t.Fatalf("%+v", got)
	}
	if got.ClassifyRetries != 1 {
		t.Fatalf("classify retries=%d", got.ClassifyRetries)
	}
	if len(rec.prompts) < 3 || !strings.Contains(rec.prompts[2], "must be one of") {
		t.Fatalf("repair prompt missing: %#v", rec.prompts)
	}
}

func TestClassifyPR_promptPutsTitleAfterBodyAndFiles(t *testing.T) {
	rec := &recording{replies: []string{
		`{"affected_area":"x","change_summary":"y"}`,
		`{"category":"refactor","confidence":0.4,"rationale":"rename"}`,
	}}
	in := Input{Title: "UNIQUE_TITLE_TOKEN", Body: "UNIQUE_BODY_TOKEN", Files: []FileInput{{Filename: "a.go", Status: "modified", Patch: "UNIQUE_PATCH_TOKEN"}}}
	got := ClassifyPR(context.Background(), rec, in, "evt-4")
	if got.Category != "refactor" {
		t.Fatalf("%+v", got)
	}
	if got.Confidence == nil || *got.Confidence != 0.4 {
		t.Fatalf("low confidence must still persist, got %+v", got.Confidence)
	}
	p := rec.prompts[0]
	bi, ti := strings.Index(p, "UNIQUE_BODY_TOKEN"), strings.Index(p, "UNIQUE_TITLE_TOKEN")
	pi := strings.Index(p, "UNIQUE_PATCH_TOKEN")
	if pi < 0 || bi < 0 || ti < 0 || !(pi < ti && bi < ti) {
		t.Fatalf("title must come after body and files in extract prompt")
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
