# redpanda-build-exercise

## Ports

Host ports come from `docker-compose.yml` (`task infra:up`).

| Host | Service | Use |
| --- | --- | --- |
| `localhost:5432` | `postgres` | PostgreSQL (`triage` / `triage` / `triage`). `task db:psql` |
| `localhost:19092` | `redpanda` | Kafka API **from the host** |

Inside the Compose network, Kafka is **`redpanda:9092`**. Tasks that `exec` into the Redpanda container (for example `task topic:consume`) use that address, not `19092`.
