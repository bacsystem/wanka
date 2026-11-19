"""Percentiles de duration_ms del log de acceso por ventana y ruta. Uso:
analyze.py ACCESS_LOG STEPS.jsonl  (cada línea: {"name","path","start","end"} en epoch s)"""
import json, sys, collections
log, steps = sys.argv[1], sys.argv[2]
from datetime import datetime
rows = []
for line in open(log):
    try:
        d = json.loads(line)
    except ValueError:
        continue
    if d.get("msg") != "request":
        continue
    t = datetime.fromisoformat(d["time"].replace("Z", "+00:00")).timestamp()
    rows.append((t, d.get("path"), d.get("status"), d.get("duration_ms", 0)))
rows.sort()
def pc(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(len(v) * p))] if v else None
print("| paso | ruta | n | req/s | p50 | p95 | p99 | max | 2xx | 4xx | 5xx |")
print("|---|---|---|---|---|---|---|---|---|---|---|")
for s in map(json.loads, open(steps)):
    w = [r for r in rows if s["start"] <= r[0] <= s["end"] and r[1] == s["path"]]
    dur = [r[3] for r in w]
    st = collections.Counter(r[2] // 100 for r in w)
    span = max(1.0, s["end"] - s["start"])
    print(f'| {s["name"]} | {s["path"]} | {len(w)} | {len(w)/span:.0f} | {pc(dur,.5)} | {pc(dur,.95)} | {pc(dur,.99)} | {max(dur) if dur else None} | {st[2]} | {st[4]} | {st[5]} |')
