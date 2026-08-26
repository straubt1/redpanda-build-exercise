# Build plan — implement and verify in order

Each phase is a complete prompt for a later coding session. Do not skip phases. Do not implement a later phase “while you are here.”

After each phase: run the **Verify** list and only then continue.

Read with every phase: `.cursor/spec/README.md` (always-on constraints), `decisions.md`, `architecture.md`, `devloop.md` (Taskfile + inspect).

---

## Phase 0 — Compose skeleton + Postgres schema

### Prompt

```
Implement Phase 0 only from .cursor/spec/.

Create docker-compose.yml with postgres:16 and Redpanda (docker.redpanda.com/redpandadata/redpanda, --mode=dev-container). Do not add Connect, Ollama, or the Go app yet if that keeps the phase small — or add them as stubs that we will fill later, but they must not be required for this phase's verify.

Add db/schema.sql for pr_triages as in architecture.md and mount it so Postgres applies it on first boot.

Add .env.example with GITHUB_TOKEN=.
Add .gitignore entries for .env if missing.

Fill Taskfile.yml (version 3, dotenv .env) with Phase 0 tasks from .cursor/spec/devloop.md: default, setup, infra:up, infra:down, github:events, topic:consume, db:psql, logs. Names are namespace:verb (two segments max); variants are vars. github:events is one script (type counts + opened PRs; VERBOSE=1 for full page). topic:consume lists topics then consumes (FOLLOW=1 to tail). infra:up creates the work topic. Do not copy .reference/Taskfile.yml. Do not add connect: or up/e2e yet.

Do not implement ingest, worker, or UI.

Verify the checklist in .cursor/spec/build-plan.md Phase 0. Use `task …` not one-off docker/rpk.
```

### In scope

- Compose: Postgres + Redpanda healthy
- Schema applied
- `.env.example`
- `Taskfile.yml` Phase 0 tasks (`devloop.md`)

### Out of scope

- Connect pipeline, Go, Ollama, HTTP UI

### Verify

- [ ] `task` lists targets; `task setup` creates `.env` if missing
- [ ] `task infra:up` starts Postgres and Redpanda without errors
- [ ] `task db:psql` shows table `pr_triages` with PK `event_id`
- [ ] `task topic:consume` runs (empty topic is fine; should still list topics)
- [ ] `task github:events` prints type counts and an opened-PR array (may be `[]`); token required
- [ ] `.env` is gitignored; `.env.example` has no real token

---

## Phase 1 — Connect: poll, filter, project, cache, produce

### Prompt

```
Implement Phase 1 only from .cursor/spec/.

Add Redpanda Connect (docker.redpanda.com/redpandadata/connect) to Compose.
Write connect/ingest.yaml:

- HTTP poll GET https://api.github.com/events
- Headers: Bearer GITHUB_TOKEN, User-Agent, Accept application/vnd.github+json
- Explode the JSON array into one message per event
- Keep PullRequestEvent where payload.action is in allowlist ["opened"] (allowlist easy to extend)
- Project the work-topic JSON in architecture.md
- Cache on event_id so a re-poll does not reproduce
- Output to topic github.pr.opened (create topic if needed)

Add Phase 1 task from .cursor/spec/devloop.md: connect:up (deps infra:up, compose up -d connect). Use existing topic:consume and logs SERVICE=connect. No extra follow/restart targets.

No LLM in Connect. No Go worker yet.

Verify Phase 1 checklist. Print 2–3 sample messages from `task topic:consume`. If the topic is empty, use `task github:events` and say whether that page had opened PRs.
```

### In scope

- Connect service, ingest config, topic
- Filter + project + cache

### Out of scope

- Go, Ollama, Postgres writes from Connect

### Verify

- [ ] `task connect:up` brings up Redpanda + Connect (+ Postgres from Phase 0)
- [ ] `task logs SERVICE=connect` shows no auth/UA errors (token in env)
- [ ] `task topic:consume` shows JSON objects, **not** a raw array (or topic empty — see last item)
- [ ] Every consumed message has `event_id`, `repo`, `pr_number`, `pr_url`, `action` (`opened`)
- [ ] No `PushEvent` / issue events on the topic
- [ ] Wait one extra poll (`task topic:consume FOLLOW=1` or a second consume): same `event_id` does not appear twice (cache)
- [ ] If `task github:events` shows no opened PRs, an empty topic is acceptable — say so; do not loosen the filter to “any PR action” to force traffic

---

## Phase 2 — Go worker: consume and upsert without GitHub fetch or LLM

### Prompt

