package reason

import "testing"

func TestMatch(t *testing.T) {
	rules := defaultRules()

	tests := []struct {
		name     string
		files    []string
		wantCat  string
		wantSkip bool
	}{
		{
			name:     "all markdown",
			files:    []string{"README.md", "docs/guide.md"},
			wantCat:  "docs",
			wantSkip: true,
		},
		{
			name:     "all lockfiles",
			files:    []string{"go.sum", "frontend/package-lock.json"},
			wantCat:  "dependency-bump",
			wantSkip: true,
		},
		{
			name:     "mixed md and go",
			files:    []string{"README.md", "main.go"},
			wantSkip: false,
		},
		{
			name:     "empty file list does not skip",
			files:    []string{},
			wantSkip: false,
		},
		{
			name:     "nil file list does not skip",
			files:    nil,
			wantSkip: false,
		},
		{
			name:     "mdx is not markdown skip",
			files:    []string{"page.mdx"},
			wantSkip: false,
		},
		{
			name:     "go.mod alone is not lockfile-only",
			files:    []string{"go.mod"},
			wantSkip: false,
		},
		{
			name:     "lockfile plus code does not skip",
			files:    []string{"go.sum", "main.go"},
			wantSkip: false,
		},
		{
			name:     "markdown plus lockfile does not skip",
			files:    []string{"README.md", "yarn.lock"},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := match(rules, tt.files)
			if ok != tt.wantSkip {
				t.Fatalf("skip=%v want %v (category=%q)", ok, tt.wantSkip, got)
			}
			if tt.wantSkip && got != tt.wantCat {
				t.Fatalf("category=%q want %q", got, tt.wantCat)
			}
		})
	}
}
