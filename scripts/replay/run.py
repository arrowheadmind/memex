#!/usr/bin/env python3
"""Replay harness: run memex search and brainctl preflight for each query; map both to memex ids."""
import json, os, re, subprocess, sys, time, unicodedata, glob
S=os.environ.get('REPLAY_SCRATCH')
STORE=os.environ['MEMEX_STORE']; MEMEX=os.environ['MEMEX_BIN']; BRAIN=os.path.expanduser('~/Programming/brain')
queries=json.load(open(sys.argv[1])); outp=sys.argv[2]
RAW=os.path.join(S,'raw'); os.makedirs(RAW,exist_ok=True)

def slug(t):
    t=unicodedata.normalize('NFKD',t); t=''.join(c for c in t if unicodedata.category(c)!='Mn').lower()
    t=re.sub(r'[^a-z0-9]+','-',t).strip('-'); return t or 'entry'
PREFIX=re.compile(r'^(Mistake / technique|Mistake|Known Noise|Known Problem|Technique|Pattern|Historical mistake[^:]*):\s*',re.I)
def strip_md(t): return re.sub(r'[`*_~]|\[([^\]]*)\]\([^)]*\)',lambda m:m.group(1) or '',t)

# index store: id -> meta; alias -> id; slug(title) -> id
entries={}; alias={}; tslug={}
for p in glob.glob(STORE+'/*/*.md'):
    typ=os.path.basename(os.path.dirname(p)); eid=os.path.basename(p)[:-3]
    fm=open(p,encoding='utf-8').read().split('\n+++\n',1)[0]
    m=re.search(r'^title = "(.*)"$',fm,re.M); title=json.loads('"'+m.group(1)+'"') if m else eid
    st=re.search(r'^status = "(.*)"$',fm,re.M).group(1)
    sm=re.search(r'^summary = "(.*)"$',fm,re.M); summ=json.loads('"'+sm.group(1)+'"') if sm else ''
    al=re.search(r'^aliases = \[(.*)\]$',fm,re.M)
    als=[json.loads(a) for a in re.findall(r'"(?:[^"\\]|\\.)*"',al.group(1))] if al else []
    entries[eid]={'type':typ,'title':title,'status':st,'summary':summ,'path':typ+'/'+eid+'.md'}
    for a in als: alias[a]=eid
    tslug.setdefault(slug(title),eid)
# fragment slug index: '.brain/known-mistakes.md#frag' -> frag->id
frag={}
for a,eid in alias.items():
    if '#' in a: frag.setdefault(a.split('#',1)[1],eid)

def map_heading(text, file):
    t=strip_md(PREFIX.sub('',text)).strip()
    for cand in (file+'#'+slug(t), file+'#'+slug(strip_md(text))):
        if cand in alias: return alias[cand]
    s=slug(t)
    if s in tslug: return tslug[s]
    # prefix match on fragments
    for fr,eid in frag.items():
        if fr.startswith(s[:50]) or s.startswith(fr[:50]): return eid
    for ts,eid in tslug.items():
        if ts.startswith(s[:40]) or s.startswith(ts[:40]): return eid
    return None
def map_path(p):
    p=p.strip().replace(BRAIN+'/','')
    for cand in (p, p.replace('.brain/',''), 'docs/'+p):
        if cand in alias: return alias[cand]
    base=os.path.basename(p)[:-3] if p.endswith('.md') else os.path.basename(p)
    if base in entries: return base
    if base in alias: return alias[base]
    return None

SECTION=re.compile(r'^## (Always|Known Mistakes|Runbooks|Decisions|Plans|Constraints|Local matches|Global matches|Approximate matches|Widened matches)')
def parse_brainctl(out):
    """Return ordered list of (id or None, label) surfaced by query-dependent sections."""
    items=[]; sect=None; seen=set()
    for line in out.splitlines():
        m=SECTION.match(line)
        if m: sect=m.group(1); continue
        if sect in (None,'Always'):
            # after Constraints table, other '## X' headings are constraint sections
            if line.startswith('## ') and sect is None: pass
            continue
        if sect=='Known Mistakes' and line.startswith('## '):
            items.append((map_heading(line[3:],'.brain/known-mistakes.md'),line[3:120]))
        elif sect in ('Runbooks','Decisions','Plans') and line.startswith('- '):
            items.append((map_path(line[2:]),line[2:120]))
        elif sect=='Constraints':
            if line.startswith('| ') and not line.startswith('|---') and not line.startswith('| Constraint'):
                cell=line[2:].split(' | ')[0].strip()
                items.append((map_heading(cell,'.brain/constraints.md'),cell[:120]))
            elif line.startswith('## '):
                items.append((map_heading(line[3:],'.brain/constraints.md'),line[3:120]))
        elif sect in ('Local matches','Global matches','Approximate matches','Widened matches') and line.startswith('| 20'):
            cells=line.split('|'); 
            if len(cells)>2:
                fn=cells[2].strip(); items.append((map_path('.brain/investigations/'+fn),fn[:120]))
    out=[]
    for eid,label in items:
        key=eid or label
        if key in seen: continue
        seen.add(key); out.append({'id':eid,'label':label})
    return out

results=[]
for i,q in enumerate(queries):
    qs=q['query']; tag='%03d'%i
    t0=time.time()
    r=subprocess.run([MEMEX,'search','--limit','10','--json',qs],capture_output=True,text=True,env={**os.environ})
    tm=time.time()-t0
    open(f'{RAW}/{tag}-memex.json','w').write(r.stdout); open(f'{RAW}/{tag}-memex.err','w').write(r.stderr)
    try: mj=json.loads(r.stdout)
    except Exception: mj={'results':[],'more':0,'hidden':0,'excluded':0}
    r2=subprocess.run([MEMEX,'search','--limit','10','--json','--all',qs],capture_output=True,text=True)
    try: mj2=json.loads(r2.stdout)
    except Exception: mj2={'results':[]}
    t0=time.time()
    b=subprocess.run(['brainctl','preflight','--no-sync',qs],capture_output=True,text=True,cwd=BRAIN)
    tb=time.time()-t0
    open(f'{RAW}/{tag}-brainctl.txt','w').write(b.stdout+b.stderr)
    bitems=parse_brainctl(b.stdout)
    results.append({'i':i,'query':qs,'expected_ids':q.get('expected_ids',[]),
        'memex':{'ids':[x['id'] for x in mj['results']],'matched':[x.get('matched') for x in mj['results']],'exit':r.returncode,'more':mj.get('more'),'hidden':mj.get('hidden'),'excluded':mj.get('excluded'),'lines':r.stdout.count('\n'),'chars':len(r.stdout),'secs':round(tm,3)},
        'memex_all':{'ids':[x['id'] for x in mj2['results']]},
        'brainctl':{'ids':[x['id'] for x in bitems if x['id']],'unmapped':[x['label'] for x in bitems if not x['id']],'n_items':len(bitems),'lines':b.stdout.count('\n'),'chars':len(b.stdout),'secs':round(tb,3),'empty':'No past knowledge found' in b.stdout}})
    print(tag,qs[:60],'| memex',len(mj['results']),'| brainctl',len(bitems),'unmapped',len([x for x in bitems if not x['id']]),file=sys.stderr)
json.dump({'store_entries':len(entries),'results':results},open(outp,'w'),indent=1,ensure_ascii=False)
json.dump(entries,open(os.path.join(S,'entries-index.json'),'w'))
