# Architecture

Rebuild this shape. Names use working defaults from `decisions.md`.

## End to end

```text
GET https://api.github.com/events
        │
        ▼
┌──────────────────────────────────────┐
│ CONNECT                              │
│  generate 30s → GET /events (multi-page) │
│  explode JSON array → one msg/event  │
│  filter PR + action allowlist        │
│  project small schema                │
│  cache by event_id                   │
│  produce                             │
└──────────────────────────────────────┘
        │
        ▼
  topic github.pr.opened
        │
        ▼
┌──────────────────────────────────────┐
│ GO REASON (compose: reason)          │
│  consume                             │
│  GET pull + GET files                │
│  Rules or Summarize→Classification   │
│  parse / retry / unknown             │
│  upsert Postgres                     │
└──────────────────────────────────────┘
        │
        ▼
  postgres table pr_triages
        │
        ▼
┌──────────────────────────────────────┐
│ GO SERVE (compose: serve)            │
│  GET /  HTML table (reference CSS)   │
│  GET /api/triages  JSON              │
│  GET /healthz                        │
└──────────────────────────────────────┘
        │
        ▼
  browser  GET /
```

## Connect

**Job:** plumbing. Rules only.

```text
 generate every 30s → multiple pages
        │
        ▼
 GET /events?per_page=100&page=N  (headers; rate-limit the sweep)
        │
        ▼
 raw JSON array per page (~30–100 events, mixed types)
        │
        ▼
 unarchive / explode to one event per message
        │
        ▼
 filter: type == PullRequestEvent
         AND action in allowlist  ["opened"]
        │
        ▼
 project: event_id, repo, pr_number, pr_url,
          author, action, created_at
        │
        ▼
 cap N per sweep (CONNECT_BATCH_LIMIT, default 1)
        │
        ▼
 cache key = event_id  (seen → drop)
        │
        ▼
 produce github.pr.opened
```

### GitHub events (implementer notes)

