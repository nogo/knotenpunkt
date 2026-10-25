# Decisions

## Decided 2026-10-24 — the claim log

- A retraction is a claim whose `retracted` field names the claim it retracts. It is never live itself.
- The fold runs from the newest entry back. A link only points at an earlier claim, so one pass decides every claim. If a correction is itself superseded, the claim it first superseded stays gone.
- A rule is checked when a claim is written and never again. Replaying the log and confirming a proposal do not check rules.
- A source needs `system` and `ref`. `version` may be empty, because only the embedding system knows if a source has versions.
- `recorded_at` is set by the log, not by the caller. An empty `id` is filled with a random hex string.
- One writer is a rule for the caller. The log guards its own state with a mutex and takes no file lock.

## Decided 2026-10-24 — word overlap recall

- Recall scores the number of distinct words shared by the input and a live claim. Repeated words do not increase the score. Zero-score claims are omitted.
- Words are lowercased Unicode letter or number sequences. Punctuation separates words. A short English and German stop-word list applies to both sides.
- Equal scores sort by newer `asof`, then ascending id. Recall uses the existing live fold and stores no index.
- `Log.Recall` accepts text, a limit and optional excluded ids. `Log.RecallClaim` accepts a claim and excludes its `supersedes` and `challenges` links, including before the claim enters the log.

## Decided 2026-10-24 — frozen oracle from sidenote logs

The oracle reads sidenote records directly, without the claim package or external dependencies. Its only persistent output is a new JSONL file outside the vault and this repository. The file has owner-only permissions. Existing output files are refused.

The fold follows sidenote's `internal/ledger` (checked 2026-10-25). A record's `author` is an object, and only `author.kind: "user"` confirms a claim directly. `supersedes` and `challenges` are lists of ids. Decisions apply in record id order (UUIDv7, so time order); the first decision on a claim wins, and a decision not made by the user is ignored, as sidenote refuses both. Only confirmed records supersede or retract claims. Historical confirmed claims remain available for conflict pairs. Retraction markers and revisions with equal text after removing whitespace and folding case supply no conflict label. Both endpoints of a challenge must be confirmed.

The reader opens each log read-only and captures its length before reading records. It takes no locks. An unfinished final line is omitted and counted, so a concurrent append cannot produce a malformed record error. This is a bounded view of each file, not a transaction across files. A later run can include newly appended records.

Unrelated pairs use live confirmed claims with no direct link in either direction and unequal source pages. Two absent pages count as shared; an absent page and a known page differ. A seeded reservoir samples these pairs without storing every candidate. Seed defaults to 1. If candidates run out, the oracle reports the shortfall instead of inventing labels. Pair identifiers hash the label and the unordered claim identifiers. Reasons contain fixed rule names, never source metadata.

Counts describe read claim records, confirmed claim identifiers, live claims, and pairs per label. Conflict exclusions count candidate links by reason. Unrelated exclusions count candidate live pairs by reason, including eligible pairs omitted by sampling. A shortfall counts missing unrelated examples. Partial records count unfinished log lines. No diagnostic contains record contents, identifiers, or paths.

## Decided 2026-10-25 — namespaces

- A `Store` is the claim layer of one vault and the only exported entry point. Every call names a `Namespace`: an org, or an org and one engagement, each lowercase letters, digits and dashes. `ParseNamespace` reads `org` or `org/engagement` and returns the zero namespace on error, which every method refuses.
- An engagement reads its own live claims, then its org's. An org reads only its own. A sibling engagement or another org is never read, by `Live`, `Recall` or `RecallClaim`. `Proposals` and `Decide` act on the named namespace only.
- `supersedes`, `retracted` and `challenges` name claims in the claim's own namespace. A fact that crosses namespaces is a new claim whose source is `Cite(ns, id)`: system `knotenpunkt`, ref `<namespace>#<id>`. The cited claim must be live when the new claim is appended.
- The per-namespace log is unexported (`nsLog`). The layout on disk is the store's own: `<dir>/<org>/_org.jsonl` for the org and `<dir>/<org>/<engagement>.jsonl` for an engagement. `_` cannot start a slug, so the names never clash. Reading a namespace with no claims creates no file.
- Rejected: one log with a `namespace` field and a filter, because one missed filter leaks across clients; a deeper hierarchy, because the vault is already the outer boundary.
