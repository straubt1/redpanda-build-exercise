# Intent — what this system must demonstrate

Field Deployed Engineer takehome: a local pipeline that ingest public data, transforms it in Redpanda Connect, reasons in **your** service, stores a result, and serves it.

This is not “call an LLM from Kafka.” They score judgment: where work lives, when a model is justified, whether you can change the loop later, and whether you can explain it to a customer.

## Required pieces

1. **Ingest** a public source (no product signup). Chosen: GitHub public events.
2. **Transform in Connect** — filter, project, route. Connect is plumbing.
3. **Reason in a service you write** (Go). More than one prompt: routing, multi-step extraction, retries on bad output, confidence-based branching. Consumes a topic (or reads the store) and writes results.
4. **Serve** something a person can see. Chosen: simple web over Postgres.
5. **Test** the riskiest reasoning logic (dirty parse, fallback, branch). One real test that fails if that logic breaks.
6. **Run** with `docker compose up`.

AI is allowed for the build. The README Tradeoffs / surprises / production-break / “Why this matters” section must be written by the candidate **without AI**.

## Stack (locked by the exercise)

| Piece | Choice |
| --- | --- |
| Broker | `docker.redpanda.com/redpandadata/redpanda` (`--mode=dev-container`) |
| Pipeline | `docker.redpanda.com/redpandadata/connect` (Apache-2.0 only) |
| LLM | Local Ollama (open-weights). Hosted is allowed by the exercise; we are not starting there. |
| Storage | `postgres:16` |

**Off-limits:** Redpanda Cloud, Enterprise Connect connectors (CDC, Snowflake Streaming, Iceberg, Salesforce), `a2a_message`.

## Model vs rule (use these words this way)

**Rule** — deterministic logic over data you already have in structured form. Regex, thresholds, “all files end in `.md`,” allowlists. Cheap, inspectable, no reading of prose.

**Model** — the LLM. It is only doing the job they asked for if it **interprets content a rule cannot decide from**.

Most feeds hand you metadata (title, comment, flag, number). Piping that into a model to get a label is **a regex wearing a costume**: delete the model and a rule still produces the labels.

The signal that needs a model lives in content the feed **does not give you**. You fetch it (PR body, changed files / patches). Enrichment is required, not extra credit.

**Allowed rules (not costumes):**

- Connect: drop non-PR events, drop actions not in the allowlist, cache duplicate event ids.
- Go, after fetch: skip LLM when every file is `.md` → `docs`; every file is a lockfile → `dependency-bump`.
- Go, around the model: parse JSON, normalize labels, retry, default to `unknown`.

**Forbidden:** classify from title/event fields alone; one prompt that only echoes metadata; putting the reasoning loop in Connect `branch`.

Control-flow rules around the model (retry, confidence branch) are required. They do not replace the model’s judgment on mixed/ambiguous diffs.

## Official tradeoff pairs (exercise)

The README must discuss **two** of these. The build already implies answers for most; write-up still names the alternative and when you would flip.

| Pair | Our direction (see decisions.md) |
| --- | --- |
| Connect as sink vs app-side writes | App-side writes from the Go worker |
| One topic + label column vs topic-per-label | One work topic; label is a Postgres column. No result topic (yet). |
| One classification call vs multi-step loop | Extract then classify |
| Worker on a topic vs sync invoke from serve | Worker consumes the work topic |
| Parse in Connect vs in the service | Parse / retry / normalize in Go |
| Bound cost: filter in Connect, batch, or gate on confidence | Filter in Connect first; skip-LLM in Go; confidence gate still open |

## What they will do after submit

- Run `docker compose up` themselves. Clean start is a signal.
- ~2 days before the walkthrough: one small change to the **reasoning service** (< 1 hour). Examples they gave: confidence tier + route urgent items to a **new topic**; skip LLM for a known-bot pattern; thread a new field end to end. Return a diff.
- 60 min screen-share: bring it up, defend tradeoffs, walk the extension diff, trace an input, say what happens on malformed model JSON. No live coding.

**Design for that diff now:** skip-LLM as a list, action allowlist as config, loop in real functions (extract / parse / classify / persist), not one 400-line `main`.

## Scoring (what “strong” looks like)

- Connect config that is more than the happy path (poll, headers, explode array, filter, cache, errors).
- Transforms that make data more useful; junk never reaches the model.
- Loop has structure; handles failure and cost; reasons over fetched content; lives in Go.
- Result a person could act on.
- One-command local repro; useful logs.
- Honest tradeoffs; failure modes; customer framing (who, what decision, cost of wrong). Public GitHub is a demo dataset — the write-up must name the real use case it stands in for. That story is still **open**.
