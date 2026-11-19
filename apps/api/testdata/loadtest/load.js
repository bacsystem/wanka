// Generador de carga abierta (llegadas constantes) para testdata/loadtest/main.go.
import http from 'k6/http';
export const options = {
  scenarios: { c: { executor: 'constant-arrival-rate', rate: parseInt(__ENV.RATE), timeUnit: '1s', duration: '30s', preAllocatedVUs: 100, maxVUs: 400 } },
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
  discardResponseBodies: true,
};
export default function () { http.get(__ENV.URL, { headers: { 'X-Request-Id': 'bench-' + __VU } }); }
