#!/bin/sh
# Escalones de carga con control pegado a cada medición. Requiere el entorno de
# README.md (LOAD_*, TOKENS_FILE) y OUT (carpeta de resultados).
# Uso: OUT=dir RATES="50 100 200 300 500 700 1000" sh run.sh
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
: "${OUT:?}" "${TOKENS_FILE:?}"
RATES=${RATES:-"50 100 200 300 500 700 1000"}
DUR=${DUR:-30s}
SECS=${DUR%s}
mkdir -p "$OUT"
export ACCESS_LOG="$OUT/access.log" DB_POOL_MAX_CONNS=${DB_POOL_MAX_CONNS:-20}
(cd "$HERE/../../.." && go build -o "$OUT/loadsrv" ./testdata/loadtest/session)
"$OUT/loadsrv" serve >"$OUT/server.out" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null || true' EXIT INT TERM
sleep 2
: >"$OUT/steps.jsonl"; : >"$OUT/load.txt"; : >"$OUT/stats.jsonl"
BASE=http://127.0.0.1:18090
run() { # nombre ruta rate [tokens]
  n=$1; p=$2; r=$3; tk=${4:-}
  curl -s "http://127.0.0.1:18091/stats?reset=1" >/dev/null
  s=$(python3 -c 'import time;print(time.time())')
  RATE=$r DUR=$DUR URL="$BASE$p" TOKENS=$tk k6 run --quiet --summary-export "$OUT/k6-$n.json" "$HERE/load.js" >"$OUT/k6-$n.txt" 2>&1 || true
  e=$(python3 -c 'import time;print(time.time())')
  printf '{"name":"%s","path":"%s","start":%s,"end":%s}\n' "$n" "$p" "$s" "$e" >>"$OUT/steps.jsonl"
  printf '{"name":"%s",' "$n" >>"$OUT/stats.jsonl"; curl -s "http://127.0.0.1:18091/stats" | cut -c2- >>"$OUT/stats.jsonl"
}
# Calentamiento (pool y caché de la base), sin medir.
RATE=100 DUR=5s URL="$BASE/v1/loadtest/ping" TOKENS=$TOKENS_FILE k6 run --quiet "$HERE/load.js" >/dev/null 2>&1 || true
for r in $RATES; do
  echo "== $r req/s: $(uptime | sed 's/.*load/load/') ; contenedores: $(docker ps -q | wc -l | tr -d ' ')" | tee -a "$OUT/load.txt"
  run "s$r" /v1/loadtest/ping "$r" "$TOKENS_FILE"
  run "c$r" /ctl "$r"
done
curl -s http://127.0.0.1:18091/flush >/dev/null
sleep 1
python3 -I "$HERE/analyze.py" "$OUT/access.log" "$OUT/steps.jsonl" | tee "$OUT/table.md"
