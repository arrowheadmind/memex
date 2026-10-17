#!/usr/bin/env python3
"""memex-only replay against the current store (MEMEX_STORE, memex on PATH); compares with the B4
results.json using the B4 grades. Usage: rerun.py [--data DIR] OUT.json > report.md. Items the B4 judges never
saw count as grade 0, so a lower score after curation is a prompt to re-grade, not a regression."""
import argparse, json, math, os, subprocess, sys, statistics as st
ap=argparse.ArgumentParser(description='memex-only rerun of the replay queries, compared with DATA/results.json.')
ap.add_argument('--data', default=os.environ.get('REPLAY_DATA'), help='replay data directory (default $REPLAY_DATA)')
ap.add_argument('out', help='new results file to write')
args=ap.parse_args()
if not args.data: ap.error('--data (or REPLAY_DATA) is required')
R=args.data
queries=json.load(open(R+'/queries.json')); old=json.load(open(R+'/results.json'))['results']
grades=json.load(open(R+'/grades.json')); gs2=json.load(open(R+'/grades-sonnet.json')) if os.path.exists(R+'/grades-sonnet.json') else {}
# DATA/grades-additions-*.json: grades for items judged after the original packets ({qi: {id: grade}}); overlaid on both sets.
import glob
adds=sorted(glob.glob(R+'/grades-additions-*.json'))
for f in adds:
    for qi,items in json.load(open(f)).items():
        for gset in (grades,gs2):
            if qi in gset: gset[qi].update(items)
env={**os.environ}
new=[]
for i,q in enumerate(queries):
    r=subprocess.run(['memex','search','--limit','10','--json',q['query']],capture_output=True,text=True,env=env)
    try: mj=json.loads(r.stdout)
    except Exception: mj={'results':[]}
    new.append({'i':i,'query':q['query'],'expected_ids':q.get('expected_ids',[]),'ids':[x['id'] for x in mj['results']],'exit':r.returncode,'chars':len(r.stdout)})
json.dump(new,open(args.out,'w'),indent=1)
def rank(ids,exp):
    for i,x in enumerate(ids):
        if x in exp: return i+1
def dcg(gs): return sum((2**g-1)/math.log2(i+2) for i,g in enumerate(gs))
def report(grades,label):
    rows=[]
    for o,n in zip(old,new):
        g=grades.get(str(o['i']))
        if not g: continue
        ideal=sorted(g.values(),reverse=True)[:10]; idd=dcg(ideal) or 1
        def m(ids):
            gsl=[g.get(x,0) for x in ids[:10]]; return {'ndcg':dcg(gsl)/idd,'top2':int(bool(gsl) and gsl[0]==2),'any2':int(2 in gsl),'ungraded':sum(1 for x in ids[:10] if x not in g)}
        rows.append((o['query'],m(o['memex']['ids']),m(n['ids'])))
    print(f'## Graded queries, {label} (n={len(rows)})')
    print('| run | mean nDCG@10 | top result grade 2 | any grade 2 in top 10 | ungraded items in top 10 |'); print('|---|---|---|---|---|')
    for k,name in ((1,'B4 (store at import)'),(2,'now')):
        print('| %s | %.3f | %d | %d | %d |'%(name,st.mean(r[k]['ndcg'] for r in rows),sum(r[k]['top2'] for r in rows),sum(r[k]['any2'] for r in rows),sum(r[k]['ungraded'] for r in rows)))
    ch=[r for r in rows if abs(r[1]['ndcg']-r[2]['ndcg'])>0.01]
    print(f'\nqueries whose nDCG changed: {len(ch)}')
    for q,a,b in sorted(ch,key=lambda r:r[2]['ndcg']-r[1]['ndcg']): print('- %s | %.2f -> %.2f'%(q[:70],a['ndcg'],b['ndcg']))
exp=[(o,n) for o,n in zip(old,new) if o['expected_ids']]
if adds: print('grade additions applied: '+', '.join(os.path.basename(f) for f in adds)+'\n')
print('## Expected-id queries (n=%d)'%len(exp)); print('| run | hit@1 | hit@5 | hit@10 | not found |'); print('|---|---|---|---|---|')
for name,ids in (('B4',[o['memex']['ids'] for o,_ in exp]),('now',[n['ids'] for _,n in exp])):
    rk=[rank(x,set(o['expected_ids'])) for x,(o,_) in zip(ids,exp)]
    print('| %s | %d | %d | %d | %d |'%(name,sum(1 for x in rk if x==1),sum(1 for x in rk if x and x<=5),sum(1 for x in rk if x and x<=10),sum(1 for x in rk if not x)))
for o,n in exp:
    a=rank(o['memex']['ids'],set(o['expected_ids'])); b=rank(n['ids'],set(o['expected_ids']))
    if a!=b: print('- %s | %s -> %s'%(o['query'][:70],a or '-',b or '-'))
print()
report(grades,'implementer grades')
if gs2: print(); report(gs2,'Sonnet grades')
same=sum(1 for o,n in zip(old,new) if o['memex']['ids']==n['ids']); print(f'\nidentical top-10 lists: {same}/{len(new)}')