```
Implement Phase 2 only from .cursor/spec/.

Add a Go module and cmd/app that:
- Reads config from env (brokers, topic, postgres DSN)
- Consumes github.pr.opened
- Upserts a pr_triages row: copy topic fields, category=unknown, source=fallback, rationale indicating "pending enrichment" or similar
- Does not call GitHub for files, does not call Ollama

Add the app service to Compose (build from Dockerfile). Health is optional this phase.

Verify Phase 2. Confirm rows in Postgres match consumed event_ids.
```

### In scope

- Kafka consumer, Postgres upsert, Compose app service
- Idempotent upsert on `event_id`

### Out of scope

- Enrichment, skip-LLM, Ollama, HTML

### Verify

- [ ] App starts and consumes without crash-loop
- [ ] Each topic message becomes one row keyed by `event_id`
- [ ] Re-consuming the same payload (if you produce a duplicate by hand) updates, does not PK-error
- [ ] `category` is `unknown` for every row this phase (expected)

---

## Phase 3 — Enrichment: fetch PR body and files

### Prompt

```
Implement Phase 3 only from .cursor/spec/.

After consume, GET pull and GET files with GITHUB_TOKEN and User-Agent.
Apply file budget from decisions.md.
On 404/403, upsert unknown + error and ack.

Store enough to see enrichment worked: at least rationale or a column showing file names / body length. If the schema needs a files_json column, add it and mention the schema change in decisions.md only if you consider it locked; otherwise keep it local to the table.

Still no Ollama. Skip-LLM not required yet (may still write unknown).

Verify Phase 3 against a real opened PR from the topic or a manually produced message with a public repo/pr_number.
```

### In scope

- GitHub client in Go
- Truncation budget
- Error path → `unknown`

### Out of scope

- Skip-LLM rules, LLM loop, UI

### Verify

- [ ] A real PR row shows evidence of fetched files (names in rationale, log, or column)
- [ ] `task github:pull` (or the Phase 3 task from `devloop.md`) shows body/files for the same repo/number
- [ ] Body from the pull API is available to the next phase (in memory or stored)
- [ ] Invalid repo/pr upserts `unknown` with `error` set, consumer continues
- [ ] Logs include event_id and GitHub status on failure

---

## Phase 4 — Skip-LLM rules

### Prompt

```
Implement Phase 4 only from .cursor/spec/.

After fetch, evaluate an ordered skip-LLM list in Go:
1. all files end with .md → category docs, source=rule
2. all files are lockfiles (list in decisions.md) → dependency-bump, source=rule
Mixed trees do not skip.

Unit-test the rule functions with fake file name lists (no Docker required for tests).

No Ollama yet. Non-matching PRs can remain unknown.

Verify Phase 4.
```

### In scope

- Rule list, tests, persist `source=rule`

### Out of scope

- LLM

### Verify

- [ ] `go test` covers: all markdown; all lockfiles; mixed md+go; empty file list (must **not** classify as docs)
- [ ] A live or fixture PR that only touches markdown lands `docs` without an Ollama log line
- [ ] Rules are a list/slice, not a one-off `if` buried in consume

---

## Phase 5 — Parse, normalize, fallback (the required test)

### Prompt

```
Implement Phase 5 only from .cursor/spec/.

Implement model-output parse in Go (no live Ollama required):
- extract first JSON object from dirty text
- unmarshal
- normalize category (trim, lowercase)
- invalid/missing label → error the caller can retry

Fallback category is unknown.

Write the real test the exercise asks for: dirty output, extra prose around JSON, wrong enum, empty string. Tests must fail if extract/normalize/fallback breaks.

Do not wire Ollama yet unless it is a no-op client.

Verify Phase 5.
```

### In scope

- `internal/...` parse helpers + tests

### Out of scope

- Prompts, Ollama HTTP, UI

### Verify

- [ ] `go test` fails if you break brace extraction or enum normalize (spot-check by briefly sabotaging if unsure, then revert)
- [ ] Cases: markdown fences, leading “here is json”, trailing commentary, `Feature` → `feature`, `not-a-label` → parse error
- [ ] No network in these tests

---

## Phase 6 — Ollama loop: extract then classify

### Prompt

```
Implement Phase 6 only from .cursor/spec/.

Add ollama/ollama to Compose. Document or automate pulling the working-default model (llama3.2 unless decisions.md changed).

Go LLM client against Ollama's OpenAI-compatible API (or native). Timeouts required.

For records that did not skip-LLM:
1. extract prompt → parse/retry
2. classify prompt using extract + body + truncated files → parse/retry
3. persist category, confidence, rationale, affected_area, source=llm
4. after retries, unknown + source=fallback

Prompts must instruct the model to use body and files, not title alone. Title is optional extra context.

Confidence: persist the number. If below 0.5, do not add a second pass unless decisions.md was updated.

Verify Phase 6. Confirm at least one llm-sourced row and one unknown path (force bad model or invalid JSON in a test double if live model is well-behaved).
```

