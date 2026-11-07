import json, re, sys
md = open(sys.argv[1]).read()
layout = json.load(open(sys.argv[2]))
entries = {e['path']: e for e in layout['paths'] if e['root'] == 'home' and e['documented']}
sections = ('## File reference', '## Application data')
rows, insec, header = [], False, None
code = re.compile(r'`([^`]+)`')
def cells(l): return [c.strip() for c in l.strip().strip('|').split('|')]
for line in md.splitlines():
    if line.startswith('## '):
        insec = line.startswith(sections); header = None; continue
    if not insec: continue
    if line.startswith('|'):
        c = cells(line)
        if all(set(x) <= set('-: ') for x in c): continue
        if header is None or not code.search(c[0]):
            header = [h.lower() for h in c]; continue
        if 'scope' in header and 'project only' in c[header.index('scope')].lower(): continue
        desc_col = next((i for i,h in enumerate(header) if h in ('what it does','contents')), 1)
        desc = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', c[desc_col])
        for p in code.findall(c[0]):
            rows.append((p, desc))
    elif re.match(r'\s*[*-] ', line):
        m = re.match(r'\s*[*-] (?:\*\*)?`([^`]+)`(?:\*\*)?(?: and `([^`]+)`)?(?:\*\*)?:\s*(.*)', line)
        if m:
            desc = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', m.group(3))
            for p in (m.group(1), m.group(2)):
                if p: rows.append((p, desc))
def norm(p):
    p = p.strip()
    if p == '~/.claude.json': return '.claude.json'
    p = p.removeprefix('~/.claude/').rstrip('/')
    out = []
    for comp in p.split('/'):
        if '<' in comp or '*' in comp: break
        out.append(comp)
    return '/'.join(out)
items, seen = [], set()
for p, desc in rows:
    if 'memory' in p: continue          # data inside the state folder projects/: label would be wrong
    n = norm(p)
    if n in entries and n not in seen:
        seen.add(n); items.append({'doc_path': p, 'path': n, 'description': desc, 'expected': entries[n]['category']})
json.dump(items, open(sys.argv[3], 'w'), indent=1)
from collections import Counter
print(len(items), 'items', dict(Counter(i['expected'] for i in items)))
missing = sorted(set(entries) - seen)
print('layout entries with no doc row:', missing)
