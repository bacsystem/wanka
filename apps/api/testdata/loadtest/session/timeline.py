"""Línea de tiempo por segundo (status del log de acceso) de la prueba de fallo."""
import json, sys, collections
from datetime import datetime
log, times = sys.argv[1], json.load(open(sys.argv[2]))
sec = collections.defaultdict(collections.Counter)
for line in open(log):
    try:
        d = json.loads(line)
    except ValueError:
        continue
    if d.get("msg") != "request" or d.get("path") != "/v1/loadtest/ping":
        continue
    t = datetime.fromisoformat(d["time"].replace("Z", "+00:00")).timestamp()
    if t >= times["t0"]:
        sec[int(t - times["stop"] // 1)][d.get("status")] += 1
print("t = segundos desde el docker stop. 'start' = docker start emitido en t=%.1f" % (times["start"] - times["stop"]))
print("| t | total | 200 | 503 | 401 | 500 | otros | %err |")
print("|---|---|---|---|---|---|---|---|")
for k in sorted(sec):
    c = sec[k]; tot = sum(c.values()); o = tot - c[200] - c[503] - c[401] - c[500]
    print(f"| {k} | {tot} | {c[200]} | {c[503]} | {c[401]} | {c[500]} | {o} | {100*(tot-c[200])/tot:.1f} |")
