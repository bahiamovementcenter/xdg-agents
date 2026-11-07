import json, os, sys, time, urllib.request
items = [i for i in json.load(open(sys.argv[1])) if i['path'] not in ('plugins','todos','statsig','logs')]
variant = sys.argv[2]  # "desc" or "path"
criteria = {
 "config": "User configuration: settings and files the user writes or chooses to customise the program",
 "data": "User data that must survive and is worth backing up, and can't be regenerated",
 "state": "Data that persists between runs but isn't worth backing up: history, logs, recent items, session records, snapshots",
 "cache": "Non-essential data that can be regenerated or fetched again, and is safe to delete",
 "runtime": "Files that only matter while a session is running, such as sockets, locks and per-session markers removed on exit",
}
out = []
for it in items:
    state = {"program": "Claude Code (an AI coding agent)", "documentation": it['description']}
    if variant == 'path': state["path"] = "~/.claude/" + it['path'] if it['path'] != '.claude.json' else "~/.claude.json"
    body = {"model": "jev-latest", "state": state, "questions": {"category": {"type": "choice",
        "instructions": "Under the XDG Base Directory Specification, which category does this file or folder belong to?",
        "criteria": criteria}}}
    req = urllib.request.Request("https://api.typesafe.ai/v1/systemone", data=json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + os.environ["TYPESAFE_API_KEY"], "Content-Type": "application/json"})
    t0 = time.time()
    r = json.load(urllib.request.urlopen(req, timeout=60))
    a = r["answers"]["category"]
    out.append({**it, "model": r["model"], "choice": a["choice"], "confidence": a["confidence"],
                "probabilities": a["probabilities"], "ms": int((time.time()-t0)*1000), "usage": r["usage"]})
json.dump(out, open(sys.argv[3], 'w'), indent=1)
ok = sum(o['choice'] == o['expected'] for o in out)
print(f"{variant}: {ok}/{len(out)} agree; model {out[0]['model']}; median {sorted(o['ms'] for o in out)[len(out)//2]} ms")
