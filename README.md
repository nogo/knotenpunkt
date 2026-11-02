# knotenpunkt

A Go module that keeps facts ("claims") for a system and decides questions about them.
Hosts embed it: an app proposes a claim, a person confirms it, and from then on the claim is live until a newer claim supersedes or retracts it. A small local model then checks new proposals against the live claims, cheaply and without writing text.

"Knotenpunkt" is German for junction: one node of a later mesh of nodes.

## When to use it

Use knotenpunkt when **a system must remember facts that a person has confirmed, and must notice when a new fact contradicts an old one.**

| Use knotenpunkt when | Look elsewhere when |
|---|---|
| A fact must say how far it can be trusted and where it came from | You want to search documents or code: knotenpunkt ingests nothing |
| A person, not a model, decides what becomes true | A model should write memory on its own, unattended |
| Facts change, and you need to see what replaced what, and when | A key-value store with overwrite is enough |
| Facts of one client must never reach another | All data may mix in one index |
| Judgement must run on your own machine (private data) | You are happy to send every fact to a hosted API |

What it costs you: the standard library only, a directory for JSONL files, one writing process per store, and an OpenAI-compatible model server (llama.cpp) if you use `decide`.

## The idea in one picture

```
              host app (sidenote, ...)
                 |  Append(ns, claim by Proposer)
                 v
   +------------------------ claim.Store ------------------------+
   |  one append-only log per namespace, the only truth          |
   |  rules checked when a claim is written, never again         |
   |  fold: confirmed, and not superseded or retracted = live    |
   +------+-----------------------------------------+------------+
          | Proposals(ns)                           | Live(ns), Recall(ns, text)
          v                                         v
   a person confirms or rejects             decide.Client: one token,
   (Confirm, Reject)                        probabilities per option
                                                    |
                                     sure --------->+<-------- unsure
                                     answer                    |
                                                    the host's slow path
                                                    (reasoning model, harness)
                                                    proposes new claims
```

- A **claim** is one fact: `Text`, `Trust`, `Source`, `AsOf` (the date it became true), and the links `Supersedes` and `Challenges`.
- A **namespace** is a confidentiality boundary: an org, or an org and one engagement. An engagement reads its own claims and its org's claims, never a sibling's.
- **Live** means confirmed, and not superseded or retracted by a live claim. A correction is a new claim, never an edit.
- **Decide** is System One: it reads a prompt and answers with probabilities over a fixed list of options. Below your threshold it is `Unsure`, and the host escalates.

## Trust

Every claim says what may be done with it.

| Trust | Meaning | Rule when written |
|---|---|---|
| `committed` | Someone committed to it | Needs a `Party` and a source |
| `sourced` | A source states it | Needs a source |
| `derived` | Follows from other claims | Needs a source |
| `position` | Someone's view | Source optional |
| `unverified` | Not yet backed | Source optional |
| `contested` | Disputes another claim | Needs `Challenges` and a source |

The rules come from sidenote's claim system. A rule is checked once, when the claim is written.

## Packages

| Package | Use it for |
|---|---|
| `claim` | `Store`, `Claim`, `Namespace`, `Trust`, `Source`, `Cite`. Record, propose, confirm, correct and recall claims. |
| `decide` | `Client`, `Request`, `Result`. Ask a local model to pick one option, with probabilities. Writes nothing. |
| `cmd/knotenpunkt` | `score` measures decide against free baselines on labelled pairs; `oracle` builds pairs from a sidenote vault; `label` labels pairs by hand. |

## Quick start

Propose a claim, confirm it, then check a new proposal against the live claims:

```go
store, err := claim.OpenStore("/var/lib/myapp/claims")
if err != nil {
	return err
}
defer store.Close()

ns, err := claim.ParseNamespace("acme/orderapp")
if err != nil {
	return err
}

// A system proposes. The claim waits for a person.
p, err := store.Append(ns, claim.Claim{
	Text:   "The OrderApp cutover finishes before the December freeze.",
	Trust:  claim.Sourced,
	AsOf:   "2026-10-20",
	Source: &claim.Source{System: "myapp", Ref: "doc/42", Version: "v3"},
	Author: claim.Author{Kind: claim.Proposer, ID: "importer"},
})
if err != nil {
	return err
}

// A person confirms. Now the claim is live.
if err := store.Confirm(ns, p.ID, "jane-roe"); err != nil {
	return err
}

live, err := store.Live(ns)
if err != nil {
	return err
}

judge := &decide.Client{BaseURL: "http://localhost:8080", Model: "qwen-moe"}
proposal := "The last OrderApp site moves during the December freeze."
for _, c := range live {
	res, err := judge.Decide(ctx, decide.Request{
		Question: "Does the new claim conflict with the old one?",
		Options: []decide.Option{
			{ID: "conflicts", Description: "both cannot be true at once"},
			{ID: "agrees", Description: "the same fact, possibly with more detail"},
			{ID: "unrelated", Description: "different facts; both can be true"},
		},
		Context:   []string{"Old claim: " + c.Text, "New claim: " + proposal},
		Threshold: 0.7,
	})
	if errors.Is(err, decide.ErrNoOption) || (err == nil && res.Unsure) {
		continue // run your slow path
	}
	if err != nil {
		return err
	}
	fmt.Println(c.ID, res.Choice, res.Probs[res.Choice]) // ... conflicts 0.956
}
```

A claim by a `claim.User` is live at once. A claim by a `claim.Proposer` waits in `store.Proposals(ns)` until a person calls `store.Confirm` or `store.Reject`.
To correct a claim, append a new one with `Supersedes: []string{oldID}`. To carry a fact into another namespace, append a new claim there with `Source: claim.Cite(ns, id)`; never copy it.

## What the host owns

knotenpunkt never touches these, by design:

- reading sources: a `Source` is `{system, ref, version}`, and only the host resolves it and notices drift;
- the slow path: a reasoning model or harness runs where its files are, in the host;
- the model server, and which machine it runs on (private claims must not leave their machine);
- the screen where a person confirms, and who may read which namespace;
- one writing process per store: the log takes no file lock.

## Measured

On synthetic vaults of invented claims (`testdata/`), on one machine with llama.cpp, in October 2026. Codex wrote the hard pairs, and their labels are not yet checked by hand. Details: `docs/research/2026-10-26-intake-and-gate.md`.

- 36 labelled pairs: qwen-27b 1.00 accuracy, qwen-moe 0.97 (0.6 s per pair), gemma-31b 0.97. A constant answer scores 0.33, word overlap 0.53.
- Judging a proposal against every live claim of its namespace found 36 of 36 planned conflicts (qwen-moe, threshold 0.7), in an English-majority and a German-majority vault.
- Word-overlap recall as a filter hid a quarter of the conflicts across languages. Do not let `Recall` decide what `decide` sees.
- 184 harmless extra claims caused no false alarm. Time grows linearly: about 2 minutes per proposal at 200 claims.

## Similar projects

Surveyed briefly in November 2026, from their descriptions, not from running them.

| Project | What it is | Pick it over knotenpunkt when |
|---|---|---|
| [Graphiti](https://pypi.org/project/graphiti-core/) (Python) | A temporal knowledge graph for agent memory. Facts carry valid and invalid times; a contradicting fact invalidates the old one instead of deleting it. | You want a graph built from conversations and documents, unattended |
| [Mem0](https://docs.mem0.ai/) (Python) | A memory layer that extracts facts from conversations into a vector store. Earlier versions let a model choose add, update or delete per fact. | You want memory written by a model, with semantic search |
| [membrane](https://github.com/GustyCube/membrane) | An episodic memory layer for agents, with decay, revision and trust gating | You want memory that fades with time |

What knotenpunkt does differently:

- **A person makes a claim live.** A model proposes and judges; it never confirms.
- **Trust and source stay with the fact,** checked by rules when the claim is written.
- **Namespaces are boundaries, not filters.** Each namespace has its own log, so one missed filter cannot leak a client's facts.
- **The judge reads, it does not write.** One token of a local model, with probabilities, and `Unsure` when it is not sure.

What knotenpunkt does not do (yet): the intake check as one call (today the host writes the loop above), links between nodes, embeddings for recall, more than one writing process per store.

## Status

Personal project, young, untagged; the API will change. No host embeds it yet; sidenote is the first.
The reasons behind each choice are in `docs/decisions.md`; the goal is in `docs/project.md`; why five earlier versions stalled is in `docs/history.md`.