- Docs: [GitHub REST Events](https://docs.github.com/en/rest/activity/events) and [PullRequestEvent](https://docs.github.com/en/rest/using-the-rest-api/github-event-types#pullrequestevent).
- Response is an **array**. If Connect leaves it as one blob, filter/cache/produce will be wrong. Explode first.
- Headers: `Authorization: Bearer ${GITHUB_TOKEN}`, `User-Agent` (non-empty; GitHub rejects empty), `Accept: application/vnd.github+json`.
- Rate limit: 60/hr anonymous, 5000/hr with token. Connect issues **multiple** GETs per 30s sweep so `/events` returns more of the window. Filter **before** the worker so most events never cause extra GETs.
- `/events` is a poll source: the same `id` reappears. Cache is mandatory given our decision.
- Do not use Connect `branch` + `ollama_chat` / `openai_chat_completion` for this exercise. The hint in the takehome about `branch` for LLM calls is for a different topology; we rejected it.

### Work topic message (JSON)

`/events` PullRequestEvent payloads are truncated: **no `title`**, and often no `html_url`. Do not map title in Connect. The worker fills `title` from `GET /repos/{repo}/pulls/{pr_number}`.

`author` / `created_at` may be empty strings if missing.

```json
{
  "event_id": "18976100919",
  "repo": "owner/name",
  "pr_number": 42,
  "pr_url": "https://github.com/owner/name/pull/42",
  "author": "login",
  "action": "opened",
  "created_at": "2026-08-26T17:06:33Z"
}
```

Mapping (verified against a live `/events` PullRequestEvent):

| Output | Path |
| --- | --- |
| `event_id` | `.id` |
| `repo` | `.repo.name` |
| `pr_number` | `.payload.pull_request.number` (or `.payload.number`) |
| `pr_url` | `.payload.pull_request.html_url` if present, else `.payload.pull_request.url`, else construct `https://github.com/{repo}/pull/{n}` |
| `author` | `.actor.login` or `.payload.pull_request.user.login` |
| `action` | `.payload.action` |
| `created_at` | `.created_at` (already ISO-8601; do not pass raw epoch into Postgres) |

Bloblang: a `mapping` rebuilds `root` from scratch. Start with `root = this` or set all fields in one mapping. Use `.catch(deleted())` on JSON parse if needed so one bad record does not poison the pipeline.

Cache: Connect `cache` resource, key `event_id`. Memory is fine for local demo (lost on restart → possible reprocess; upsert makes that safe).

## Reason (Go)

**Job:** fetch what the feed omitted, classify with a Rule when that is enough, otherwise run Models, persist a valid row.

```text
 consume github.pr.opened
        │
        ▼
 GET /repos/{repo}/pulls/{pr_number}
 GET /repos/{repo}/pulls/{pr_number}/files
        │
        ▼
 files empty AND body empty? ──yes──► upsert category=unknown, source=fallback
        │
       no
        │
        ▼
 Rules (first match wins)
   all paths end with .md     → docs, source=rule
   all paths in lockfile set  → dependency-bump, source=rule
        │
     no classification
        │
        ▼
 STEP 1 Summarize (Model): affected area + summary
        parse; retry; else unknown + stop
        │
        ▼
 STEP 2 Classification (Model): category + confidence + rationale
        using Summary + body + files (truncated)
        parse; retry; else unknown
        │
        ▼
 upsert pr_triages (PK event_id)
```

### Enrichment APIs

Same token + User-Agent as Connect.

- Pull: `GET https://api.github.com/repos/{owner}/{repo}/pulls/{number}`
- Files: `GET https://api.github.com/repos/{owner}/{repo}/pulls/{number}/files`

Respect file budget from `decisions.md`. Always keep `filename`, `status` (added/modified/removed), and `additions`/`deletions`/`changes`. Patches are optional evidence for the model; filenames alone are not enough for the “read the change” requirement when a patch exists — send truncated patch when present, and mark that file heading `[truncated]`.

404 / 403: upsert `unknown`, store error, do not crash the consumer.

### Rules

Implement as a list of functions `(files) -> (category, ok)` so more rules can be added later without touching the loop. A Rule either classifies or does not.

- Zero files: do not match; fall through to “not enough content” / Model / `unknown` as in the flow.
- All `.md`: includes `README.md`, `docs/foo.md`. A single `foo.mdx` or `foo.go` means **no** Rule hit.
- All lockfiles: names in `decisions.md`. Case-sensitive as GitHub returns them.

When a Rule fires: still persist repo, pr, title, url, file list summary if the schema allows. `confidence` may be null. `rationale` can be a short static string (“all changed files are markdown”).

### Model I/O

Instruction prefixes are files under `internal/reason/prompts/` (`summarize.txt`, `classify.txt`, `classify_repair.txt` on classify retry). `reason` embeds them at compile time. Go appends `## Summary` (classify only), then a `## Input` section: title, truncated body inside a markdown fence, then changed-file totals. Classification also gets per-file patches; Summarize does not. See `internal/reason/prompts/README.md`.

**Summarize** must produce structured JSON, e.g. `{ "affected_area": string, "summary": string }`. Exact keys may vary; parse must be tolerant.

**Classification** must produce `{ "category": string, "confidence": number, "rationale": string }`.

`category` normalized: trim, lowercase, hyphenate spaces if needed. If not in the enum → retry once with a “label must be one of …” repair prompt, then `unknown`.

Do not classify from title alone in the prompt. Title is always included in `## Input` first (the string may be empty), then body, then files.

### Parse (must be unit-tested)

Dirty local-model output is expected.

1. Extract the first JSON object `{...}` from the text (brace matching or first `{` to last `}` of the first block).
2. `json.Unmarshal`.
3. Normalize `category`.
4. On failure: retry the **same step** up to the retry budget, then `unknown`.

This is the logic the exercise wants tested. A test that only checks a happy-path struct is not enough.

### Consumer and failures

- Process one message at a time in v1 (simpler to defend).
- Do not commit/ack until upsert succeeds (or a deliberate dead-letter — v1: log + `unknown` row if the failure is “model/parse”; retry Kafka if the failure is Postgres down).
- **Assumption:** GitHub/Ollama errors after retries → row with `unknown` + `error` text, then ack (avoid poison-message loops). Postgres errors → do not ack.

## Postgres

Minimum schema (add columns only if a phase needs them):

```sql
CREATE TABLE IF NOT EXISTS pr_triages (
  event_id        TEXT PRIMARY KEY,
  repo            TEXT NOT NULL,
  pr_number       INTEGER NOT NULL,
  title           TEXT NOT NULL DEFAULT '',
  pr_url          TEXT NOT NULL DEFAULT '',
  author          TEXT NOT NULL DEFAULT '',
  action          TEXT NOT NULL DEFAULT 'opened',
  category        TEXT NOT NULL,
  confidence      DOUBLE PRECISION,
  rationale       TEXT NOT NULL DEFAULT '',
  affected_area   TEXT NOT NULL DEFAULT '',
  summary         TEXT NOT NULL DEFAULT '',
  source          TEXT NOT NULL DEFAULT 'unknown', -- rule | model | fallback
  error           TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ,
  classified_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pr_triages_classified_at ON pr_triages (classified_at DESC);
```

`category` check constraint optional; application enum is source of truth.

Upsert: `INSERT ... ON CONFLICT (event_id) DO UPDATE`.

`created_at`: parse topic `created_at` ISO string into `TIMESTAMPTZ`. Never insert a raw epoch number.

## Serve

Compose service **`serve`**. Separate binary (`cmd/serve`). Reads Postgres only — does not consume Kafka or call Ollama.

v1, `localhost:8080`:

- `GET /` — HTML table of `pr_triages`. Default newest first (`classified_at` desc).
- `GET /api/triages` — JSON of the **same** rows (same filters/sort) plus `stats`. `Content-Type: application/json`.
- `GET /healthz` — 200 if Postgres ping succeeds.

Show: when, repo, PR number (link `pr_url`) with title under it, category, confidence, source, affected area, summary, rationale.

**List cap:** inner query takes the **20** newest by `classified_at`; outer query applies `sort`/`dir`. Newest rows are always in the set.

**Stats** (all rows, not just the 20): total; not reasoned (`source` not `model` or `rule`); count `source=model`; count `source=rule`. Shown above the table.

**When:** ISO timestamp in `<time datetime>`; a few lines of inline JS format the browser-local clock (`1:30 pm`) and set `title` to the full local datetime with seconds. No JS framework.

**CSS:** Inline styles matching `.reference/index.html` (system UI font, `#f7f8f5` page, white table, `#0b5` links). No Pico, no SPA, no JS framework.

**Sort:** query params `sort` and `dir` (`asc`|`desc`). Column headers on `GET /` are links that toggle dir. Allowlist `sort` to real columns (e.g. `classified_at`, `category`, `repo`, `title`, `confidence`, `source`). Reject unknown `sort` with 400 on the API; HTML falls back to default. HTML and JSON share one list/query function.

Empty table: visible empty state, not a 500.

## Reason

Compose service **`reason`**. Binary `cmd/reason`. Consumes `github.pr.opened`, fetches GitHub, Rules or Models, upserts `pr_triages`. No host port. Needs Kafka, Postgres, `GITHUB_TOKEN`, host Ollama at `host.docker.internal:11434`. Always-on debug dumps under `/logs/{event_id}/` (kafka message, enrichment, prompts, Postgres row); fail-open. Compose bind-mounts `.local/reason-logs:/logs`.

## Compose (target for the last infra phase)

Services: `redpanda`, `connect`, `postgres`, **`reason`**, **`serve`**. Local inspect (Compose profile `debug`): Console `:8081`, pgAdmin `:8082` (not the product UI). **Ollama is not a Compose service** — it runs on the host (`task ollama:up`). **`reason`** calls `host.docker.internal:11434`.

- Redpanda: `--mode=dev-container`.
- **`reason`** waits for broker + postgres. Host model: `ollama pull` of `OLLAMA_MODEL`.
- **`serve`** waits for postgres; publishes host **8080**. No Kafka, GitHub, or Ollama.
- `.env` gitignored. `.env.example` lists `GITHUB_TOKEN` and any future LLM keys (empty).
- Connect gets the token from env. Do not bake secrets into YAML.

Developer loop (Taskfile, how to dump `/events`, how to consume topics): [devloop.md](devloop.md). `task up` may wrap Compose; `docker compose up` must still work for the takehome.

## Seams for the pre-walkthrough extension

Keep these as obvious lists/functions, not scattered `if`s:

1. Action allowlist (Connect).
2. Rule list (Go).
3. Category enum + normalize (Go).
4. After-classify hook: e.g. if `security` and confidence high, produce an extra topic later.
5. Topic JSON + table columns: adding a field should touch project mapping, struct, upsert, HTML **and** `/api/triages` — in obvious places.

## What not to build

- Connect SQL sink of classifications
- Reason invoked only when someone hits the web page
- Regex on `title` to assign `docs` / `dependency-bump` (that is the costume; Rules use **fetched files**)
