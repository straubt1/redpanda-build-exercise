# Testing

Offline fixture runner for Reason. No Kafka, GitHub, or Postgres. Same Rules then Models path as the live worker.

Each case is a folder under `tests/`:

```text
tests/<name>/
  message.json        # work-topic payload
  enrichment.json     # fetched PR title, body, files (no API call)
  results/            # written on each run (gitignored)
```

Included: `docs-only` and `lockfile-only` (Rules, no Ollama), `mixed-tiny` (Models, needs Ollama on the host).

## Run

```bash
# Taskfile
task reason:test DIR=tests/docs-only

# Without Taskfile
go run ./cmd/reason-test tests/docs-only
```

Models path (`tests/mixed-tiny`): Ollama at `http://127.0.0.1:11434` (`OLLAMA_MODEL`, default `llama3:8b`).

## Review after a run

Open these under `tests/<name>/results/`:

- **`outcome.json`** — category, source (`rule` / `model` / `fallback`), confidence, rationale, summary
- **`{event_id}/prompts/`** — assembled prompts and model replies (`summarize_attempt*.md`, `classify_attempt*.md`). Only when Rules miss.

## Not automated

This is for **manual** inspection of input vs output. Automating grading (golden labels, scoring, pass/fail) and feeding results back into prompt updates would take more work.
