import json, glob, re, sys, os, shlex
out=[]
for f in glob.glob(os.path.expanduser('~/.claude/projects/*/*.jsonl')):
    proj=os.path.basename(os.path.dirname(f))
    last_user=''
    try:
        lines=open(f,encoding='utf-8',errors='replace').read().splitlines()
    except Exception: continue
    for ln in lines:
        try: o=json.loads(ln)
        except Exception: continue
        m=o.get('message') or {}
        role=m.get('role') or o.get('type')
        content=m.get('content')
        if role=='user' and isinstance(content,str) and content.strip():
            last_user=content.strip()[:300]
        if isinstance(content,list):
            for c in content:
                if not isinstance(c,dict): continue
                if role=='user' and c.get('type')=='text' and c.get('text','').strip():
                    last_user=c['text'].strip()[:300]
                if c.get('type')=='tool_use' and c.get('name')=='Bash':
                    cmd=(c.get('input') or {}).get('command','')
                    for mm in re.finditer(r'brainctl preflight((?:\s+--?[\w-]+)*)\s+(.+)',cmd):
                        rest=mm.group(2).strip()
                        try: parts=shlex.split(rest)
                        except Exception: parts=[rest]
                        parts=[p for p in parts if not p.startswith('-')]
                        q=' '.join(parts).strip()
                        q=q.split('\n')[0].split('&&')[0].split('|')[0].split('2>')[0].strip().strip('"\'')
                        if not q or len(q)>200: continue
                        out.append({'query':q,'date':(o.get('timestamp') or '')[:10],'project':proj,'context':last_user.replace('\n',' ')[:300],'session':os.path.basename(f)[:8]})
json.dump(out,open(sys.argv[1],'w'),indent=1,ensure_ascii=False)
print(len(out),'raw;',len({x['query'].lower() for x in out}),'distinct')
