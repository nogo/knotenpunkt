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

## Decided 2026-11-02 — confirm and reject

- `Store.Confirm(ns, id, user)` and `Store.Reject(ns, id, user)` replace `Store.Decide(ns, Decision)`. A decision's author can only be a user, and the log sets its time, so the struct asked the caller for one field with meaning. `Decision` and `Outcome` are now unexported. The line on disk is unchanged.
- Rejected: keeping the name `Decide`, because the `decide` package is the model's judgement and a confirmation is the owner's; a namespace handle (`store.Namespace(ns)`), because "every call names a `Namespace`" above is a recorded decision, and the repeated argument has cost nothing yet.
- `Append` refuses a claim whose `Namespace` is set and names another namespace, instead of filing it silently where the call says.
