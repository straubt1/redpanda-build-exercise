# Rebuild spec — PR triage pipeline

These files are the source of truth. If all application code is deleted, a later session should be able to rebuild the system from this directory plus a GitHub token and Docker.

Do not treat existing repo code as canonical. Treat **this directory** as canonical.

Cursor project rules in [`.cursor/rules/`](../rules/) are **short constraints**. They are not a second spec. If a rule and this kit disagree, update both; prefer this kit for design detail.

## How to use this in a later implementation session

1. Attach or `@` this `README.md` plus the files listed for that phase.
2. Paste the phase prompt from [`build-plan.md`](build-plan.md).
3. Implement **only that phase**. Stop when the verify checklist passes.
4. If a new decision is made, update [`decisions.md`](decisions.md) in the same session before coding further.
5. If the design changes, update [`architecture.md`](architecture.md) to match. Do not leave the spec stale.

### Always-on constraints (also in `.cursor/rules/00-pipeline.mdc`)

- Redpanda Connect does **plumbing only**: ingest, filter, project, cache, produce. **No LLM in Connect.** Not in a Connect `branch` processor.
- The Go service owns enrichment, skip-LLM rules, the reasoning loop, parse/retry, and Postgres writes.
- The model must classify from **fetched PR body and changed files**, not from title/comment/event metadata alone.
- Skip-LLM rules are allowed for obvious file-only cases (see decisions). Mixed trees go to the model.
- One command to run the full system eventually: `docker compose up` (Phase 8). Developer loop is **Taskfile** (`task …`); see [`devloop.md`](devloop.md). Each early phase may run a subset of tasks.
- Do not write the README Tradeoffs / “Why this matters” with AI. That is a human-only deliverable at the end.
- Prefer small, defendable Go over cleverness. The walkthrough will trace malformed model output line by line.

### Suggested attach set by kind of work

| Work | Read |
| --- | --- |
| Any implementation | `README.md`, `decisions.md`, `architecture.md`, current phase in `build-plan.md` |
| Inspect GitHub / topics / `task` targets | `devloop.md` |
| “Why is the loop this shape?” | `intent.md`, `decisions.md` |
| Changing a filter, topic field, or table column | `architecture.md`, `decisions.md` |
| Parking or locking a choice | `decisions.md` only, then resume the phase |

## Files

| File | Role |
| --- | --- |
| [`intent.md`](intent.md) | What the exercise is testing; model vs rule; scoring |
| [`decisions.md`](decisions.md) | Locked, working defaults, assumptions, open questions |
| [`architecture.md`](architecture.md) | Flows, APIs, message/table contracts, failure behavior |
| [`build-plan.md`](build-plan.md) | Ordered phases with copy-paste prompts and verify steps |
| [`devloop.md`](devloop.md) | Taskfile leaves vs composites; how to view GitHub payloads and Redpanda topics |

## Related (not required to rebuild)

Scratch (gitignored): `.local/FDE Takehome Assessment.md`, `.local/logs.md`, `.local/notes.md`.

If those are missing, this `spec/` directory is still sufficient.