### In scope

- Ollama service, client, two-step loop, retries, upsert of LLM fields
- Logs: event_id, step name, parse retry count (no dumped secrets)

### Out of scope

- Fancy UI, extra topics, hosted Claude

### Verify

- [ ] `docker compose up` includes Ollama; app can reach it from its container (service name, not localhost, unless host-network is explicit)
- [ ] A mixed-file PR gets a category in the enum or `unknown`, never a raw model sentence in `category`
- [ ] Skip-LLM PRs still have **no** LLM call (log or `source=rule`)
- [ ] Parse retries happen on garbage (test double preferred)
- [ ] Consumer does not spin on one poison prompt forever (retry cap)

---

## Phase 7 — Serve a simple page

### Prompt

```
Implement Phase 7 only from .cursor/spec/.

Same Go process: GET / renders an HTML table of recent pr_triages (newest first). Link pr_url. Show category, confidence, affected_area, rationale, source.

GET /healthz returns 200 if Postgres ping succeeds.

No JS framework. Readable on a laptop browser.

Verify Phase 7.
```

### In scope

- HTML + health
- Compose port publish `8080`

### Out of scope

- Auth, websockets, SPA

### Verify

- [ ] Open `http://localhost:8080` — rows visible, including rule and llm sources if present
- [ ] PR link opens GitHub
- [ ] Empty table is a clear empty state, not a 500
- [ ] `/healthz` is 200 while Postgres is up

---

## Phase 8 — One-command local repro

### Prompt

```
Implement Phase 8 only from .cursor/spec/.

Make docker compose up the full path: redpanda, connect, postgres, ollama, app.

App Dockerfile is reproducible. Connect waits for Redpanda. App waits for broker + postgres.

README.md: copy-pasteable run instructions only (env, compose **or** `task up`, wait for Ollama model, open :8080). Do NOT write Tradeoffs, Why this matters, or surprises with AI. Leave a stub heading for those if you want, empty of generated prose.

Add Phase 8 tasks from .cursor/spec/devloop.md (`up`, `down`, `smoke`; reuse `logs`). Wrap Compose. `docker compose up` must still work without Task.

.env.example complete. Logs on app/connect are greppable (event_id).

Verify Phase 8 from a cold compose down -v if possible.
```

### In scope

- Dependencies, README run section, model pull story, logging pass

### Out of scope

- Human write-up paragraphs

### Verify

- [ ] Fresh `docker compose up` (documented wait) yields a working UI without undocumented manual steps
- [ ] Missing `GITHUB_TOKEN` fails loudly in logs
- [ ] README is enough for someone else to run it

---

## Phase 9 — Hardening and extension seams

### Prompt

```
Implement Phase 9 only from .cursor/spec/.

Tighten error paths: GitHub rate limit (log remaining if headers present), Ollama timeout, Connect input retries already in YAML if missing.

Confirm seams: action allowlist in Connect; skip-LLM slice; parse/classify functions; upsert column list easy to extend.

Optional: drop Connect cache on restart note in README (memory cache).

Do not add new product features (no new labels, no new actions) unless listed as open and the human asked.

Verify Phase 9.
```

### In scope

- Retries/timeouts, comments at seams, README operational notes (cache reset, rate limits)

### Verify

- [ ] You can point to the skip-LLM list and the parse function in one breath
- [ ] Postgres down: app does not ack-loop-delete data; logs show the error
- [ ] Walkthrough question “malformed model response?” is answerable from parse.go + tests

---

## Phase 10 — Human write-up (not an AI implementation task)

Do this yourself. Do not prompt an LLM to draft it.

README must include:

- Run instructions (already from Phase 8)
- **Tradeoffs** — two official pairs; each: what you chose, the alternative, when you would flip, what shaped the choice
- What surprised you
- Where this breaks in production (cache vs retry, firehose junk, local model JSON, GitHub rate limit, memory cache on restart, …)
- **Why this matters** — 3–4 sentences for a non-engineer: who, what decision, cost of wrong; demo vs real use case

Suggested pairs given this design (you may pick others):

1. App-side writes vs Connect sink
2. Multi-step extract+classify vs one classification call

---

## After a full rebuild from zero

Run phases 0→8 in order. If a phase verify fails, fix that phase; do not “make it work” by implementing Phase 6 during Phase 1.

When a verify forces a new decision, write it in `decisions.md` first, then code.
