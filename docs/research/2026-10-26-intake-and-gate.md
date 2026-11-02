# Intake check and question gate on synthetic vaults

Experiments of 2026-10-25/26 on friday (llama-server router, one model at a time). Spikes in `spikes/intake` and `spikes/gate`, plan in `spikes/NIGHT.md`. All data is invented: the synthetic vaults (`testdata/synthetic-vault`, `testdata/synthetic-vault-de`), plus Codex-written hard pairs, extra claims, source documents and questions. Codex's labels were not checked by the owner. Treat the numbers as direction: the claims are cleaner than real ones.

## Result

- **Decide works without recall inside a namespace.** Judging a proposal against every claim its namespace reads found 36 of 36 planned conflicts (qwen-moe, threshold 0.7), in an English-majority and a German-majority vault, at about 10 s per proposal for 16 claims.
- **Word-overlap recall must not gate decide.** Top-5 overlap hid a quarter of the cross-language conflicts (3 of 12 English-majority, 6 of 24 German-majority) and 5 of 6 cross-language gate answers.
- **Size costs time, not precision.** 184 harmless extra claims in one namespace caused no false alarm at any size up to 200; time grows linearly to about 2 minutes per proposal at 200 claims.
- **The question gate is precise.** No model said "yes" to a question the claims do not answer (0 of 24 for every model). qwen-moe answered 17 of 18 answerable questions with "yes" at 1.2 s.
- **The loop closes.** For 18 of 18 questions answerable only from the source document, qwen-27b with reasoning found the answer, quoted it verbatim, proposed a valid claim, and after confirmation the gate flipped from "no" to "yes". It invented no answer for the 6 unanswerable questions.

## Intake check (proposal against live claims)

96 proposals (36 labelled, 60 hard ones by Codex), 16 claims per namespace, threshold 0.7.

| run | conflicts found | off-target alarms | ms per call |
|---|---|---|---|
| qwen-moe, English-majority | 36/36 | 6 in 6 proposals | 608 |
| qwen-moe, German-majority | 35/36 | 9 in 8 proposals | 615 |
| qwen-27b, English-majority | 33/36 | 0 | 1,787 |

- At least 2 of qwen-moe's 6 alarms are real conflicts nobody planned (an "account required" proposal against "guest carts exist"; "sometimes keeps card numbers" against "PCI SAQ A").
- Threshold 0.9 cut alarms to 2 but lost 4 of 36 conflicts.
- Proposal first in the prompt gave no speed gain (605 ms either way): prompts this short gain nothing from the prompt cache.

## Question gate and slow path

42 questions over 6 engagements (18 answerable from claims, 18 only from the document, 6 from neither), 14 in German.

| gate, every claim | yes on answerable | yes on not answerable | ms |
|---|---|---|---|
| qwen-moe | 17/18 | 0/24 | 1,247 |
| qwen-27b | 18/18 | 0/24 | 5,653 |
| gemma-31b | 18/18 | 0/24 | 3,726 |

Top-5 overlap instead of every claim: 13/18 for all three models; cross-language 1 of 6.

Slow path (qwen-27b, thinking on, one document): 49 s and 210 written tokens per question (median). Found 18/18 document answers, 0/6 invented, all 30 quotes verbatim (10 across languages). The qwen-moe grader matched all 30 found answers to the expected ones; a plain substring check matched only 19, so grading needs the judge. All 30 proposed claims passed the claim rules.

## Limits

- Codex wrote the documents' extra facts as one paragraph of trivia (room names, paper size), so the slow path had it easy.
- One namespace for the size test; 16 proposals.
- The router was shared with another job for part of the run; ms values from those hours are high and were not used above.
