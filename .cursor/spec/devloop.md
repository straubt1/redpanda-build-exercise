# Dev loop — Taskfile, GitHub payloads, Redpanda topics

How we run and inspect the pipeline while building. Runtime is still Docker Compose. **Taskfile is the reproducible CLI** around Compose, `rpk`, `curl`, and later `go test`.

Install [Task](https://taskfile.dev/) on the host (`brew install go-task`). Repo root: `Taskfile.yml`. `task` with no args lists targets (`default` → `task --list`).

Do not treat `.reference/Taskfile.yml` as canonical. Names below match **this** spec (two Go processes: **reason** + **serve**, Ollama on the **host**, labels including `unknown`).

---

## Principles

1. **A task can be a short script.** Fetch once, `jq` several views, print a summary — that is one task, not three. Avoid a swarm of one-liner targets.
2. **`deps:`** when you truly start something else first (`connect:up` depends on `infra:up`). Do not explode every step into its own name.
3. **Verify** in `build-plan.md` calls `task …`. Variants use Task **vars**, not extra targets.
4. Takehome “one command” remains `docker compose up` (Phase 8). `task up` may wrap that. Both must work.

`dotenv: ['.env']`. Vars: `TOPIC: github.pr.opened`, `OLLAMA_MODEL: qwen2.5:14b`. Compose commands call `docker compose` directly.

---

## Naming

Pattern: `namespace:verb`. At most **two** segments, except a destructive qualifier on `down` (`infra:down:clean`). kebab-case verbs.

| Style | Use |
| --- | --- |
| `infra:up`, `connect:up` | Lifecycle of a compose slice |
| `github:events`, `github:pull` | Same system, different fetches |
| `topic:consume` | Inspect the work topic |
| `db:psql` | Database |
| `setup`, `up`, `down`, `test`, `logs` | Singletons — no colon until a second sibling exists |

**Variants are vars**, not new names:

| Instead of | Do |
| --- | --- |
| `github:events:opened` | `task github:events` (script already prints opened PRs; `VERBOSE=1` for the full page) |
| `topic:consume:follow` | `task topic:consume FOLLOW=1` |
| `connect:restart` | `task connect:up` (`compose up -d` is idempotent) |
| `topic:list` / `topic:create` | Create inside `infra:up`; `topic:consume` can print `rpk topic list` first |
| `test:parse` | `task test` with `go test ./...` (or `CLI_ARGS` if you need `-run`) |

---

## Inspect GitHub payloads

Connect `generate`s every 30s and GETs **multiple** pages of `/events?per_page=100` so each sweep covers more of the firehose. Each body is a **JSON array of mixed types**. The topic only shows what **survived** the filter. Inspect the API (`task github:events`) to see whether the firehose had opened PRs.

Needs `GITHUB_TOKEN`, non-empty `User-Agent`, and **`jq`** (fail with “install jq” if missing).

**`task github:events`** (Phase 0+) — one curl, then in the same script:

1. Histogram of `.type` (why the page looks empty of PRs)
2. Array of `PullRequestEvent` with `action==opened` (what Connect should keep)
3. If `VERBOSE=1`, the full page

```bash
# intent — one task, several jq views of the same JSON
body=$(curl -sS \
  -H "Authorization: Bearer ${GITHUB_TOKEN}" \
  -H "User-Agent: redpanda-build-exercise" \
  -H "Accept: application/vnd.github+json" \
  https://api.github.com/events)
echo "$body" | jq 'group_by(.type) | map({type: .[0].type, n: length})'
echo "$body" | jq '[.[] | select(.type=="PullRequestEvent" and .payload.action=="opened")]'
```

**`task github:pull`** (Phase 3+) — `REPO=owner/name PR=42`: pull JSON + files list (truncated like the worker budget if easy).

Optional: write the last `/events` body to `.local/last-events.json` (gitignored). Not required in Phase 0.

Do not use a Connect `log` processor as the primary inspect path.

---

## Inspect Redpanda topics

`rpk` **inside** the Redpanda service: `-X brokers=redpanda:9092` (do not rely on remembering the host port).

**`task topic:consume`** (Phase 1+; useful in Phase 0 once Redpanda is up):

1. `rpk topic list` (so you see whether `github.pr.opened` exists)
2. Consume `{{.TOPIC}}`
   - default: `-n {{.N | default 5}}` so the task **exits**
   - `FOLLOW=1`: stay on the stream

Empty topic is **valid** when `github:events` showed no opened PRs. Do not loosen the filter to force traffic. Optional later: `task topic:seed` (one public PR as work-topic JSON) — not Phase 0.

Create the work topic in **`infra:up`**, not a separate `topic:create`.

---

## Tasks by phase

Add a target when that phase first needs it. Scripts may be longer than one line.

### Phase 0

| Task | Does |
| --- | --- |
| `default` | `task --list` |
| `setup` | `.env` from example if missing; warn if `GITHUB_TOKEN` empty |
| `infra:up` | `docker compose up -d --build --wait`, then create `{{.TOPIC}}`. Requires `GITHUB_TOKEN`. |
| `infra:down` | Stop that stack, keep volumes |
| `infra:down:clean` | Stop stack and **delete volumes** (Postgres + topic log) |
| `github:events` | `/events`: counts + opened PRs (`VERBOSE=1` for raw page) |
| `topic:consume` | List topics + consume (`FOLLOW=1` to tail) |
| `db:psql` | `psql` in the postgres container |
| `logs` | `docker compose logs -f` (`SERVICE=redpanda` optional) |

### Phase 1

| Task | Does |
| --- | --- |
| `connect:up` | `deps: [infra:up]`, require token, `compose up -d connect` |

### Phase 2

| Task | Does |
| --- | --- |
| `reason:up` | `deps: [infra:up]`, require token, `compose up -d --build reason` (consume + upsert). Compose service was renamed from `app`. |
| `reason:down` | `compose stop reason` |

### Phase 3

| Task | Does |
| --- | --- |
| `github:pull` | `REPO=owner/name PR=42`: pull JSON + files (max 20, patch 4000 chars) |

### Phase 4

| Task | Does |
| --- | --- |
| `test` | `go test ./...` (Rules + parse/normalize). `CLI_ARGS` for `-run` |

### Host Ollama (not Compose)

| Task | Does |
| --- | --- |
| `ollama:up` | If `GET {{.OLLAMA_URL}}/api/ps` is not OK, `OLLAMA_HOST={{.OLLAMA_HOST}} ollama serve`; then `ollama:check` |
| `ollama:logs` | `tail -f .local/ollama.log` |
| `ollama:down` | If `pgrep ollama`, `/bin/kill` those pids |
| `ollama:check` | GET `/api/version` + `/api/tags`. Warn if `OLLAMA_MODEL` (`qwen2.5:14b`) is missing |

### Sim (local fixtures, not GitHub)

Work-topic JSON in `sim/pr-opened/*.json` (`event_id` prefix `sim-`). Produced with `rpk`; **Connect and ingest.yaml are unchanged**. `repo` + `pr_number` are real public PRs so a later worker GET still works.

| Task | Does |
| --- | --- |
| `sim:produce` | Write fixture files onto `github.pr.opened` |
| `sim:reset` | `DELETE` Postgres `event_id LIKE 'sim-%'`; **recreate** the work topic (wipes **all** topic messages); restart Connect if it was running |
| `sim:reset:all` | `DELETE FROM pr_triages` (every row); **recreate** the work topic; restart Connect if it was running |
| `sim:replay` | `sim:reset` then `sim:produce` |

Add more files in `sim/pr-opened/`; keep `event_id` unique and `sim-` prefixed.

### Phase 7

| Task | Does |
| --- | --- |
| `serve:up` | `deps: [infra:up]`, `compose up -d --build serve` (HTML + JSON on `:8080`) |
| `serve:down` | `compose stop serve` (`reason` can keep running) |

### Later (do not implement early)

- Phase 8: `up` (full compose `--build`), `down`, `smoke` (healthz + row count). `logs` already covers the stack.

---

## Mapping to build-plan verify

| Question | Command |
| --- | --- |
| GitHub reachable, and any opened PRs? | `task github:events` |
| Full `/events` page? | `task github:events VERBOSE=1` |
| Did Connect produce? | `task topic:consume` |
| Watch polls / cache (no duplicate ids)? | `task topic:consume FOLLOW=1` |
| Enrichment payload? | `task github:pull REPO=… PR=…` vs `task db:psql` |
| Is host Ollama running? | `task ollama:check` (`task ollama:up` to start) |

If opened PRs in `github:events` is `[]` and consume is quiet, the firehose is quiet — not a broken pipeline.
