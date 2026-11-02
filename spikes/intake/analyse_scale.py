"""False alarms and cost as a namespace grows, from one padded intake run:
python3 -I -B analyse_scale.py scale.jsonl distractors.jsonl"""
import json, sys, collections

rows = [json.loads(l) for l in open(sys.argv[1])]
order = {"c-" + json.loads(l)["id"]: i for i, l in enumerate(open(sys.argv[2]))}
base = len({r["candidate"] for r in rows if r["candidate"] not in order})
by_prop = collections.defaultdict(list)
for r in rows:
    by_prop[r["proposal"]].append(r)
print(f"proposals={len(by_prop)} original claims={base} padding={len(order)}")
for size in (base, 50, 100, 200):
    extra = size - base
    stats = collections.Counter()
    for pid, rs in by_prop.items():
        shown = [r for r in rs if r["candidate"] not in order or order[r["candidate"]] < extra]
        p = rs[0]
        stats["calls"] += len(shown)
        stats["ms"] += sum(r["ms"] for r in shown)
        flagged = [r for r in shown if r["pred"] == "conflicts"]
        if p["label"] == "conflicts":
            stats["conflicts"] += 1
            stats["hit"] += any(r["is_target"] for r in flagged)
        fa = [r for r in flagged if not (r["is_target"] and p["label"] == "conflicts")]
        stats["fa"] += len(fa)
        stats["fa_pad"] += sum(r["candidate"] in order for r in fa)
        stats["props_fa"] += bool(fa)
        stats["fa_conf"] += sum(r["probs"].get("conflicts", 0) >= 0.7 for r in fa)
    n = len(by_prop)
    print(f"size={size:<4} hit={stats['hit']}/{stats['conflicts']} false_alarms={stats['fa']} (on padding {stats['fa_pad']}, confident {stats['fa_conf']})"
          f" proposals_with_alarm={stats['props_fa']}/{n} seconds_per_proposal={stats['ms']/n/1000:.1f}")
