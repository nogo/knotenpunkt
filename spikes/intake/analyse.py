"""Compares recall strategies from one intake run:
python3 -I -B analyse.py results.jsonl [vault] [pad.jsonl]"""
import json, re, sys, collections

rows = [json.loads(l) for l in open(sys.argv[1])]
de = lambda t: bool(re.search(r"\b(der|die|das|und|laut|werden|wird|nicht|ist)\b", t.lower()))

# Language of every text, from the vault and the hard candidates.
lang = {}
vault = sys.argv[2] if len(sys.argv) > 2 else "testdata/synthetic-vault"
import glob
for f in glob.glob(vault + "/log/*/*.jsonl"):
    for l in open(f):
        r = json.loads(l)
        if r["type"] == "claim":
            lang[r["id"]] = de(r["text"])
if len(sys.argv) > 3:
    for l in open(sys.argv[3]):
        r = json.loads(l)
        lang["c-" + r["id"]] = de(r["text"])
for l in open(vault + "/hard/candidates.jsonl"):
    r = json.loads(l)
    lang[r["id"]] = de(r["new"])

by_prop = collections.defaultdict(list)
for r in rows:
    by_prop[r["proposal"]].append(r)

def arm(name, keep):
    stats = collections.Counter()
    for pid, rs in by_prop.items():
        p = rs[0]
        cross = "cross" if lang[pid] != lang[p["target"]] else "same"
        shown = [r for r in rs if keep(r)]
        stats["calls"] += len(shown)
        flagged = [r for r in shown if r["pred"] == "conflicts"]
        target_in = any(r["is_target"] for r in shown)
        if p["label"] == "conflicts":
            stats[f"conflict_props_{cross}"] += 1
            stats[f"target_shown_{cross}"] += target_in
            stats[f"hit_{cross}"] += any(r["is_target"] for r in flagged)
        else:
            stats["clean_props"] += 1
            stats["clean_props_flagged"] += bool(flagged)
        stats["false_alarms"] += sum(not (r["is_target"] and p["label"] == "conflicts") for r in flagged)
        stats["props_with_false_alarm"] += any(not (r["is_target"] and p["label"] == "conflicts") for r in flagged)
    n = len(by_prop)
    print(f"{name:<12} calls/prop={stats['calls']/n:5.1f}"
          f"  conflict hit same={stats['hit_same']}/{stats['conflict_props_same']} cross={stats['hit_cross']}/{stats['conflict_props_cross']}"
          f"  target shown same={stats['target_shown_same']}/{stats['conflict_props_same']} cross={stats['target_shown_cross']}/{stats['conflict_props_cross']}"
          f"  false alarms={stats['false_alarms']} in {stats['props_with_false_alarm']}/{n} proposals")

arm("all", lambda r: True)
for k in (5, 3, 1):
    arm(f"overlap-{k}", lambda r, k=k: 0 < r["rank"] <= k)
ms = sorted(r["ms"] for r in rows)
print(f"ms per call median={ms[len(ms)//2]}  proposals={len(by_prop)}")
