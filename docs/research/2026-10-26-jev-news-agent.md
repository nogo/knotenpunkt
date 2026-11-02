# jev in a proactive news agent — what applies to decide

Read on 2026-10-26: `github.com/disler/pi-durable-news-agent` (IndyDevDan, MIT, video of 2026-10-19 "Pi Durable Agent curates news: hype, fact or signal"). Only the README, the jev package and the jev commands were read. The video transcript was not available. Nothing was run, so no number below is measured here.

## What the project is

A news agent built on pi-durable (Earendil's TypeScript harness). A cron tick wakes it and it reads about 28 sources. A cascade of cheap judges filters the items, and an expensive agent writes cards only for the items that survive. The author's formula is "proactive agent = durable agent + trigger + sandbox".

## jev is System One as a service

jev is TypeSafe's "System One" decision API (`api.typesafe.ai/v1/systemone`, also on OpenRouter as `~typesafe/jev-latest`). It is the same idea as `decide`: the model reads a prompt and answers with probabilities, not text. The README states about $0.00004 a call and under half a second.

A request is a `state` (the text or JSON being judged) and a map of named questions. There are three question types:

| type | criteria | answer |
|---|---|---|
| `noul` (yes/no) | optional text for yes and no | `noul`: probability of yes |
| `choice` | option → description, up to 255 | `choice`, `probabilities`, `confidence` |
| `score` | 2–10 ordered levels, low to high | `score`: probability-weighted position between levels, `probabilities`, `confidence` |

`decide` already covers all three. Yes/no is a choice with two options. A score is a choice over ordered levels, read as the expected level (Σ p·level). So the gap is a helper, not a new model call.

## The gate cascade

Each stage drops items before the next stage, so the expensive model sees only what survives:

1. `relevance`: is the item on topic?
2. `worth-attention`: is it worth an engineer's attention today? (yes/no, policy `worth_min: 0.35`, plus a novelty score)
3. `same-story`: does it tell a story that already has a card?
4. `importance`: a score, multiplied by the topic weight, the source weight and freshness. The top 7 survive.
5. `claim-support`: during research, does a source support a claim?
6. `hfs`: three scores (hype, facts, signal) on the excerpt and the page text. A combined `attention` weighting decides the order: signal 0.5, facts 0.35, hype −0.25, relevance 0.4.

## Commands are files

Each question set is a JSON file (`prompt_engineering/jev/<name>/command.json`) with a `name`, a `version`, a `state` template, the `questions` and a `policy` (thresholds and weights). The prompt, the options and the threshold change without a code change, and the version names what produced a stored answer.

## Taste

- "More" and "less" on a card move the matching topic weights by +0.10 and −0.15, and they move the card's source weight too.
- A hype, facts or signal label moves the keywords of that section by 0.10.
- The six most recent votes go into the judge's state as `taste_examples`, as few-shot examples.
- If the scores show hype and signal but no facts, the agent asks the reader directly.

## What applies to knotenpunkt

- **Decide is the right primitive.** A second, independent project put a System One judge at every gate and kept the reasoning model behind it. This supports "cheap when it knows, slow only when it doesn't".
- **A score helper.** Ordered levels read as an expected value make `decide` usable for "how much" questions without a new API.
- **Questions as versioned files.** Today the conflict question lives in `cmd/knotenpunkt`. A versioned question file, with its threshold in the same file, would let a caller own its questions and let a result name the question version that produced it.
- **Contested means ask.** jev's "hype and signal without facts" is the same pattern as `contested`: the judge does not settle a disagreement between its own signals, it asks the owner.
- **Taste as claims, not weights.** jev's taste is numbers that drift, and few-shot examples that fall out after six votes. In knotenpunkt a rejection could be a proposal such as "the owner ignores vendor launch posts without a release". The owner confirms it, and recall puts it into the next decision. This is slower than a weight, but it is readable, it has a source and it can be superseded.
- **Not taken:** the hosted judge. Work-vault data must not leave the machine, so `decide` keeps calling the local llama-server.

## Not measured by the project

The repository tests durability (a killed sweep resumes and pays once), not judgement. No precision or false-alarm rate for the gates is published. The README itself says "the model can still choose badly". knotenpunkt's rule holds: an oracle before trust.
