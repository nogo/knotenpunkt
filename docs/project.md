# Knotenpunkt

## Vision

knotenpunkt is the claim: its format, its rules and its life from proposal to live to superseded. Every system that needs memory embeds it.

A claim is one fact with its trust, its source and the date it became true.
Memory grows from need: a question that the live claims cannot answer goes to a slow path, which proposes claims, and I confirm them.
The slow answers teach the fast path, so the same question costs less the next time.

Five earlier versions started with a code ingestion pipeline and a graph, and none of them reached real use (`history.md`). This version starts with the claim and ingests nothing.

## Outcome

**A claim layer that sidenote embeds, with a decide step.**

- **The claim:** one record type with `text`, `trust`, `source`, `asof`, and the links `supersedes` and `challenges`. Its rules are those of sidenote's claim system (`sidenote/docs/claim-system.md`), checked when a claim is written.
- **Store:** an append-only claim log. A correction is a new claim. The fold defines what is live.
- **Propose and confirm:** a system proposes claims; only I make a claim live.
- **Recall:** the live claims for a question.
- **Decide (System One):** a model scores a fixed list of options from log-probabilities, with the recalled claims in the prompt. It returns the probabilities, the claim ids it used, and `unsure` below a threshold. It reads and writes nothing.
- **Escalate (System Two):** on `unsure`, the calling system runs its own slow path (a reasoning model or a harness) and proposes claims from the result.

**First job:** sidenote's intake check. For each proposed claim, decide scores "agrees / contradicts / unrelated" against the live claims and flags possible `contested` claims before I confirm.

**Oracle before code:** a frozen set of claim pairs with known answers. The set comes from my work vault. Pairs linked by `supersedes` or `challenges` in its claims are "contradicts"; pairs from different topics are "unrelated". Decide must beat a constant answer on that set before sidenote calls it. After that, my Confirm and Reject keep measuring it.

**Measure:** sidenote holds no claim code of its own, and the intake check flags real contradictions more often than it raises false alarms.

**Goal after one node works — the mesh.** Each embedded knotenpunkt is a node. Nodes link into a mesh with no center, and a node reads the shared claims of another. Alpha built federation before anyone used it; the mesh waits until one node is in daily use.

## Value

- **Trust stays with the fact.** Every claim says what may be done with it and how to get back to its source.
- **Grown from need, not from bulk.** Nothing is ingested because it exists.
- **Cheap when it knows, slow only when it doesn't.** Reading a prompt is fast on my hardware; writing text is slow. Decide reads; escalation writes.
- **Each system keeps its own data.** A node is the truth for its own claims and shares only what it marks as shareable.

## Constraints

- **No ingestion.** knotenpunkt never parses code, docs or git, and holds no graph. A claim enters only through a proposal. Reading a source is the caller's job, on demand.
- **knotenpunkt is the only source of truth for claims.** A system that embeds it keeps no claim log of its own. sidenote uses knotenpunkt and writes no second log.
- **Nothing unattended writes or confirms.** Unattended writing decayed twice, measured. A system proposes; I confirm.
- **One source per extraction run.** It cut wrong citations from 39 % to 1/20.
- **A generated document proves what it says, not that it is true.** A claim from a generated source is attributed (*"X states …"*), and the fact stays `unverified` until a human source confirms it.
- **The source is opaque to knotenpunkt.** A claim stores `{system, ref, version}`. The owning system resolves the reference and reports drift. knotenpunkt never knows what a sidenote block is.
- **Harnesses stay in their systems.** A harness must run where its files are, so knotenpunkt runs no harness. The caller escalates.
- **Only the owner changes a claim.** Only the node that holds a claim supersedes or retracts it. Another node may challenge it.
- **Read, never copy.** A node reads another node's claims when it needs them. A copy would fork the truth.
- **Nothing is shared by default.** Work vault data must not leave its machine. That applies to links between nodes and to the model that decide calls.
- **Embeddable and small.** A Go package and a file format. Standard library first; follow sidenote's stack.

## Open questions

- **How the oracle runs on the Mac.** The work vault is the oracle; my personal vault grows from use and is not frozen. Work data stays on the Mac, so the pairs are built there, decide runs on a model there, and only scores and counts leave the machine. Which model the Mac can run is not checked.
- **How recall finds claims.** Plain text match first. exp-002 found the embedder was the strongest lever for code search; add embeddings only if they raise the oracle's score.
- **Transport between nodes.** A goal, not decided. The mesh rules above must hold whatever the transport is.
- **Claim ids across nodes.** An id that names its node, so a link needs no central registry. The format is not fixed.
- **Where decide runs for the work vault.** llama-server runs on friday, and work claims must not leave the Mac. Either a model runs on the Mac, or decide is off for that node.
- **The judge's reliability.** No System One model has yet beaten a constant answer: on 31 forge failures, always answering "provider" scores 0.65, the same as the best models. A judge that says "yes" too often gives confident wrong answers. Measure precision when it is confident, not accuracy.
- **One model or two.** The llama-server router holds one model at a time. Using the slow path's model as the judge keeps the prompt cache warm; a small judge model costs a switch.
- **The extraction from sidenote.** The claim types, rules and fold move to knotenpunkt; source resolution stays in sidenote. This changes sidenote's recorded decision that its log is the only truth.

## Parked

- forge as a consumer (failure causes with labels; a good judge test, but a classification task).
- invest's context problem: tool outputs reached 35k tokens. Context built from claims may help; a separate problem.
- Threads, meetings, entities and source cards from the claim system.
- A check of a diff against a project's `decisions.md`.
- An MCP read surface for harnesses (sidenote already serves claims read-only over MCP).
