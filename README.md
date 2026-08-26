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
