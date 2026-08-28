# Model instruction prefixes

Static instructions for the Summarize → Classification loop. Compiled into the `reason` binary with `//go:embed` (see `internal/reason/embed.go`). Edit these files, then rebuild `reason`. They are **not** templates: no `{{`. JSON examples are literal.

Go still builds `## Summary` (classify only), the `## Input` section (title, fenced body, then changed-file totals — title always present, may be empty; classify also gets per-file patches), the parse-error line on classify retry, and a low-confidence suffix when Go retries Classification. Body is wrapped in a markdown fence (backtick run longer than any run in the truncated body). A cut patch is marked `[truncated]` on the file heading.

| File | Loaded as | When |
| --- | --- | --- |
| `summarize.txt` | `summarizeInstructions` | Step 1 (`summarizePrompt`) |
| `classify.txt` | `classifyInstructions` | Step 2 (`classifyPrompt`) |
| `classify_repair.txt` | `classifyRepairInstructions` | Classification **parse retry** (`repairSuffix`). Not a third Model step. Low-confidence retry is a Go suffix, not this file. |

## Assembly order (tests lock this)

1. **summarize:** `summarize.txt` → `## Input` → title → body → changed-file totals (no per-file patches)
2. **classify:** `classify.txt` → `## Summary` → `## Input` → title → body → changed-file totals + per-file patches
3. **classify retry (invalid JSON):** that classify prompt + blank line + `Your previous output was invalid (...).` + `classify_repair.txt`
4. **classify retry (confidence below threshold):** that classify prompt + blank line + previous score vs threshold (Go; not a prompt file). One extra attempt; persist even if still low.

Not in these files: file list/patches, body truncation, title, Summary values, parse error text.
