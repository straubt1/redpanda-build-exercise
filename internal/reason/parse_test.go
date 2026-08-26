package reason

import (
	"strings"
	"testing"
)

func TestParseClassification_dirtyAndNormalize(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantCat string
		wantErr bool
	}{
		{
			name: "markdown fence",
			raw: "```json\n" +
				`{"category": "docs", "confidence": 0.9, "rationale": "readme only"}` +
				"\n```\n",
			wantCat: "docs",
		},
		{
			name:    "leading prose",
			raw:     "here is json {\"category\": \"feature\", \"confidence\": 0.8, \"rationale\": \"new endpoint\"} thanks",
			wantCat: "feature",
		},
		{
			name: "trailing commentary and second braces",
			raw: `Sure, I classified it:
{"category": "refactor", "confidence": 0.7, "rationale": "renames helpers"}
hope that helps {not json}`,
			wantCat: "refactor",
		},
		{
			name:    "title case Feature",
			raw:     `{"category": "Feature", "confidence": 0.6, "rationale": "adds UI"}`,
			wantCat: "feature",
		},
		{
			name:    "hyphenate spaces",
			raw:     `{"category": "Dependency Bump", "confidence": 0.5, "rationale": "lockfile"}`,
			wantCat: "dependency-bump",
		},
		{
			name:    "braces inside rationale string",
			raw:     `{"category": "security", "confidence": 0.95, "rationale": "checks {auth} headers"}`,
			wantCat: "security",
		},
		{
			name:    "model unknown is a valid label",
			raw:     `{"category": "unknown", "confidence": 0.2, "rationale": "unclear"}`,
			wantCat: "unknown",
		},
		{
			name:    "wrong enum is a parse error",
			raw:     `{"category": "not-a-label", "confidence": 0.9, "rationale": "x"}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "empty category field",
			raw:     `{"category": "", "confidence": 0.9, "rationale": "x"}`,
			wantErr: true,
		},
		{
			name:    "missing category",
			raw:     `{"confidence": 0.9, "rationale": "x"}`,
			wantErr: true,
		},
		{
			name:    "no object",
			raw:     "I cannot produce JSON today.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClassification(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Category != tt.wantCat {
				t.Fatalf("category=%q want %q", got.Category, tt.wantCat)
			}
		})
	}
}

func TestFirstObject_doesNotUseLastBrace(t *testing.T) {
	raw := `prefix {"category":"docs","confidence":1,"rationale":"ok"} trailing {oops}`
	obj, err := FirstObject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(obj, `{"category":"docs"`) {
		t.Fatalf("first object = %q", obj)
	}
	if strings.Contains(obj, "oops") {
		t.Fatalf("included trailing braces: %q", obj)
	}
}

func TestParseExtraction(t *testing.T) {
	got, err := ParseExtraction("here: {\"affected_area\": \"auth\", \"change_summary\": \"adds middleware\"} done")
	if err != nil {
		t.Fatal(err)
	}
	if got.AffectedArea != "auth" || got.ChangeSummary != "adds middleware" {
		t.Fatalf("%+v", got)
	}
}
