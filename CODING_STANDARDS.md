# Coding standards

Language rules for knotenpunkt. Stack, dependency policy and project rules are in `AGENTS.md`.

Base: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Google Go Style](https://google.github.io/styleguide/go/). Where this file is silent, they apply.

## Check

`mise run check` (vet, gofmt, tidy, `go test -race`) must pass before a change is done.

## Go

### Structure
- knotenpunkt is embedded by other systems. Packages they import live at the module root (`claim/`, `decide/`); everything else lives under `internal/`.
- One concept per package, named for it, never `util` or `common`.
- Every package starts with a package comment saying what it is and when to reach for it; `doc.go` when the package has several files.
- An interface is declared where it's consumed and holds only the methods that consumer calls. No interface for a single implementation unless a test fake is the second.
- Wiring happens in `cmd/knotenpunkt`. A package doesn't construct its own collaborators.

### Naming
- Short receivers, same name on every method of a type.
- Exported identifiers read well with the package: `claim.Fold`, not `claim.FoldClaims`.
- Constants for closed sets of strings (`claim.Sourced`), typed when the set is a kind (`claim.Trust`).

### Errors
- Messages start with the package: `"claim: no source"`, `"decide: no options"`.
- Wrap with `%w` and add the context the caller lacks (file, line, id). Don't wrap twice with the same context.
- Callers branch on sentinels (`ErrInvalid`) via `errors.Is`, never on message text.
- No `panic` outside `main` and impossible states.

### Concurrency
- Every goroutine has an owner that stops it; `Close` waits for it.
- A mutex guards named fields; say which in a comment on the field or struct.
- Tests run with `-race`; a race is a bug, not flake.

### Tests
- Fakes over mocks: an in-memory type with the real interface, asserting on outcomes, not calls.
- Temporary files via `t.TempDir()`; no fixed paths, no shared state between tests.
- Table-driven only when the cases share one shape; otherwise separate tests with descriptive names.
- Tests that need a real model are `Live` tests, gated by `KNOTENPUNKT_LIVE=1`.
- Wait for a signal (channel, event, condition), never `time.Sleep`.
- A bug fix starts with a test that fails for the bug's reason.
- A pure refactor changes no test assertions.

## Overrides

- None yet. Add one when a review hits the same defect twice.
