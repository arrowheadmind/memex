#!/usr/bin/env python3
"""Build blind judging packets: per query, the union of both systems' top-10, shuffled, unlabeled."""
import json, os, random, re, sys
S=os.environ['REPLAY_SCRATCH']; STORE=os.environ['MEMEX_STORE']
res=json.load(open(sys.argv[1]))['results']; ent=json.load(open(os.path.join(S,'entries-index.json')))
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
    packets.append({'i':r['i'],'query':r['query'],'context':next(q['context'] for q in json.load(open(sys.argv[2])) if q['query']==r['query']),'items':items})
json.dump(packets,open(os.path.join(S,'packets.json'),'w'),indent=1,ensure_ascii=False)
json.dump(key,open(os.path.join(S,'packets-key.json'),'w'))
print(len(packets),'packets;',sum(len(p['items']) for p in packets),'items')
