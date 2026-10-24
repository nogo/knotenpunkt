# Knotenpunkt — agent instructions

An embeddable claim layer: the claim record, its rules, its log and a decide step. Other systems (first sidenote) embed it as their only claim store.
It is a personal project, built issue by issue (`gh issue list`).

## Read first

- `docs/project.md`: the north star (vision, outcome, constraints, open questions). If a change doesn't serve it, it doesn't belong.
- `docs/history.md`: five earlier versions, and why each stalled. Don't repeat the pattern.
- `CODING_STANDARDS.md`: the Go rules a review checks against.
- The claim model comes from sidenote: `github.com/nogo/sidenote`, `docs/claim-system.md` and `docs/decisions.md` ("Decided 2026-09-30 — claims"), with the code in `internal/ledger/claim.go` and `internal/vault/claim.go`.

## Stack

- **Go**, standard library only. Every dependency needs a line in `docs/decisions.md` first.
- Storage: an append-only JSONL claim log is the truth. Anything else is derived and can be rebuilt.

## Rules

- **No ingestion.** knotenpunkt never parses code, docs or git. A claim enters only through a proposal.
- **No client data, ever.** Nothing from the work vault enters this repo: no claims, pages, client names or people's names. Use invented examples (ACME, OrderApp, Jane Roe). A tool that reads the work vault prints counts and scores, never content.
- **No secrets** in files, commits or test fixtures.
- **Decisions live in `docs/`, edited in the same change as the code** that implements or overturns them.
- **Verify before claiming.** Name the command or test that proves it.
- **Small changes.** Do the task, not the adjacent improvement. Put anything else in a note.
- **Comments:** a short one above a function (what it's for, when to reach for it); inline comments only when they earn their place.

## Commits

- `scope: imperative description`, at most 50 characters where possible, in English.
- The body is bullets: the problem, why this way, what was rejected, what breaks if reverted.
- The last trailer is `Model: <exact model id>` when a model wrote the change.
