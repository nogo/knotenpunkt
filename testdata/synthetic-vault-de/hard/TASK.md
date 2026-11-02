# Write hard test pairs for a claim-conflict judge

`seeds.jsonl` holds 90 claims (id, namespace, lang, text) about three invented clients: ACME, OrderApp and Globex. A judge reads an OLD claim and a NEW claim and answers one of:

- `conflicts`: both cannot be true at once.
- `agrees`: the new claim states the same fact as the old one, possibly with more detail or other wording.
- `unrelated`: different facts; both can be true.

The judges already score about 97% on easy pairs, where a conflict changes one number or date in an otherwise copied sentence. Write pairs that are hard, but whose correct label is still clear to a careful human reading only the two claims.

Write `candidates.jsonl` in this directory: exactly 60 lines, one JSON object each:

{"id": "h01", "seed": "<seed id>", "namespace": "<seed namespace>", "kind": "<kind>", "intended": "<label>", "new": "<new claim text>"}

The old claim is the seed. Use each seed at most once. Spread the seeds over all six namespaces.

Kinds, 12 pairs each:

1. `implicit-conflict` (intended `conflicts`): shares no restated fact, but both cannot be true; noticing it needs one step of reasoning. Example idea: old says the cutover must finish before the freeze; new says the last site moves during the freeze.
2. `small-flip` (intended `conflicts`): one or two words change the meaning: must/may, at least/at most, before/after, only/also, inside/outside, never/sometimes. Keep the sentence natural, not a copy with one word swapped.
3. `harmless-rewording` (intended `agrees`): many words differ, numbers may be expressed differently (14 days / two weeks, 0.3 s / 300 ms), the fact is the same.
4. `refinement` (intended `agrees`): adds a detail that is compatible with the old claim and could look like a change at first glance.
5. `same-numbers-other-fact` (intended `unrelated`): same client, reuses the old claim's numbers, dates or names, but about a different fact that can be true at the same time.

Language: write 20 of the 60 new claims in the other language than their seed (German seed → English new claim, English seed → German), spread over all kinds. German should be natural business German.

Style: like the seeds. One sentence, 120 to 220 characters, attributed to a document ("The revised test plan states that ...", "Laut Protokoll ..."). Only the invented names already in the seeds. No explanation, no hint at the label inside the text.

When done, check the file: 60 lines, valid JSON, each kind 12 times, 20 cross-language, no seed used twice. Print only those counts.
