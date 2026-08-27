package reason

import (
	"strings"
	"testing"
)

func sampleEvidence() Input {
	return Input{
		Title: "UNIQUE_TITLE_TOKEN",
		Body:  "UNIQUE_BODY_TOKEN",
		Files: []FileInput{
			{Filename: "a.go", Status: "modified", Patch: "UNIQUE_PATCH_A", Additions: 3, Deletions: 1, Changes: 4},
			{Filename: "b.go", Status: "added", Patch: "UNIQUE_PATCH_B", Additions: 2, Deletions: 0, Changes: 2},
		},
	}
}

func TestWriteEvidence_summarizeOmitsFilePatches(t *testing.T) {
	var b strings.Builder
	writeEvidence(&b, sampleEvidence(), false)
	got := b.String()
	assertTitleBodyThenFiles(t, got)
	if !strings.Contains(got, "additions: 5\ndeletions: 1\nchanges: 6\n") {
		t.Fatalf("want summed file stats: %q", got)
	}
	if strings.Contains(got, "#### ") || strings.Contains(got, "UNIQUE_PATCH_A") || strings.Contains(got, "UNIQUE_PATCH_B") {
		t.Fatalf("summarize Input must not include per-file patches: %q", got)
	}
}

func TestWriteEvidence_classifyIncludesFilePatches(t *testing.T) {
	var b strings.Builder
	writeEvidence(&b, sampleEvidence(), true)
	got := b.String()
	assertTitleBodyThenFiles(t, got)
	fileAAt := strings.Index(got, "#### a.go (modified)\n")
	fileBAt := strings.Index(got, "#### b.go (added)\n")
	filesAt := strings.Index(got, "### Changed Files:\n")
	if fileAAt < 0 || fileBAt < 0 || fileAAt < filesAt || fileBAt < fileAAt {
		t.Fatalf("classify Input must list files after totals: %q", got)
	}
	if !strings.Contains(got, "UNIQUE_PATCH_A") || !strings.Contains(got, "UNIQUE_PATCH_B") {
		t.Fatalf("classify Input must include patches: %q", got)
	}
}

func assertTitleBodyThenFiles(t *testing.T, got string) {
	t.Helper()
	inputAt := strings.Index(got, "## Input\n")
	titleAt := strings.Index(got, "Title:\n")
	bodyAt := strings.Index(got, "Body:\n")
	filesAt := strings.Index(got, "### Changed Files:\n")
	if inputAt < 0 || titleAt < 0 || bodyAt < 0 || filesAt < 0 {
		t.Fatalf("missing Input headings: %q", got)
	}
	if !(inputAt < titleAt && titleAt < bodyAt && bodyAt < filesAt) {
		t.Fatalf("want Title, Body, then Changed Files: %q", got)
	}
	if strings.Index(got, "UNIQUE_TITLE_TOKEN") > strings.Index(got, "UNIQUE_BODY_TOKEN") {
		t.Fatalf("title value must precede body: %q", got)
	}
}

func TestClassifyPrompt_summaryThenInput(t *testing.T) {
	got := classifyPrompt(
		Input{Title: "UNIQUE_TITLE_TOKEN", Body: "UNIQUE_BODY_TOKEN", Files: []FileInput{{Filename: "a.go", Status: "modified", Patch: "UNIQUE_PATCH_TOKEN"}}},
		Summary{AffectedArea: "UNIQUE_AREA_TOKEN", Summary: "UNIQUE_SUMMARY_TOKEN"},
	)
	sumAt := strings.Index(got, "## Summary\n")
	inputAt := strings.Index(got, "## Input\n")
	if sumAt < 0 || inputAt < 0 || sumAt > inputAt {
		t.Fatalf("## Summary must precede ## Input: %q", got)
	}
	ti, bi := strings.Index(got, "UNIQUE_TITLE_TOKEN"), strings.Index(got, "UNIQUE_BODY_TOKEN")
	pi := strings.Index(got, "UNIQUE_PATCH_TOKEN")
	areaAt, summaryAt := strings.Index(got, "UNIQUE_AREA_TOKEN"), strings.Index(got, "UNIQUE_SUMMARY_TOKEN")
	if ti < 0 || bi < 0 || pi < 0 || areaAt < 0 || summaryAt < 0 {
		t.Fatalf("missing injected fields: %q", got)
	}
	if !(sumAt < areaAt && areaAt < inputAt && inputAt < ti && ti < bi && bi < pi) {
		t.Fatalf("want Summary, then Input title/body/files: %q", got)
	}
}

func TestSummarizePrompt_omitsPatches(t *testing.T) {
	got := summarizePrompt(Input{
		Title: "UNIQUE_TITLE_TOKEN",
		Body:  "UNIQUE_BODY_TOKEN",
		Files: []FileInput{{Filename: "a.go", Status: "modified", Patch: "UNIQUE_PATCH_TOKEN", Additions: 1, Changes: 1}},
	})
	if strings.Contains(got, "UNIQUE_PATCH_TOKEN") || strings.Contains(got, "#### a.go") {
		t.Fatalf("summarize prompt must not include patches: %q", got)
	}
	if !strings.Contains(got, "additions: 1\ndeletions: 0\nchanges: 1\n") {
		t.Fatalf("summarize prompt must include totals: %q", got)
	}
}

func TestWriteEvidence_alwaysIncludesTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "nonempty", title: "UNIQUE_TITLE_TOKEN"},
		{name: "empty string", title: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			writeEvidence(&b, Input{
				Title: tt.title,
				Body:  "UNIQUE_BODY_TOKEN",
				Files: []FileInput{{Filename: "a.go", Status: "modified", Patch: "x"}},
			}, false)
			got := b.String()
			if strings.Contains(got, "optional") {
				t.Fatalf("title labeled optional: %q", got)
			}
			marker := "Title:\n"
			i := strings.Index(got, marker)
			if i < 0 {
				t.Fatalf("Title section missing: %q", got)
			}
			rest := got[i+len(marker):]
			line, _, ok := strings.Cut(rest, "\n")
			if !ok {
				t.Fatalf("title line missing newline: %q", got)
			}
			if line != tt.title {
				t.Fatalf("title value %q want %q in %q", line, tt.title, got)
			}
		})
	}
}

func TestWriteEvidence_fencesBody(t *testing.T) {
	var b strings.Builder
	writeEvidence(&b, Input{
		Title: "t",
		Body:  "## What\n\n```\nSEMSQL\n```\n",
	}, false)
	got := b.String()
	bodyAt := strings.Index(got, "Body:\n")
	filesAt := strings.Index(got, "### Changed Files:\n")
	if bodyAt < 0 || filesAt < 0 || bodyAt > filesAt {
		t.Fatalf("Body must precede Changed Files: %q", got)
	}
	section := got[bodyAt:filesAt]
	if !strings.Contains(section, "````\n## What\n\n```\nSEMSQL\n```\n\n````\n") {
		t.Fatalf("body must be fenced with a longer backtick run than nested fences: %q", section)
	}
}
