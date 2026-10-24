# Decisions

## Decided 2026-10-24 — the claim log

- A retraction is a claim whose `retracted` field names the claim it retracts. It is never live itself.
- The fold runs from the newest entry back. A link only points at an earlier claim, so one pass decides every claim. If a correction is itself superseded, the claim it first superseded stays gone.
- A rule is checked when a claim is written and never again. Replaying the log and confirming a proposal do not check rules.
- A source needs `system` and `ref`. `version` may be empty, because only the embedding system knows if a source has versions.
- `recorded_at` is set by the log, not by the caller. An empty `id` is filled with a random hex string.
- One writer is a rule for the caller. The log guards its own state with a mutex and takes no file lock.
