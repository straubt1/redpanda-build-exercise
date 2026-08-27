# Model instruction prefixes

Static instructions for the Summarize → Classification loop. Compiled into the `reason` binary with `//go:embed` (see `internal/reason/embed.go`). Edit these files, then rebuild `reason`. They are **not** templates: no `{{`. JSON examples are literal.

Go still builds the evidence (changed files, truncated PR body, title last) and the parse-error line on classify retry.

| File | Loaded as | When |
| --- | --- | --- |
| `summarize.txt` | `summarizeInstructions` | Step 1 (`summarizePrompt`) |
| `classify.txt` | `classifyInstructions` | Step 2 (`classifyPrompt`) |
| `classify_repair.txt` | `classifyRepairInstructions` | Classification **retry only** (`repairSuffix`). Not a third Model step. |

## Assembly order (tests lock this)

1. **summarize:** `summarize.txt` → changed files → PR body → title last
2. **classify:** `classify.txt` → Summary fields (`affected_area`, `summary`) → changed files → PR body → title last
3. **classify retry:** that classify prompt + blank line + `Your previous output was invalid (...).` + `classify_repair.txt`

Not in these files: file list/patches, body truncation, title, Summary values, parse error text.
