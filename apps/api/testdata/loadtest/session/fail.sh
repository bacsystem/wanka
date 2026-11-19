#!/bin/sh
# Prueba de fallo: carga RATE req/s sobre la ruta Tenant; a los PRE s se detiene el
# contenedor de la base durante DOWN s y se vuelve a levantar. Uso:
# OUT=dir CONTAINER=nombre RATE=300 PRE=20 DOWN=30 POST=40 sh fail.sh
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
: "${OUT:?}" "${TOKENS_FILE:?}" "${CONTAINER:?}"
RATE=${RATE:-300}; PRE=${PRE:-20}; DOWN=${DOWN:-30}; POST=${POST:-40}
TOTAL=$((PRE+DOWN+POST+10))
mkdir -p "$OUT"
export ACCESS_LOG="$OUT/access.log" DB_POOL_MAX_CONNS=${DB_POOL_MAX_CONNS:-20}
(cd "$HERE/../../.." && go build -o "$OUT/loadsrv" ./testdata/loadtest/session)
"$OUT/loadsrv" serve >"$OUT/server.out" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null || true; docker start "$CONTAINER" >/dev/null 2>&1 || true' EXIT INT TERM
sleep 2
RATE=100 DUR=5s URL=http://127.0.0.1:18090/v1/loadtest/ping TOKENS=$TOKENS_FILE k6 run --quiet "$HERE/load.js" >/dev/null 2>&1 || true
T0=$(python3 -c 'import time;print(time.time())')
RATE=$RATE DUR=${TOTAL}s URL=http://127.0.0.1:18090/v1/loadtest/ping TOKENS=$TOKENS_FILE k6 run --quiet --out json="$OUT/k6-points.json" --summary-export "$OUT/k6-fail.json" "$HERE/load.js" >"$OUT/k6-fail.txt" 2>&1 &
K6=$!
sleep "$PRE"
D0=$(python3 -c 'import time;print(time.time())'); docker stop -t 1 "$CONTAINER" >/dev/null
sleep "$DOWN"
D1=$(python3 -c 'import time;print(time.time())'); docker start "$CONTAINER" >/dev/null
U=$(python3 -c 'import time;print(time.time())')
wait $K6 || true
curl -s http://127.0.0.1:18091/flush >/dev/null; sleep 1
printf '{"t0":%s,"stop":%s,"start":%s,"up":%s}\n' "$T0" "$D0" "$D1" "$U" >"$OUT/fail-times.json"
python3 -I "$HERE/timeline.py" "$OUT/access.log" "$OUT/fail-times.json" | tee "$OUT/fail-timeline.md"
