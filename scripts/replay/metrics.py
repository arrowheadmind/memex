#!/usr/bin/env python3
"""Compute replay metrics from results.json (+ grades.json if present)."""
import argparse, json, math, os, sys, statistics as st
ap=argparse.ArgumentParser(description='Replay metrics for a results file, graded with DATA/grades.json.')
ap.add_argument('--data', default=os.environ.get('REPLAY_DATA'), help='replay data directory (default $REPLAY_DATA)')
ap.add_argument('results', nargs='?', help='results file (default DATA/results.json)')
args=ap.parse_args()
if not args.data: ap.error('--data (or REPLAY_DATA) is required')
res=json.load(open(args.results or os.path.join(args.data,'results.json')))['results']
gp=os.path.join(args.data,'grades.json')
grades=json.load(open(gp)) if os.path.exists(gp) else {}
def rank(ids,exp):
    for i,x in enumerate(ids):
        if x in exp: return i+1
    return None
exp=[r for r in res if r['expected_ids']]
print('## Expected-id queries (n=%d)'%len(exp))
print('| system | hit@1 | hit@5 | hit@10 | median rank of hits | not found |'); print('|---|---|---|---|---|---|')
for sysn,key in (('memex search',('memex','ids')),('memex search --all',('memex_all','ids')),('brainctl preflight',('brainctl','ids'))):
    ranks=[rank(r[key[0]][key[1]],set(r['expected_ids'])) for r in exp]
    hits=[x for x in ranks if x]
    print('| %s | %d | %d | %d | %s | %d |'%(sysn,sum(1 for x in ranks if x==1),sum(1 for x in ranks if x and x<=5),sum(1 for x in ranks if x and x<=10),st.median(hits) if hits else '-',sum(1 for x in ranks if not x)))
print('\n| query | expected | memex rank | memex --all | brainctl rank |'); print('|---|---|---|---|---|')
for r in exp:
    e=set(r['expected_ids'])
    print('| %s | %s | %s | %s | %s |'%(r['query'],r['expected_ids'][0][:40],rank(r['memex']['ids'],e) or '-',rank(r['memex_all']['ids'],e) or '-',rank(r['brainctl']['ids'],e) or '-'))
print('\n## Output size and time, all queries (n=%d)'%len(res))
print('| system | median lines | median chars | max chars | median items | median secs | empty/no-match |'); print('|---|---|---|---|---|---|---|')
m=[r['memex'] for r in res]; b=[r['brainctl'] for r in res]
print('| memex search --limit 10 | %d | %d | %d | %d | %.2f | %d exit 3 |'%(st.median(x['lines'] for x in m),st.median(x['chars'] for x in m),max(x['chars'] for x in m),st.median(len(x['ids']) for x in m),st.median(x['secs'] for x in m),sum(1 for r in res if r['memex']['exit']==3)))
print('| brainctl preflight --no-sync | %d | %d | %d | %d | %.2f | %d "No past knowledge" |'%(st.median(x['lines'] for x in b),st.median(x['chars'] for x in b),max(x['chars'] for x in b),st.median(x['n_items'] for x in b),st.median(x['secs'] for x in b),sum(1 for x in b if x['empty'])))
print('\nbrainctl items unmapped to a memex id: %d of %d'%(sum(len(x['unmapped']) for x in b),sum(x['n_items'] for x in b)))
if grades:
    def dcg(gs): return sum((2**g-1)/math.log2(i+2) for i,g in enumerate(gs))
    rows=[]
    for r in res:
        g=grades.get(str(r['i']))
        if not g: continue
        out={}
        for sysn,key in (('memex',('memex','ids')),('brainctl',('brainctl','ids'))):
            ids=r[key[0]][key[1]][:10]; gs=[g.get(x,0) for x in ids]
            ideal=sorted([v for v in g.values()],reverse=True)[:10]
            out[sysn]={'p5':sum(1 for x in gs[:5] if x==2)/5,'ndcg':(dcg(gs)/dcg(ideal)) if ideal and dcg(ideal)>0 else 0,'top2':1 if gs and gs[0]==2 else 0,'any2':1 if 2 in gs else 0}
        rows.append((r,out))
    print('\n## Graded queries (n=%d, blind grades 0/1/2)'%len(rows))
    print('| system | mean P@5 (grade 2) | mean nDCG@10 | top result grade 2 | any grade 2 in top 10 |'); print('|---|---|---|---|---|')
    for sysn in ('memex','brainctl'):
        print('| %s | %.2f | %.2f | %d | %d |'%(sysn,st.mean(o[sysn]['p5'] for _,o in rows),st.mean(o[sysn]['ndcg'] for _,o in rows),sum(o[sysn]['top2'] for _,o in rows),sum(o[sysn]['any2'] for _,o in rows)))
    diffs=sorted(rows,key=lambda ro:ro[1]['memex']['ndcg']-ro[1]['brainctl']['ndcg'])
    print('\n### memex worst relative to brainctl (nDCG delta)'); 
    for r,o in diffs[:10]: print('- %d | %s | memex %.2f brainctl %.2f'%(r['i'],r['query'][:70],o['memex']['ndcg'],o['brainctl']['ndcg']))
    print('\n### memex best relative to brainctl')
    for r,o in diffs[-10:][::-1]: print('- %d | %s | memex %.2f brainctl %.2f'%(r['i'],r['query'][:70],o['memex']['ndcg'],o['brainctl']['ndcg']))
