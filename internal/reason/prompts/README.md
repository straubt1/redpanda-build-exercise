# LLM instruction prefixes

Static instructions for the extract → classify loop. Compiled into the `reason` binary with `//go:embed` (see `internal/reason/embed.go`). Edit these files, then rebuild `reason`. They are **not** templates: no `{{`. JSON examples are literal.

Go still builds the evidence (changed files, truncated PR body, title last) and the parse-error line on classify retry.

| File | Loaded as | When |
| --- | --- | --- |
| `extract.txt` | `extractInstructions` | Step 1 (`extractPrompt`) |
| `classify.txt` | `classifyInstructions` | Step 2 (`classifyPrompt`) |
| `classify_repair.txt` | `classifyRepairInstructions` | Classify **retry only** (`repairSuffix`). Not a third model step. |

## Assembly order (tests lock this)

1. **extract:** `extract.txt` → changed files → PR body → title last
2. **classify:** `classify.txt` → extraction fields (`affected_area`, `change_summary`) → changed files → PR body → title last
3. **classify retry:** that classify prompt + blank line + `Your previous output was invalid (...).` + `classify_repair.txt`

Not in these files: file list/patches, body truncation, title, extraction values, parse error text.
