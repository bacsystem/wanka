# Despliegue: Railway primero, nube mayor después

Diagrama: [`railway.mmd`](railway.mmd). Decisión: [ADR 0007](../adr/0007-deployment-railway.md).

## Railway (inicio)

- Un proyecto Railway por entorno (`staging`, `production`); cada uno con los servicios `web`, `api`, `platform-api`, `webhooks`, `worker`, un Postgres gestionado y un bucket S3 compatible (bucket de Railway o un proveedor S3 externo; a decidir con el tech-lead, ver supuestos).
- Dos imágenes Docker: `web` (Next.js `output: "standalone"`, Node LTS) y `wanka` (binario Go estático sobre `distroless/static`). `api`, `platform-api` y `worker` son la **misma imagen** con distinto comando, así lo que se prueba es lo que corre.
- Railway publica el puerto completo de un servicio, sin filtrar por ruta. Por eso `api` y `platform-api` no tienen dominio público y solo se alcanzan por `*.railway.internal` desde `web`. Los webhooks de Khipu entran por un servicio aparte, `webhooks`: la misma imagen en `serve --plane=webhooks`, que solo monta `/webhooks/khipu/{routeId}` y los health checks. Opcional: CDN/WAF delante para rate limit en el borde.
- Migraciones como **pre-deploy command** de `api` (`wanka migrate up`); si fallan, el despliegue no avanza.
- Health checks: `GET /livez` (proceso vivo, no toca la base) y `GET /readyz` (ping a Postgres con timeout 1 s, River iniciado). Railway usa `readyz` para el corte de tráfico en despliegues.
- Apagado ordenado: ante `SIGTERM`, `readyz` pasa a 503, `http.Server.Shutdown` con plazo 20 s, el worker deja de tomar jobs y espera los que están en curso (River `StopAndCancel` tras el plazo). El `drainingSeconds` de Railway se configura mayor que ese plazo.
- Secretos: variables de entorno por servicio (solo nombres en el diagrama). Cada servicio recibe únicamente las suyas: `platform-api` no tiene `DATABASE_URL_APP` y `api` no tiene `DATABASE_URL_PLATFORM`.
- Backups: los del Postgres gestionado + `pg_dump` diario lógico a object storage de otro proveedor (prueba de restauración mensual). RPO/RTO objetivo a fijar con producto.
- Escala: inicialmente 1 réplica de `api`, 1 de `worker`, 2 de `web`. Como las capas son sin estado (sesiones en Postgres, jobs en Postgres, archivos en S3), subir réplicas no exige rediseño; lo único que cambia es el rate limit en memoria (ver ADR 0007).

## Camino a una nube mayor (AWS, GCP o Azure)

12-factor desde el día uno: configuración por entorno, logs a stdout, procesos sin estado, puertos por variable, backing services por URL. Por eso la migración es un cambio de infraestructura, no de código:

| Railway | Equivalente en nube | Nota |
|---|---|---|
| Servicios Docker | ECS Fargate / Cloud Run / Container Apps (o Kubernetes si el equipo ya lo opera) | mismas imágenes |
| Postgres gestionado | RDS / Cloud SQL / Azure Database for PostgreSQL | migración con replicación lógica y corte en ventana corta; extensiones usadas: `citext`, `pg_trgm`, `pgcrypto` (verificar disponibilidad) |
| Bucket S3 compatible | S3 / GCS (API S3 o adaptador) / Blob | el puerto `blob.Store` aísla el SDK |
| Variables de entorno | Secrets Manager / Secret Manager / Key Vault inyectados como env | mismos nombres |
| Red privada `railway.internal` | VPC + service discovery | `API_INTERNAL_URL` cambia, nada más |
| Pre-deploy command | job de migración en el pipeline antes del rollout | |
| Proxy de Railway | ALB / Cloud Load Balancing + WAF | ajustar keep-alive y timeouts (ver `api/conventions.md`) |

Disparadores para migrar (cualquiera): necesidad de réplicas de lectura o PITR más fino que lo que ofrece Railway, requisitos de cumplimiento (residencia de datos, auditorías), costo por encima de lo equivalente gestionado, o necesidad de más de una región.
