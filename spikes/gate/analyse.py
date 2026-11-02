"""Summarises the gate spike. Counts and rates only, never text.

python3 -I -B analyse.py --gate gate-*.jsonl --slow slow.jsonl --regate regate.jsonl --grade grade.jsonl
Every option is optional and takes one or more files.
"""
import argparse, collections, json, statistics

p = argparse.ArgumentParser()
for name in ("gate", "slow", "regate", "grade"):
    p.add_argument("--" + name, nargs="*", default=[])
a = p.parse_args()


def rows(paths):
    for path in paths:
        for line in open(path):
            if line.strip():
                yield json.loads(line)


def rate(n, d):
    return f"{n}/{d} ({n / d:.2f})" if d else "0/0"


def cross(r):
    """Same or cross language: globex claims are German, the others English."""
    claims_lang = "de" if r["namespace"].startswith("globex") else "en"
    return "same" if r.get("lang") == claims_lang else "cross"


def median(v):
    return statistics.median(v) if v else 0


gate = list(rows(a.gate))
if gate:
    print("== gate (S1: do these claims answer the question?)")
    groups = collections.defaultdict(list)
    for r in gate:
        groups[(r["model"], r["variant"])].append(r)
    for (model, variant), rs in sorted(groups.items()):
        print(f"-- {model} {variant}: {len(rs)} questions, ms median {median([r['ms'] for r in rs]):.0f}, claims median {median([r['claims'] for r in rs]):.0f}")
        for cls in ("claims", "document", "none"):
            sub = [r for r in rs if r["answerable"] == cls]
            print(f"   yes-rate answerable={cls:<8} {rate(sum(r['choice'] == 'yes' for r in sub), len(sub))}")
        for lang in ("same", "cross"):
            sub = [r for r in rs if cross(r) == lang]
            yes = [r for r in sub if r["choice"] == "yes"]
            conf = [r for r in yes if max(r["probs"].values()) >= 0.7]
            truly = [r for r in sub if r["answerable"] == "claims"]
            print(f"   {lang:<5} yes precision {rate(sum(r['answerable'] == 'claims' for r in yes), len(yes))}"
                  f"  at>=0.7 {rate(sum(r['answerable'] == 'claims' for r in conf), len(conf))}"
                  f"  recall of answerable {rate(sum(r['choice'] == 'yes' for r in truly), len(truly))}")
        yes = [r for r in rs if r["choice"] == "yes"]
        conf = [r for r in yes if max(r["probs"].values()) >= 0.7]
        print(f"   all   yes precision {rate(sum(r['answerable'] == 'claims' for r in yes), len(yes))}"
              f"  at>=0.7 {rate(sum(r['answerable'] == 'claims' for r in conf), len(conf))}")

slow = list(rows(a.slow))
if slow:
    print("== slow (S2: answer from the document, propose a claim)")
    by_model = collections.defaultdict(list)
    for r in slow:
        by_model[r["model"]].append(r)
    for model, rs in sorted(by_model.items()):
        ok = [r for r in rs if not r.get("err") and not r.get("parse_err")]
        print(f"-- {model}: {len(rs)} questions, errors {len(rs) - len(ok)}, ms median {median([r['ms'] for r in ok]):.0f},"
              f" completion tokens median {median([r['completion_tokens'] for r in ok]):.0f}")
        for cls in ("claims", "document", "none"):
            sub = [r for r in ok if r["answerable"] == cls]
            print(f"   found-rate answerable={cls:<8} {rate(sum(r['found'] for r in sub), len(sub))}")
        found = [r for r in ok if r["found"]]
        print(f"   quote verbatim {rate(sum(r['quote_verbatim'] for r in found), len(found))}")
        none = [r for r in ok if r["answerable"] == "none"]
        print(f"   false answer on 'none' {rate(sum(r['found'] for r in none), len(none))}")
        for lang in ("same", "cross"):
            sub = [r for r in found if cross(r) == lang]
            print(f"   {lang:<5} quote verbatim {rate(sum(r['quote_verbatim'] for r in sub), len(sub))}")

grade = list(rows(a.grade))
if grade:
    print("== grade (S1 grader on S2 answers)")
    for cls in ("claims", "document", "none"):
        sub = [r for r in grade if r["answerable"] == cls]
        found = [r for r in sub if r["found"]]
        print(f"   answerable={cls:<8} correct {rate(sum(r['correct'] for r in sub), len(sub))}"
              f"  decide match {rate(sum(r['choice'] == 'match' for r in found), len(found))}"
              f"  substring match {rate(sum(r['substring'] for r in found), len(found))}")

regate = list(rows(a.regate))
if regate:
    print("== regate (gate again after confirming the slow claim)")
    acc = [r for r in regate if r["accepted"]]
    print(f"   accepted {rate(len(acc), len(regate))}")
    reasons = collections.Counter(r["refusal"].split(":")[-1].strip()[:60] for r in regate if not r["accepted"])
    for reason, n in reasons.most_common():
        print(f"   refused {n}: {reason}")
    for cls in ("claims", "document", "none"):
        sub = [r for r in acc if r["answerable"] == cls]
        print(f"   answerable={cls:<8} before yes {rate(sum(r['before'] == 'yes' for r in sub), len(sub))}"
              f"  after yes {rate(sum(r['choice'] == 'yes' for r in sub), len(sub))}"
              f"  flipped no->yes {rate(sum(r['before'] != 'yes' and r['choice'] == 'yes' for r in sub), len(sub))}")
