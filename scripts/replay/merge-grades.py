import json, glob, os, sys
S = os.path.dirname(os.path.abspath(__file__)) + "/replay"
key = json.load(open(S + "/packets-key.json"))
merged = {}
for f in sorted(glob.glob(S + "/sonnet-raw-*.json")):
    raw = json.load(open(f))
    for qi, items in raw.items():
        merged.setdefault(qi, {})
        for n, g in items.items():
            eid = key[qi].get(str(n))
            if eid is None:
                print("unknown item", qi, n, file=sys.stderr); continue
            merged[qi][eid] = int(g)
json.dump(merged, open(S + "/grades-sonnet.json", "w"), indent=1)
fork = json.load(open(S + "/grades.json"))
# agreement on the union of graded items (unlisted = 0)
agree = total = exact2 = 0; conf = {}
for qi in key:
    ids = set(key[qi].values())
    for eid in ids:
        a = fork.get(qi, {}).get(eid, 0); b = merged.get(qi, {}).get(eid, 0)
        total += 1; agree += (a == b); conf[(a, b)] = conf.get((a, b), 0) + 1
print("packets graded by sonnet:", len(merged), "of", len(key))
print("item agreement fork vs sonnet: %d/%d = %.2f" % (agree, total, agree / total))
print("confusion (fork,sonnet)->count:", dict(sorted(conf.items())))
