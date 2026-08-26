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
| LLM runtime | Ollama, local |
| Serve | Simple web UI. No framework requirement (`net/http` is enough). |
| Result store | Postgres only. The worker writes. Serve reads Postgres. No result topic for v1. |

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
| Skip-LLM | Ordered list of rules in Go, evaluated after fetch. Hits write a real label and **do not** call the model. |
| Skip-LLM now | (1) every changed file ends with `.md` → `docs`. (2) every changed file is a lockfile → `dependency-bump`. |
| Skip-LLM later | Append more rules to the same list |
| Mixed trees | `.md` + code, or lockfile + code → **model**, not skip |
| Loop | Multi-step: **extract** (what changed / affected area) then **classify** (label + confidence + rationale). Not one prompt that returns three fields with no structure. |
| Parse | In Go: extract first `{...}` from dirty model text, trim/lowercase labels, retry on bad output, then `unknown`. |

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
| Go entrypoint | `cmd/app/main.go` | One process: Kafka consumer **and** HTTP serve |
| HTTP port | `8080` | |
| GitHub poll | First page of `/events` only, interval **30s** | Do not paginate the firehose in v1 |
| Ollama model | `llama3.2` | Must be pulled in Compose or documented. Change if quality is poor. |
| LLM retries | **2** extra attempts after first bad parse (3 tries total) then `unknown` | |
| Confidence branch | Persist label + confidence; if confidence **< 0.5**, still persist but do not treat as a second model pass yet | Threshold and “second pass vs unknown” are **open**; this default unblocks Phase 6 |
| Lockfile names | See list below | Widen later via the same skip-LLM list |
| File budget | Max **20** files; each patch truncated to **4000** chars; filenames + status always kept | Open to retune |
| Bot filter | **None** in v1 | Dependabot PRs may still flow. Extension-shaped later. |
| Draft PRs | **Keep** (no filter) | Open to drop drafts in Connect later |

### Lockfile set (skip-LLM)

`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, `Cargo.lock`, `go.sum`, `poetry.lock`, `Pipfile.lock`, `Gemfile.lock`, `composer.lock`, `bun.lock`, `bun.lockb`

Bare `go.mod` without `go.sum` is **not** treated as lockfile-only.

---

## Assumptions

State these if asked in a walkthrough.

1. “GitHub as the source for PRs” means the public [Events API](https://api.github.com/events), not org webhooks or a single-repo poll.
2. Event `id` is unique per event. A future `synchronize` on the same PR is a **new** `id` and a **new** row.
3. `payload.pull_request.body` on the event may exist but is not trusted as complete; the worker always GET-pulls.
4. Skip-LLM file rules use the **fetched** file list, never the event payload.
5. Title, author, and action may be stored and shown; they are not the classification input for the model.
6. “Several prompts” means **distinct steps** (extract vs classify), not three ways of asking for the same label.
7. One Go binary for worker + serve is acceptable for a local demo; split processes if serve and worker need different scale (not v1).
8. Ollama speaks an OpenAI-compatible HTTP API from the Go service. Connect must not call it.
9. Public GitHub is a **demo dataset** standing in for an internal “opened PRs in our orgs” feed (customer story still open).

---

## Open

Do not silently resolve these into architecture-changing behavior.

### Must decide before a polished demo (not before Phase 1)

- Exact Ollama model if `llama3.2` is too weak or too slow on diffs
- Confidence: number-only vs **control-flow** (second extract, extra fetch, or force `unknown`)
- What the UI highlights (`security` first? filter? confidence sort?)
- Customer paragraph: who uses this, what decision a row drives, cost of wrong/missing
- Which **two** official tradeoff pairs to write in README (human-only)

### Can stay default through v1

- Poll interval / `X-Poll-Interval`
- Bot / draft filters in Connect
- File and patch size caps
- Whether to store raw model text
- HTML vs JSON-first page (both fine; pick in Phase 7)
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
