// Generador de carga abierta para session/main.go. Reparte las peticiones entre
// todos los tokens sembrados (turno rotatorio: cada sesión recibe RATE/N req/s),
// así ninguna sesión concentra la carga y no se mide el lock de una sola fila.
// Sin TOKENS (control) pide URL sin Authorization.
//
//	RATE=300 DUR=30s URL=http://127.0.0.1:18090/v1/loadtest/ping TOKENS=tokens.txt k6 run load.js
import http from 'k6/http';
import exec from 'k6/execution';
import { SharedArray } from 'k6/data';
import { Counter } from 'k6/metrics';

const tokens = new SharedArray('tokens', () => (__ENV.TOKENS ? open(__ENV.TOKENS).split('\n').filter(Boolean) : []));
const byCode = new Counter('by_code'); // status + code del problema
const dropped = new Counter('dropped'); // llegadas que k6 no pudo lanzar por falta de VUs

export const options = {
  scenarios: {
    c: {
      executor: 'constant-arrival-rate',
      rate: parseInt(__ENV.RATE), timeUnit: '1s', duration: __ENV.DUR || '30s',
      preAllocatedVUs: parseInt(__ENV.VUS || '200'), maxVUs: parseInt(__ENV.MAXVUS || '3000'),
    },
  },
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  const i = exec.scenario.iterationInTest;
  const headers = { 'X-Request-Id': 'lt-' + __VU + '-' + i };
  if (tokens.length) headers.Authorization = 'Bearer ' + tokens[i % tokens.length];
  const res = http.get(__ENV.URL, { headers, timeout: '10s', responseCallback: http.expectedStatuses({ min: 200, max: 599 }) });
  let code = '';
  if (res.status !== 200) {
    try { code = res.json('code') || ''; } catch (e) { code = 'nobody'; }
  }
  byCode.add(1, { status: String(res.status), code });
}
