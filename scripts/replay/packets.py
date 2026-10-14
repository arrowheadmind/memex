#!/usr/bin/env python3
"""Build blind judging packets: per query, the union of both systems' top-10, shuffled, unlabeled."""
import argparse, json, os, random, re, sys
ap=argparse.ArgumentParser(description='Blind judging packets into $REPLAY_SCRATCH.')
ap.add_argument('--data', default=os.environ.get('REPLAY_DATA'), help='replay data directory (default $REPLAY_DATA)')
ap.add_argument('results', nargs='?', help='results file (default DATA/results.json)')
args=ap.parse_args()
if not args.data: ap.error('--data (or REPLAY_DATA) is required')
S=os.environ['REPLAY_SCRATCH']; STORE=os.environ['MEMEX_STORE']
res=json.load(open(args.results or os.path.join(args.data,'results.json')))['results']; ent=json.load(open(os.path.join(S,'entries-index.json')))
QUERIES=json.load(open(os.path.join(args.data,'queries.json')))
rnd=random.Random(20261013)
def body(eid):
    p=os.path.join(STORE,ent[eid]['path']); t=open(p,encoding='utf-8').read().split('\n+++\n',1)[1]
    t=re.sub(r'\s+',' ',t).strip(); return t[:300]
packets=[]; key={}
for r in res:
    if r['expected_ids']: continue
    ids=list(dict.fromkeys(r['memex']['ids'][:10]+r['brainctl']['ids'][:10]))
    rnd.shuffle(ids)
    items=[{'n':n+1,'title':ent[e]['title'],'summary':ent[e]['summary'],'body':body(e)} for n,e in enumerate(ids) if e in ent]
    key[str(r['i'])]={str(n+1):e for n,e in enumerate(ids) if e in ent}
    packets.append({'i':r['i'],'query':r['query'],'context':next(q['context'] for q in QUERIES if q['query']==r['query']),'items':items})
json.dump(packets,open(os.path.join(S,'packets.json'),'w'),indent=1,ensure_ascii=False)
json.dump(key,open(os.path.join(S,'packets-key.json'),'w'))
print(len(packets),'packets;',sum(len(p['items']) for p in packets),'items')
