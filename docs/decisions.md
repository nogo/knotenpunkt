# Decisions

## Decided 2026-10-24 — the claim log

- A retraction is a claim whose `retracted` field names the claim it retracts. It is never live itself.
- ~~The fold runs from the newest entry back. A link only points at an earlier claim, so one pass decides every claim. If a correction is itself superseded, the claim it first superseded stays gone.~~ *2026-11-02: the backward pass skipped the links of a superseded correction, so the claim it first superseded came back to life. The rule stays; see "confirm, reject and memory".*
- A rule is checked when a claim is written and never again. Replaying the log and confirming a proposal do not check rules.
- A source needs `system` and `ref`. `version` may be empty, because only the embedding system knows if a source has versions.
- `recorded_at` is set by the log, not by the caller. ~~An empty `id` is filled with a random hex string.~~ *2026-11-02: the id is the hash of the content; see "content ids".*
- ~~One writer is a rule for the caller. The log guards its own state with a mutex and takes no file lock.~~ *2026-11-02: reads take no lock; see "working tree". One writer per store and no file lock still hold.*

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
- An engagement reads its own live claims, then its org's. An org reads only its own. A sibling engagement or another org is never read, by `Live`, `Recall` or `RecallClaim`. `Proposals`, `Confirm` and `Reject` act on the named namespace only.
- `supersedes`, `retracted` and `challenges` name claims in the claim's own namespace. A fact that crosses namespaces is a new claim whose source is `Cite(ns, id)`: system `knotenpunkt`, ref `<namespace>#<id>`. The cited claim must be live when the new claim is appended.
- The per-namespace log is unexported (`nsLog`). The layout on disk is the store's own: `<dir>/<org>/_org.jsonl` for the org and `<dir>/<org>/<engagement>.jsonl` for an engagement. `_` cannot start a slug, so the names never clash. Reading a namespace with no claims creates no file.
- Rejected: one log with a `namespace` field and a filter, because one missed filter leaks across clients; a deeper hierarchy, because the vault is already the outer boundary.

## Decided 2026-10-25 — decide and score

- `decide.Client.Decide` sends one chat request with `max_tokens: 1`, `temperature: 0`, `logprobs`, `top_logprobs: 20` and `enable_thinking: false`, and reads the option keys `A`–`I` from the first token, as `systemone-spike/run_http.py` did. A key that appears as several tokens (`A`, ` A`, `A)`) counts once, at its likeliest. Probabilities are renormalised over the options; `Mass` keeps what fell on them.
- No option among the top tokens is `ErrNoOption`, with an `Unsure` result. An HTTP error is returned, never scored as an answer; `score` stops on it.
- The conflict question and its three option descriptions live in `cmd/knotenpunkt`, next to their one caller. They move when the intake check needs them.
- `score` prints one line each for the model, the constant baseline and word overlap. The overlap cut-off is learned on the same pairs and below it the answer is `unrelated`; it flatters the baseline on purpose. Per-pair results hold ids, labels and probabilities, never text, and are refused inside this repository.
- First run, synthetic pairs (36, a third cross-language), threshold 0.7: qwen-27b 1.00 accuracy at 1.8 s per pair; qwen-moe 0.97 at 0.6 s, its one miss below the threshold; gemma-31b 0.97 at 1.7 s, its one miss a confident false alarm. Constant 0.33, overlap 0.53. The set is near its ceiling: it shows decide works, not which model is best.

## Decided 2026-11-02 — confirm, reject and memory

- `Store.Confirm(ns, id, user)` and `Store.Reject(ns, id, user)` replace `Store.Decide(ns, Decision)`. A decision's author can only be a user, and the log sets its time, so the struct asked the caller for one field with meaning. `Decision` and `Outcome` are now unexported. The line on disk is unchanged.
- Rejected: keeping the name `Decide`, because the `decide` package is the model's judgement and a confirmation is the owner's; a namespace handle (`store.Namespace(ns)`), because "every call names a `Namespace`" above is a recorded decision, and the repeated argument has cost nothing yet.
- `Append` refuses a claim whose `Namespace` is set and names another namespace, instead of filing it silently where the call says.
- The fold is kept per entry, not rebuilt per call. A confirmed claim takes what it supersedes or retracts out of the live set for good, even when it is itself superseded later. A claim is live when it is confirmed, not taken out, and no retraction. The order of confirmations does not matter, so an append checks links without a pass over the log.
- A log line is decoded once, into one struct that holds both a claim's and a decision's fields.
- Measured with `go test ./claim -bench .` at 2,000 claims in one namespace, before and after: `Live` 2.99 MB and 66 allocations per call to 0.41 MB and 6; `Recall` 6.25 MB to 2.23 MB; opening a namespace 5.4 MB to 3.3 MB; a proposal 124 KB and 40 allocations to 1.7 KB and 7, the same at 200 claims. `TestProposingCostsTheSameInASmallAndALargeNamespace` guards the last one.

## Decided 2026-11-02 — working tree

