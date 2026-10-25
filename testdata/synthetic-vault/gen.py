"""Writes the synthetic vault from claims.py: a sidenote log with invented
claims, labels.jsonl with the expected answer for every open proposal, and
pairs.jsonl with the same proposals in the oracle's pair format.

Run from this directory: python3 -I -B gen.py
"""
import json, os, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from claims import PAGES, CORRECTIONS, PROPOSALS, NAMESPACES, ORG_LEVEL, LOOKALIKES

HERE = os.path.dirname(os.path.abspath(__file__))
LOG = os.path.join(HERE, "log", "synthetic")
SESSION = {"kind": "session", "id": "session-synthetic"}
USER = {"kind": "user", "id": "user-synthetic"}
page_of = {}
decisions = 0


def namespace(key, page):
    """The claim's namespace: its page's engagement, or the org for ORG_LEVEL."""
    ns = NAMESPACES[page]
    return ns.split("/")[0] if key in ORG_LEVEL else ns


def claim(key, text, trust, party, attributed, asof, page, supersedes=None, ns=None):
    c = {"type": "claim", "id": "c-" + key, "namespace": ns or namespace(key, page),
         "text": text, "trust": trust, "asof": asof}
    if trust not in ("unverified", "position"):
        c["source"] = {"kind": "block", "ref": "blk-" + key, "page": page, "version": "v1"}
    if party:
        c["party"] = party
    if attributed:
        c["attributed"] = True
    if supersedes:
        c["supersedes"] = ["c-" + supersedes]
    c["author"] = SESSION
    c["recorded_at"] = asof + "T10:00:00Z"
    return c


def confirm(c):
    global decisions
    decisions += 1
    return {"type": "claim_decision", "id": "d-%04d" % decisions, "claim": c["id"],
            "confirm": True, "author": USER, "recorded_at": c["recorded_at"]}


def write(day, records):
    with open(os.path.join(LOG, day + ".jsonl"), "w") as f:
        for r in records:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")


os.makedirs(LOG, exist_ok=True)
live = []
for page, entries in PAGES.items():
    for key, text, trust, party, attributed in entries:
        page_of[key] = page
        c = claim(key, text, trust, party, attributed, "2026-09-01", page)
        live += [c, confirm(c)]
write("2026-09-01", live)

fixes = []
for key, old, text, trust, party, attributed in CORRECTIONS:
    c = claim(key, text, trust, party, attributed, "2026-09-15", page_of[old], old)
    fixes += [c, confirm(c)]
write("2026-09-15", fixes)

texts = {k: t for e in PAGES.values() for k, t, *_ in e}
open_ = []
labels = []
pairs = []
for key, target, label, kind, text, trust, party, attributed in PROPOSALS:
    open_.append(claim(key, text, trust, party, attributed, "2026-10-01", page_of[target]))
    labels.append({"proposal": "c-" + key, "namespace": NAMESPACES[page_of[target]],
                   "target": "c-" + target, "target_namespace": namespace(target, page_of[target]),
                   "label": label, "kind": kind})
    pairs.append({"id": key, "old": texts[target], "new": text, "label": label, "why": kind})
# Lookalikes across orgs: not in pairs.jsonl, because they must never be compared.
ns_page = {ns: page for page, ns in NAMESPACES.items()}
corrected = {k: old for k, old, *_ in CORRECTIONS}
for key, ns, lookalike, text, trust, party, attributed in LOOKALIKES:
    open_.append(claim(key, text, trust, party, attributed, "2026-10-01", ns_page[ns], ns=ns))
    base = corrected.get(lookalike, lookalike)
    labels.append({"proposal": "c-" + key, "namespace": ns, "target": "c-" + lookalike,
                   "target_namespace": namespace(base, page_of[base]),
                   "label": "other-namespace", "kind": "lookalike"})
write("2026-10-01", open_)

with open(os.path.join(HERE, "labels.jsonl"), "w") as f:
    for l in labels:
        f.write(json.dumps(l) + "\n")
# The same proposals in the oracle's pair format, for `knotenpunkt score`.
with open(os.path.join(HERE, "pairs.jsonl"), "w") as f:
    for pr in pairs:
        f.write(json.dumps(pr, ensure_ascii=False) + "\n")
print(f"live={sum(len(e) for e in PAGES.values())} corrections={len(CORRECTIONS)} "
      f"proposals={len(PROPOSALS)} lookalikes={len(LOOKALIKES)} org_level={len(ORG_LEVEL)}")
