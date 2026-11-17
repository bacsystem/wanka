# ADR 0007: Despliegue en Railway con imágenes Docker, portable a una nube mayor

- Estado: aceptado (Railway es decisión del usuario); topología propuesta
- Fecha: 2026-11-17

## Contexto

Arranque con poco tráfico (supuestos en [`capacity.md`](../capacity.md)), equipo pequeño, migración futura a una nube mayor. Contenedores: `web` (Next.js), `api`, `platform-api`, `worker` (Go), PostgreSQL, almacenamiento de objetos.

## Opciones

1. **Railway con imágenes Docker y servicios separados** por proceso.
2. **Railway con buildpacks (Nixpacks)**: menos archivos, pero la imagen no es portable tal cual.
3. **Nube mayor desde el día uno**: más control, más costo y operación antes de tener carga.

## Decisión

Opción 1. Diagrama y detalles: [`deployment/railway.mmd`](../deployment/railway.mmd), [`deployment/cloud-migration.md`](../deployment/cloud-migration.md).

- Dos `Dockerfile` multi-stage: `apps/web` y `apps/api`. La imagen Go sirve para `api`, `platform-api`, `webhooks`, `worker` y `migrate` (subcomandos).
- 12-factor: configuración solo por variables de entorno (lista de nombres en el diagrama), logs JSON a stdout, sin archivos locales persistentes, puertos por `PORT`.
- Red: Railway expone el **puerto completo** de un servicio con dominio público; no filtra por ruta. Por eso la frontera es por servicio:
  - `web` es el único dominio público de la app.
  - `webhooks` (misma imagen, `serve --plane=webhooks`) tiene su propio dominio público, y el binario en ese modo **solo monta** `/webhooks/khipu/{routeId}`, `/livez` y `/readyz`. Usa el rol `wanka_webhooks` (ADR 0001).
  - `api` y `platform-api` **no tienen dominio público**: solo son alcanzables por `*.railway.internal` desde `web`. Así `/v1/*` no se puede llamar desde internet aunque alguien conozca la URL.
  - Si más adelante se quiere un solo servicio público, haría falta un proxy con filtro por ruta delante (CDN/WAF o Caddy); mientras tanto, el servicio separado es lo más simple.
- Réplicas iniciales: `web` 2, `api` 1, `platform-api` 1, `worker` 1, `webhooks` 1. Todo es sin estado; escalar es subir réplicas. Al pasar `api` a 2+ réplicas, el rate limit en memoria se vuelve por réplica (límite efectivo ×N): se mueve a Postgres (tabla de buckets con `UPDATE ... RETURNING`) o a Redis si la medición lo pide, más rate limit en el borde (CDN/WAF).
- Health: `/livez` sin dependencias, `/readyz` con Postgres; apagado ordenado con plazo 20 s (< `drainingSeconds` de Railway).
- Pools explícitos por servicio vía `DB_POOL_MAX_CONNS` (inicial `api` 20, `worker` 10, `platform-api` 5), sumando por debajo del máximo de conexiones del Postgres de Railway (a verificar por plan).
- Entornos: `staging` y `production` como proyectos Railway separados; PR previews opcionales sin datos reales.

## Consecuencias

- Buenas: operación mínima; mismas imágenes en cualquier nube; despliegues con migraciones previas y corte por readiness.
- Malas: dependencia de las capacidades de Postgres gestionado de Railway (réplicas de lectura, PITR, extensiones: a verificar); una sola región; límites de plan de Railway por confirmar.
- Qué haría cambiar la decisión: ver disparadores en `cloud-migration.md` (PITR/réplicas, cumplimiento, costo, multirregión).
