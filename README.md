# redpanda-build-exercise

## Ports

Host ports come from `docker-compose.yml` (`task infra:up`).

| Host | Service | Use |
| --- | --- | --- |
| `localhost:8080` | `serve` | HTML table + JSON API (`/api/triages`, `/healthz`). `task serve:up` |
| `localhost:5432` | `postgres` | PostgreSQL (`triage` / `triage` / `triage`). `task db:psql` |
| `localhost:19092` | `redpanda` | Kafka API **from the host** |
| `localhost:8081` | `console` | [Redpanda Console](http://localhost:8081) — browse topics and messages |
| `localhost:8082` | `pgadmin` | [pgAdmin](http://localhost:8082) — browse Postgres. Server **triage** is pre-registered (no login). |

`connect` has **no host port**. It polls GitHub and produces to Kafka on the Compose network (`redpanda:9092`). Logs: `task logs SERVICE=connect`.

`reason` has **no host port**. It consumes `github.pr.opened`, GETs PR body/files, skip-LLM or extract→classify via **host** Ollama (`host.docker.internal:11434`), and upserts Postgres. Logs: `task logs SERVICE=reason`. Start: `task ollama:up` then `task reason:up`. Stop: `task reason:down`.

`serve` only reads Postgres and hosts `http://localhost:8080`. It does not consume Kafka or call Ollama. Start: `task serve:up`. Stop: `task serve:down`. Stopping one Go service does not stop the other.

Inside the Compose network, Kafka is **`redpanda:9092`**. Tasks that `exec` into the Redpanda container (for example `task topic:consume`) use that address, not `19092`.

## Host Ollama

Ollama is **not** in Docker. Start it on the laptop (Metal) before the worker needs a model:

```
task ollama:up            # background ollama serve, then version + models
task ollama:logs          # tail .local/ollama.log
task ollama:check         # API only
task ollama:down          # stop whatever is listening on 11434
```

Model name is Taskfile var `OLLAMA_MODEL` (default `qwen2.5:14b`). If `ollama:check` warns it is missing: `ollama pull qwen2.5:14b`.

## Simulate opened PRs

GitHub `/events` rarely includes `PullRequestEvent` + `opened`. Fixtures in `sim/pr-opened/*.json` are work-topic payloads (`event_id` prefix `sim-`). They are produced with `rpk`; Connect and `connect/ingest.yaml` are unchanged.

```
task sim:replay          # wipe sim Postgres rows + recreate topic, then produce fixtures
task topic:consume       # read github.pr.opened from the start
```

`task sim:reset` deletes `pr_triages` rows whose `event_id` starts with `sim-` and **recreates** `github.pr.opened` (all messages, not only sim). `task sim:reset:all` deletes **every** `pr_triages` row and recreates the topic. `task sim:produce` writes the JSON files onto the topic without resetting. Add files under `sim/pr-opened/`; keep `event_id` unique and `sim-` prefixed. `repo` / `pr_number` are real public PRs so a later worker GET still works. Skip-LLM fixtures: `sim-004` is markdown-only (`rust-lang/rfcs#1` → `docs`); `sim-005` is lockfile-only (`grommet/grommet-site#568` → `dependency-bump`). Both use `source=rule`. `task test` runs those rules without Docker.
