# redpanda-build-exercise

## Ports

Host ports come from `docker-compose.yml` (`task infra:up`).

| Host | Service | Use |
| --- | --- | --- |
| `localhost:5432` | `postgres` | PostgreSQL (`triage` / `triage` / `triage`). `task db:psql` |
| `localhost:19092` | `redpanda` | Kafka API **from the host** |
| `localhost:8081` | `console` | [Redpanda Console](http://localhost:8081) — browse topics and messages |

`connect` has **no host port**. It polls GitHub and produces to Kafka on the Compose network (`redpanda:9092`). Logs: `task logs SERVICE=connect`.

Inside the Compose network, Kafka is **`redpanda:9092`**. Tasks that `exec` into the Redpanda container (for example `task topic:consume`) use that address, not `19092`.

## Simulate opened PRs

GitHub `/events` rarely includes `PullRequestEvent` + `opened`. Fixtures in `sim/pr-opened/*.json` are work-topic payloads (`event_id` prefix `sim-`). They are produced with `rpk`; Connect and `connect/ingest.yaml` are unchanged.

```
task sim:replay          # wipe sim Postgres rows + recreate topic, then produce fixtures
task topic:consume       # read github.pr.opened from the start
```

`task sim:reset` deletes `pr_triages` rows whose `event_id` starts with `sim-` and **recreates** `github.pr.opened` (all messages, not only sim). `task sim:produce` writes the JSON files onto the topic without resetting. Add files under `sim/pr-opened/`; keep `event_id` unique and `sim-` prefixed. `repo` / `pr_number` are real public PRs so a later worker GET still works.
