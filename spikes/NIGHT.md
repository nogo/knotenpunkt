# Night run 2026-10-25 — plan

Throwaway spikes. Findings go to `docs/research/` when the run ends.

| # | Question | Data (author) | Run | Model |
|---|---|---|---|---|
| 0 | Does recall lose conflicts across languages? | synthetic vault, 96 proposals | `spikes/intake`, every claim of the namespace, overlap ranks recorded | qwen-moe |
| 1 | Does the intake check hold on a German-majority vault? | ACME + OrderApp translated (Codex) | `spikes/intake -vault testdata/synthetic-vault-de` | qwen-moe |
| 2 | Where does "no recall" stop working as a namespace grows? | 184 harmless claims for acme/migration (Codex) | `spikes/intake -pad`, sizes 16/50/100/200 from one run | qwen-moe |
| 3 | Does the question gate say "yes" only when the claims answer? | 6 source documents, 42 questions (Codex) | `spikes/gate -phase gate`, all claims vs top 5 | qwen-moe, qwen-27b, gemma-31b |
| 4 | Does the slow path answer from the document and leave a valid claim that flips the gate? | same | `spikes/gate -phase slow`, then `regate`, `grade` | qwen-27b (thinking), qwen-moe |
| 5 | Does new-first prompt order make the intake check faster without losing accuracy? | synthetic vault | `spikes/intake -order new-first` | qwen-moe |

Router: one model at a time, so the runs go in series: 0 → 5 → 1 → 2 → 3 (moe) → 4 slow (27b) → 3 (27b) → 3 (gemma) → 4 regate/grade (moe).

Labels from Codex are unverified by the owner. False alarms can be real conflicts nobody planned.
