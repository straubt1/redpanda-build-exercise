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
│ GO WORKER + HTTP (one process)       │
│  consume                             │
│  GET pull + GET files                │
│  skip-LLM rules or extract→classify  │
│  parse / retry / unknown             │
│  upsert Postgres                     │
│  HTTP reads Postgres                 │
└──────────────────────────────────────┘
        │
        ▼
  postgres table pr_triages
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
 cap 4 per sweep (batch_index)
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

**Job:** fetch what the feed omitted, skip when a rule is enough, otherwise run a loop, persist a valid row.

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
 skip-LLM list (first match wins)
   all paths end with .md     → docs, source=rule
   all paths in lockfile set  → dependency-bump, source=rule
        │
     no match
        │
        ▼
 STEP 1 extract (model): affected area + what changed
        parse; retry; else unknown + stop
        │
        ▼
 STEP 2 classify (model): category + confidence + rationale
        using extract output + body + files (truncated)
        parse; retry; else unknown
        │
        ▼
 upsert pr_triages (PK event_id)
```

### Enrichment APIs

Same token + User-Agent as Connect.

- Pull: `GET https://api.github.com/repos/{owner}/{repo}/pulls/{number}`
- Files: `GET https://api.github.com/repos/{owner}/{repo}/pulls/{number}/files`

Respect file budget from `decisions.md`. Always keep `filename` and `status` (added/modified/removed). Patches are optional evidence for the model; filenames alone are not enough for the “read the change” requirement when a patch exists — send truncated patch when present.

404 / 403: upsert `unknown`, store error, do not crash the consumer.

### Skip-LLM

Implement as a list of functions `(files) -> (category, ok)` so more rules can be added later without touching the loop.

- Zero files: do not skip; fall through to “not enough content” / model / `unknown` as in the flow.
- All `.md`: includes `README.md`, `docs/foo.md`. A single `foo.mdx` or `foo.go` means **no** skip.
- All lockfiles: names in `decisions.md`. Case-sensitive as GitHub returns them.

When skip-LLM fires: still persist repo, pr, title, url, file list summary if the schema allows. `confidence` may be null. `rationale` can be a short static string (“all changed files are markdown”).

### Model I/O

**Extract** must produce structured JSON, e.g. `{ "affected_area": string, "change_summary": string }`. Exact keys may vary; parse must be tolerant.

**Classify** must produce `{ "category": string, "confidence": number, "rationale": string }`.

`category` normalized: trim, lowercase, hyphenate spaces if needed. If not in the enum → retry once with a “label must be one of …” repair prompt, then `unknown`.

Do not classify from title alone in the prompt. Title may be included as context **after** body/files, not instead of them.

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
  source          TEXT NOT NULL DEFAULT 'unknown', -- rule | llm | fallback
  error           TEXT NOT NULL DEFAULT '',
  received_at     TIMESTAMPTZ,
  classified_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pr_triages_classified_at ON pr_triages (classified_at DESC);
```

`category` check constraint optional; application enum is source of truth.

Upsert: `INSERT ... ON CONFLICT (event_id) DO UPDATE`.

`received_at`: parse topic `created_at` ISO string into `TIMESTAMPTZ`. Never insert a raw epoch number.

## Serve

v1: `GET /` HTML table of recent rows (newest first). `GET /healthz` for Compose.

Show: repo, pr number (link), title, category, confidence, affected area, rationale, source, time.

Plain CSS, no SPA. Usable at `localhost:8080`.

## Compose (target for the last infra phase)

Services: `redpanda`, `connect`, `postgres`, `ollama`, `app`. Local inspect: Console `:8081`, pgAdmin `:8082` (not the product UI).

- Redpanda: `--mode=dev-container`.
- App waits for broker + postgres; Ollama model may need a pull sidecar or documented `ollama pull`.
- `.env` gitignored. `.env.example` lists `GITHUB_TOKEN` and any future LLM keys (empty).
- Connect gets the token from env. Do not bake secrets into YAML.

Developer loop (Taskfile, how to dump `/events`, how to consume topics): [devloop.md](devloop.md). `task up` may wrap Compose; `docker compose up` must still work for the takehome.

## Seams for the pre-walkthrough extension

Keep these as obvious lists/functions, not scattered `if`s:

1. Action allowlist (Connect).
2. Skip-LLM rule list (Go).
3. Category enum + normalize (Go).
4. After-classify hook: e.g. if `security` and confidence high, produce an extra topic later.
5. Topic JSON + table columns: adding a field should touch project mapping, struct, upsert, HTML — in obvious places.

## What not to build

- Connect SQL sink of classifications
- Reason invoked only when someone hits the web page
- Regex on `title` to assign `docs` / `dependency-bump` (that is the costume; skip-LLM is on **fetched files**)
