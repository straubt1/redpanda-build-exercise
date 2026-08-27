# Decisions

Update this file when a choice is locked, a default is promoted, or an open question is answered. Date the change in one line under **Decision log**.

Status vocabulary:

- **Locked** — do not reopen in an implementation session unless the human says so.
- **Working default** — use this to build; not defended as final.
- **Assumption** — believed true; say it aloud in the walkthrough.
- **Open** — do not invent a clever answer; use the working default if one exists, otherwise ask.

---

## Locked

### Product / stack

| Item | Choice |
| --- | --- |
| Source | GitHub public events: `GET https://api.github.com/events` |
| Auth | `GITHUB_TOKEN` (required). User-Agent required on all GitHub calls (Connect and Go). |
| Language | Go |
| LLM runtime | Ollama on the **host** (not a Compose service). **`reason`** reaches it at `host.docker.internal:11434`. |
| Serve | Compose service **`serve`**: HTML + JSON API. Reads Postgres only. |
| Reason | Compose service **`reason`**: consume topic, enrich, Rules or Summarize→Classification, upsert Postgres. |
| Result store | Postgres only. **`reason`** writes. **`serve`** reads. No result topic for v1. |
| Dev CLI | [Task](https://taskfile.dev/) (`Taskfile.yml` at repo root). Leaf tasks for inspect/run; composites stitch them later. `.reference/Taskfile.yml` is not the spec. |

### Connect

| Item | Choice |
| --- | --- |
| Role | Ingest, filter, project, cache, produce to a work topic. No LLM. |
| Keep | `type == PullRequestEvent` AND `payload.action` in an **allowlist** |
| Allowlist now | `["opened"]` only |
| Allowlist later | Add actions (e.g. `reopened`, `synchronize`) via config, not a new pipeline |
| Dedupe | Connect **cache**, key = GitHub event `id` |
| Project | Small schema: ids + urls the worker needs to fetch and upsert. Not the raw event blob. |

### Reason (Go)

| Item | Choice |
| --- | --- |
| Topology | Worker **consumes** the work topic (Connect produces; Redpanda does not “post”) |
| Enrichment | Fetch PR **body** and **changed files** from GitHub. Events do not include the file list. |
| Writes | App-side upsert to Postgres. Connect is not the classification sink. |
| Idempotency | GitHub event `id` is cache key, message identity, and Postgres primary key |
| Labels | `security`, `feature`, `refactor`, `docs`, `dependency-bump`, `unknown` |
| `unknown` | Failure/fallback: cannot fetch enough content, or model output still unusable after retries, or label not in enum |
| Rules | Ordered list of non-model checks in Go, evaluated after fetch. A rule either classifies or does not. Hits write a real label and **do not** call a Model. |
| Rules now | (1) every changed file ends with `.md` → `docs`. (2) every changed file is a lockfile → `dependency-bump`. |
| Rules later | Append more rules to the same list |
| Mixed trees | `.md` + code, or lockfile + code → **Model**, not a Rule |
| Loop | Multi-step: **Summarize** (affected area + summary) then **Classification** (category + confidence + rationale). Not one prompt that returns three fields with no structure. |
| Prompt files | Static instructions in `internal/reason/prompts/` (`summarize.txt`, `classify.txt`, `classify_repair.txt`), `//go:embed`. Go appends `## Summary` (classify only), a `## Input` section (title, fenced body, then changed-file totals), and the classify-retry error line. Body is a markdown fence (backtick run longer than any run in the truncated body) so PR headings are not part of the prompt. Classification Input includes per-file patches; Summarize does not. Title is always present; the value may be `""`. Not templates; not loaded from disk at runtime. |
| Parse | In Go: take first `{...}` from dirty model text, trim/lowercase labels, retry on bad output, then `unknown`. |
| `source` | `rule` (a Rule classified), `model` (Models classified), `fallback` (unknown after failure) |

### Cache consequence (locked behavior, not a preference)

Connect cache means a later GitHub poll **will not** re-deliver the same `id`. Failed LLM work cannot be healed by “wait for the next poll.” Recovery is **in the worker**: retry before success, and/or do not commit the consumer offset until upsert succeeds. Upsert still overwrites if the same `id` is produced again (manual replay).

---

## Working defaults (use these until changed)

| Item | Default | Notes |
| --- | --- | --- |
| Work topic name | `github.pr.opened` | Rename consistently in Connect + Go if changed |
| Postgres db/user/db name | `triage` / `triage` / `triage` | |
| Table | `pr_triages` | PK `event_id` |
| Connect config path | `connect/ingest.yaml` | |
| Go entrypoints | `cmd/reason/main.go`, `cmd/serve/main.go` | Two Compose services, two binaries. Same module. |
| HTTP port | `8080` | Compose publishes `8080`. |
| UI CSS | Inline styles from `.reference/index.html` | Warm gray page (`#f7f8f5`), white table, uppercase headers, green links (`#0b5`). No Pico. |
| Table sort | Query `sort` + `dir` | Default `classified_at` `desc`. Same params on HTML and JSON. Allowlist columns (no raw SQL identifiers). |
| Table list | Latest **20** by `classified_at`, then sort | Newest rows always in the result set. Sort reorders those 20, it does not pick a different slice. |
| UI stats | Counts over **all** `pr_triages` | Total; not reasoned (`source` not `model`/`rule`); `source=model`; `source=rule`. |
| When column | Browser local time | Display like `1:30 pm`; `title` hover is full local datetime with seconds. Tiny inline JS; no library. |
| JSON API | `GET /api/triages` | Same row set as `GET /`, plus `stats`. |
| GitHub poll | `generate` every **30s**, then `http` GET **multiple** pages of `/events?per_page=100` | Several requests per sweep so more of the timeline is covered; rate-limit so that sweep can finish. **max 1** opened PR per sweep (`batch_index() >= 1` → drop) before cache. Cache still drops duplicate `id`s. |
| Ollama model | `qwen2.5:14b` | Taskfile var `OLLAMA_MODEL`. Pull on the host: `ollama pull qwen2.5:14b`. |
| LLM retries | **2** extra attempts after first bad parse (3 tries total) then `unknown` | |
| Confidence branch | Persist label + confidence; if confidence **< 0.5**, still persist but do not treat as a second model pass yet | Threshold and “second pass vs unknown” are **open**; this default unblocks Phase 6 |
| Lockfile names | See list below | Widen later via the same Rule list |
| File budget | Max **20** files; each patch truncated to **4000** chars; filenames + status always kept. Classify Input marks a cut patch with `[truncated]` on the file heading. | Open to retune |
| Bot filter | **None** in v1 | Dependabot PRs may still flow. Extension-shaped later. |
| Draft PRs | **Keep** (no filter) | Open to drop drafts in Connect later |
| Task binary | `task` (go-task) on the host | `brew install go-task` |
| Inspect GitHub | `task github:events` (`VERBOSE=1` for full page) | One curl; counts + opened PRs |
| Inspect topics | `task topic:consume` (`FOLLOW=1` to tail) | `rpk` in the Redpanda container |
| Inspect Postgres UI | pgAdmin `http://localhost:8082` | Compose profile `debug` (`docker compose --profile debug up -d`). No login (`SERVER_MODE=False`). Server **triage** is registered from `pgadmin/servers.json`. |
| Host Ollama | `task ollama:up` / `ollama:logs` / `ollama:down` / `ollama:check` | `up` starts `OLLAMA_HOST=0.0.0.0:11434 ollama serve` unless `GET http://127.0.0.1:11434/api/ps` is already OK. `down` is `pgrep ollama` then `kill`. Logs: `.local/ollama.log`. |
| `infra:up` | Default Compose stack | Postgres, Redpanda, Connect, **reason**, **serve**. Creates the work topic. Requires `GITHUB_TOKEN`. Ollama stays on the host (`task ollama:up`). Console and pgAdmin are profile `debug`. |
| `infra:down` | Stop that stack, keep volumes | `infra:down:clean` also deletes volumes. |
| jq | Required for `github:events` / `github:pull` / `ollama:check` | Fail clearly if missing |
| Reason debug dumps | `/logs/{event_id}/` | Always on. Kafka message, enrichment, created prompts + Ollama responses, Postgres row. Fail-open (stderr). Compose bind-mounts `.local/reason-logs:/logs`. |

### Lockfile set (Rules)

`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, `Cargo.lock`, `go.sum`, `poetry.lock`, `Pipfile.lock`, `Gemfile.lock`, `composer.lock`, `bun.lock`, `bun.lockb`

Bare `go.mod` without `go.sum` is **not** treated as lockfile-only.

---

## Assumptions

State these if asked in a walkthrough.

1. “GitHub as the source for PRs” means the public [Events API](https://api.github.com/events), not org webhooks or a single-repo poll.
2. Event `id` is unique per event. A future `synchronize` on the same PR is a **new** `id` and a **new** row.
3. `payload.pull_request.body` on the event may exist but is not trusted as complete; the worker always GET-pulls.
4. Rule file checks use the **fetched** file list, never the event payload.
5. Title, author, and action may be stored and shown; they are not the classification input for the model.
6. “Several prompts” means **distinct steps** (Summarize vs Classification), not three ways of asking for the same label.
7. **`reason`** consumes the work topic and writes Postgres. **`serve`** only reads Postgres and hosts HTTP. Hitting the web page does not classify.
8. Ollama speaks an OpenAI-compatible HTTP API from the Go service. Connect must not call it. The model process is on the host; Compose does not run `ollama/ollama`.
9. Public GitHub is a **demo dataset** standing in for an internal “opened PRs in our orgs” feed (customer story still open).
10. Docker Desktop on Mac can reach host Ollama at `host.docker.internal:11434`.

---

## Open

Do not silently resolve these into architecture-changing behavior.

### Must decide before a polished demo (not before Phase 1)

- Exact Ollama model if `qwen2.5:14b` is too weak or too slow on diffs
- Confidence: number-only vs **control-flow** (second Summarize, extra fetch, or force `unknown`)
- What the UI highlights (`security` first? filter? confidence sort?)
- Customer paragraph: who uses this, what decision a row drives, cost of wrong/missing
- Which **two** official tradeoff pairs to write in README (human-only)

### Can stay default through v1

- Poll interval / `X-Poll-Interval`
- Bot / draft filters in Connect
- File and patch size caps
- Whether to store raw model text
- Hosted Claude/OpenAI as a second backend (noted as a future option; v1 is Ollama)
- Topic-per-label or a result topic (v1 is Postgres column only)
- Batching LLM calls

### Explicitly out of v1

- Redpanda Cloud
- LLM inside Connect
- Classifying issues, pushes, or PR reviews
- PR actions other than `opened` (allowlist is ready; do not enable until asked)
- Pagination through all public events
- Multi-service fan-out by category

---

## Decision log

- 2026-08-26 — Discovery: Connect = plumbing; Go = reason; enrich before reason.
- 2026-08-26 — Source GitHub PRs, Ollama local, Go, simple web.
- 2026-08-26 — `/events`; opened-only allowlist; token; Connect cache on `id`.
- 2026-08-26 — Keep exercise labels + `unknown`; skip-LLM `.md` and lockfile; idempotency = event `id`; Postgres results.
- 2026-08-26 — Working defaults filled so a from-scratch rebuild is not blocked (topic, table, poll, retries, file budget, model name).
- 2026-08-26 — Spec moved to `.cursor/spec/`; thin Cursor rules in `.cursor/rules/` point at it.
- 2026-08-26 — Taskfile: `namespace:verb`, vars for variants, scripts may be multi-step (no task sprawl). See `devloop.md`.
- 2026-08-26 — Connect poll: `generate` + `http` processor, multiple `/events` pages every 30s to cover more of the timeline. Not `http_client` on a single URL.
- 2026-08-26 — Connect: max **4** opened-PR messages per generate sweep (`batch_index()`), then cache.
- 2026-08-26 — pgAdmin on host **8082**, started with `infra:up` like Console. Desktop mode (`SERVER_MODE=False`) skips the login page.
- 2026-08-26 — Postgres image **18** (latest stable). Local volume wiped on the bump.
- 2026-08-26 — `/events` has no PR `title`. Connect does not map it; worker GET pull supplies title for Postgres.
- 2026-08-26 — Phase 3: enrichment evidence lives in `rationale` (`body_len` + filename/status). No `files_json` column. Body and files stay in memory for later phases.
- 2026-08-26 — Ollama runs on the **host**, not Compose. Model working default `qwen2.5:14b` (`OLLAMA_MODEL` in Taskfile). Tasks: `ollama:up`, `ollama:logs`, `ollama:down`, `ollama:check`.
- 2026-08-26 — App reaches host Ollama at `host.docker.internal:11434`. Do not `extra_hosts` that name to `host-gateway` on Docker Desktop (it became `172.17.0.1` and connection refused).
- 2026-08-26 — Phase 7 UI: Pico CSS tables, sortable via `sort`/`dir`, JSON at `GET /api/triages`. Still `net/http`, no SPA.
- 2026-08-26 — Split Go: Compose **`reason`** (Kafka + GitHub + Ollama + upsert) vs **`serve`** (HTML + JSON, Postgres read). Former `app` service is `reason`.
- 2026-08-26 — `task ollama:up` sets `OLLAMA_HOST=0.0.0.0:11434` so LAN clients can reach this Mac. Override: `task ollama:up OLLAMA_HOST=127.0.0.1:11434`.
- 2026-08-27 — UI CSS matches `.reference/index.html` (inline, no Pico). Sort + extra columns (area, source) stay.
- 2026-08-27 — Serve lists the latest 20 rows; stats (total / not reasoned / model / rule) over the whole table; When is browser-local with a full-time hover.
- 2026-08-27 — Reason terminology: Rules then Summarize then Classification; `source=model`; persist `summary`; `created_at` matches the Message.
- 2026-08-27 — LLM instructions live in `internal/reason/prompts/` and are `//go:embed`’d. Evidence assembly stays in Go.
- 2026-08-27 — Connect: max **1** opened-PR message per generate sweep (`batch_index() >= 1`).
- 2026-08-27 — `task infra:up` starts the full Compose stack (including Connect, reason, serve). Ollama remains host-side.
- 2026-08-27 — Model `## Input` order is title, body, then changed files. File totals (`additions`/`deletions`/`changes`) are summed from the fetched file list. Title may be `""`.
- 2026-08-27 — Summarize Input is title, body, and change totals only. Classification Input also includes per-file patches.
- 2026-08-27 — Reason writes per-event debug dumps to `/logs/{event_id}/` (always on, fail-open). Compose bind-mounts `.local/reason-logs:/logs`.
- 2026-08-27 — Model `## Input` fences the PR body in markdown so its headings/code fences are not part of the prompt.
- 2026-08-27 — Classify Input marks a cut patch with `[truncated]` on `#### {filename} ({status})`. classify.txt splits counts vs per-file vs how to read a unified-diff patch.
- 2026-08-27 — `ollama:up` starts serve only if `GET /api/ps` is not OK. `ollama:down` is `pgrep ollama` then `kill`.
- 2026-08-27 — Console and pgAdmin sit on Compose profile `debug`. They are not started by `docker compose up` / `task infra:up`. Start with `docker compose --profile debug up -d`.