- As in git, the log file is the history and memory is the working tree. A namespace keeps the bodies of its live claims and its proposals; every other id keeps only its status (rejected, superseded or retracted, a retraction). A body leaves memory when its claim is rejected or taken out, during replay too.
- Readers take no lock. Each namespace publishes an immutable snapshot through an `atomic.Pointer`; a writer holds the mutex, builds the next snapshot and publishes it. Removing from a snapshot copies; appending may use spare capacity, because no published slice reaches past its own length. Open namespaces sit in a `sync.Map`; only opening one takes a lock.
- `Store.Live` and `Store.Proposals` return `iter.Seq[Claim]`. The error is only for opening the namespaces. A loop sees the snapshot from when it began, so a caller may append or confirm inside it, for example between two model calls.
- Live claims come in the order they became live, not in log order: a proposal confirmed late comes after claims confirmed before it. Replay keeps the same order. Recall's ties use `asof` and id, so they do not depend on it.
- Rejected: an iterator over the locked fold, because it holds the lock during a caller's model call and deadlocks when the caller appends; an `RWMutex`, because a reader still waits for a writer's sync; returning a copied slice, because the copy grows with the live set on every read.
- Measured with `go test ./claim -bench .` at 2,000 claims, before and after: `Live` 0.41 MB per call to 536 B; the heap held after opening 2,000 claims of which half are history 1.33 MB to 0.64 MB; `Recall` 2.23 MB to 1.82 MB. Opening allocates 3.3 MB to 3.6 MB, for the order kept during replay. `TestHistoryLeavesMemory` guards the held heap.

## Decided 2026-11-02 — content ids

- A claim's id is the lowercase hex SHA-256 (64 characters) of a fixed JSON encoding of its namespace, `text`, `trust`, `asof`, `source`, `party`, `quote`, `attributed`, `supersedes`, `challenges`, `retracted` and `author`. `recorded_at` is left out, and an empty list hashes like a missing one. As in git, the id names the record, not the meaning: two wordings of one fact are two claims, and `decide` judges whether they agree.
- A repeat is a retry: the same claim appended again writes nothing and never revives a claim. While it is live or pending, `Append` returns it as recorded. Once it is history, `Append` returns a `*HistoryError` (`errors.Is(err, ErrHistory)`, not `ErrInvalid`) with its state, `rejected`, `superseded`, `retracted` or `retraction`, and for the middle two the claim that did it. A repeated superseded claim means its source still states it unchanged: source drift for the host to show. A revert is no repeat: it supersedes the newer claim, and the links are part of the hash. A fact stated again after a mistaken retraction needs a new source or `asof`.
- To answer "by which claim", each superseded or retracted id keeps the id of the claim that took it out; the first one wins. At 2,000 claims of which half are history this holds 0.77 MB instead of 0.74 MB.
- Rejected: a content id and an entry id, as git's blob and commit, so that a repeat could state a fact again; retries would need a request id again, and every link would have to choose an id.
- A caller may leave the id empty; an id that differs from the hash is refused. Ids already in a log are read, never recomputed.
- Rejected: random ids with a request id for retries, because ids across nodes would stay open; ids from embeddings or a model, because they change with the model and break every citation; `recorded_at` in the hash, because a retry would get a new id.
- Memory: at 2,000 claims of which half are history, the heap held after open is 0.64 MB with 16-character random ids, 0.67 MB with 128-bit hashes (32 characters) and 0.74 MB with full SHA-256. Full length follows git; the difference is the id of every history entry.

## Decided 2026-11-24 — knotenpunkt owns the claims, behind a backend interface

- **knotenpunkt owns the claim spec and its storage.** Claims flow from a host into knotenpunkt, and knotenpunkt decides how they are stored. A host keeps no claim log of its own, so there is one truth for claims.
- Storage may sit behind an interface, as faden's `Store` does. The backends are knotenpunkt's: the JSONL files of "namespaces" first, an in-memory one for tests if a test needs it. The interface serves knotenpunkt's own backends, not a host that stores claims elsewhere. Not built: `claim` opens its files directly until a second backend is needed.
- A host writes its own records and its claims in two commits. A repeated `Append` is a retry and writes nothing ("content ids"), so a host appends after its own commit and appends again after a restart.
- Rejected: claims inside the host's log, with knotenpunkt as a library over it. The host would then own a copy of the spec and the second truth.

## Decided 2026-11-24 — what sidenote needs to embed the store

- **`Store.Check(ns, c)` says why `Append` would refuse c, and writes and creates nothing.** A retry of a live or pending claim passes; a claim that is history returns its `*HistoryError`. sidenote checks a harness's lines with it before anything is recorded.
- **`Store.Lookup(ns, id)` returns a claim's body while it is live or pending, and its `Standing`:** live, pending, or its `History` and the claim that took it out. It looks in ns itself, never in its org. `IDOf(ns, c)` is the id `Append` would record c under.
- **A proposal that repeats a live claim or an open proposal is refused:** equal text, ignoring case and spacing, and the same source system and ref, or both without a source. A claim is no repeat of one it supersedes or retracts. A user's own claim is never a repeat. The compare walks both texts and allocates nothing, so proposing still costs the same in a large namespace. Ported from sidenote's intake check, where it stopped intake run twice from doubling what the user confirmed.
- **A committed claim needs a party that is more than white space.**
- **`OpenStoreReadOnly(dir)` reads beside a writer.** It creates nothing, every write returns `ErrReadOnly`, and a namespace is read once, when a call first names it.
- **A last line without its newline is unfinished.** Replay skips it. A writable log cuts it off at open, so the next append starts on a clean line. Rejected: refusing to open, because a write cut off by a crash would then stop the host.
- **Checked by** `TestSameTextIgnoresCaseAndSpacing`, `TestARepeatInOtherWordsOfTheSameSourceIsRefused`, `TestCheckSaysWhatAppendWouldDoAndWritesNothing`, `TestLookupSaysWhereAClaimStands`, `TestAReadOnlyStoreReadsBesideAWriter`, `TestAWriterCutsOffAnUnfinishedLineAtOpen`; `TestProposingCostsTheSameInASmallAndALargeNamespace` still passes with the repeat check.
