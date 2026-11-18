# ADR 0002: Monolito modular hexagonal en Go y librerías base

- Estado: aceptado (2026-11-17: el dueño aprobó River, sqlc, goose, oapi-codegen, shopspring/decimal, pquerna/otp, OpenTelemetry y el SDK de S3, y también pgx/v5, golang.org/x/crypto y go-arch-lint; Go y PostgreSQL ya eran decisión del usuario)
- Fecha: 2026-11-17

## Contexto

Hay ~20 bounded contexts (taxonomía del PO: identidad, empresas, clientes, catalogo, inventario, ventas, facturacion-electronica, compras, finanzas, reportes, agenda, integraciones, auditoria, notificaciones, plataforma y seis rubros). El flujo central, la venta POS, debe ser atómico a través de ventas, inventario, empresas (numeración), finanzas (caja) y facturación (CPE pendiente + job). Equipo pequeño, una sola base (ADR 0001), despliegue inicial en Railway (ADR 0007).

## Opciones

1. **Microservicios por módulo**: despliegue independiente, pero la venta necesitaría sagas entre 4-5 servicios, y el equipo pagaría red, versionado de contratos internos y observabilidad distribuida sin una necesidad medida.
2. **Monolito sin fronteras** (paquetes por capa técnica): rápido al inicio; con 20 módulos termina en acoplamiento por tablas.
3. **Monolito modular hexagonal**: un binario, módulos con fronteras verificadas, transacción local cuando hace falta, extracción posible de un módulo si una medición lo pide.
4. **Framework [go-bricks](https://github.com/gaborage/go-bricks)** (evaluado el 2026-11-17, descartado): su multi-tenancy toma el tenant de una cabecera, un subdominio o el path, y aísla con una base y un pool por tenant, sin `set_config` ni RLS; choca con ADR 0001 (tenant desde la sesión, base única con `FORCE ROW LEVEL SECURITY`). Sus migraciones usan Flyway (CLI de Java) y su outbox entrega a RabbitMQ, frente a goose y River sobre Postgres (ADR 0006); sus respuestas van envueltas en `{data, meta}`, frente a RFC 9457 (`api/conventions.md`). Está en v0.x con cambios incompatibles en versiones menores y casi un solo mantenedor, e impone su estructura de módulos: habría que desactivar su núcleo multi-tenant y aun así sumaría días al kernel.

## Decisión

Opción 3.

**Estructura** (`apps/api`, módulo Go `github.com/<org>/wanka/apps/api`):
```
cmd/wanka/                 # main: serve --plane=tenant|platform|webhooks, worker, migrate
internal/platform/         # kernel: db (InTenantTx), httpx, auth middleware, jobs (River), money, blob, entitlements, numbering, clock, idgen
internal/<modulo>/         # identidad, empresas, clientes, catalogo, inventario, ventas, facturacion, compras, finanzas, reportes, agenda, integraciones, auditoria, notificaciones
  domain/                  # entidades, value objects, reglas; sin imports de infraestructura
  app/                     # casos de uso; definen puertos (interfaces) que necesitan
  adapters/postgres/       # sqlc; solo tablas del esquema del módulo
  adapters/http/           # handlers que implementan la interfaz generada de OpenAPI
  module.go                # Facade pública + registro (rutas, jobs, permisos, migraciones)
internal/rubros/<rubro>/   # lubricentro, libreria, restaurante, veterinaria, odontologia, retail (misma forma)
internal/plataforma/       # solo se monta con --plane=platform
migrations/                # goose, SQL
openapi/                   # contrato (ADR 0009)
```
- Un módulo importa de otro **solo** su `Facade` (paquete raíz del módulo). Prohibido importar `domain`, `app` o `adapters` ajenos y leer tablas ajenas. Verificado en CI con `go-arch-lint`.
- Transacción compartida: la fachada recibe el `pgx.Tx` (o un `ctx` que lo transporta) del caso de uso que orquesta, p. ej. `ventas.CreateSale` llama a `inventario.Decrement(ctx, tx, ...)`, `empresas.NextNumber(ctx, tx, ...)`, `facturacion.CreatePending(ctx, tx, ...)`. Integraciones asíncronas entre módulos van por jobs (ADR 0006).
- `--plane=platform` monta solo `plataforma` + lecturas permitidas; `--plane=tenant` nunca monta `plataforma`.

**Librerías** (todas maduras y con licencia permisiva). Aprobadas por el dueño el 2026-11-17: `oapi-codegen`, `sqlc`, `goose`, `River`, `shopspring/decimal`, `pquerna/otp`, OpenTelemetry Go SDK, `aws-sdk-go-v2/service/s3`, `jackc/pgx/v5`, `golang.org/x/crypto` (argon2) y `go-arch-lint`. Cualquier dependencia que no esté en esa lista necesita la aprobación del dueño antes de agregarse:
| Necesidad | Elección | Alternativa descartada |
|---|---|---|
| HTTP | `net/http` (patrones de ruta de Go 1.22+) + `oapi-codegen` strict server | chi, echo, gin: no hacen falta con el mux estándar |
| Postgres | `jackc/pgx/v5` + `pgxpool` | `database/sql`+lib/pq (sin mantenimiento activo) |
| Consultas | `sqlc` (SQL a mano, código tipado generado) | GORM/ent: ocultan el SQL que necesitamos controlar (locks, RLS) |
| Migraciones | `pressly/goose/v3` | golang-migrate, Atlas (ver `data/data-model.md`) |
| Jobs/outbox | `riverqueue/river` (ADR 0006) | outbox propio |
| Decimal | `shopspring/decimal` | `cockroachdb/apd` (más estricto, API menos cómoda); `float64` prohibido |
| Hash de contraseñas | `golang.org/x/crypto/argon2` | bcrypt (límite de 72 bytes, menos resistente a GPU) |
| TOTP | `pquerna/otp` | implementación propia (RFC 6238 es corto, pero no vale el riesgo) |
| Logs | `log/slog` (estándar) JSON | zap/zerolog |
| Trazas/métricas | OpenTelemetry Go SDK | — |
| S3 | `aws-sdk-go-v2/service/s3` (sirve para cualquier S3 compatible) | minio-go |
| Validación | generada desde OpenAPI + reglas de dominio en Go | validator con tags |

Versiones: Go estable vigente (1.25 o superior al iniciar) y **PostgreSQL 16**, la versión con que se verifica `data/rls-check/`. Pasar a otra versión mayor exige volver a correr esa verificación (exit 0) y registrar el cambio en un ADR nuevo.

## Consecuencias

- Buenas: la venta POS es una transacción local (sin sagas); un solo artefacto que probar y desplegar; las fronteras son explícitas y verificadas, así que extraer un módulo después (p. ej. `facturacion` a su propio servicio si su carga lo exige) es mover un paquete y cambiar la fachada por un cliente.
- Malas: un despliegue lleva todos los módulos; un módulo con una fuga de memoria afecta a todos (mitigado: `worker` separado de `api`); la disciplina de fronteras depende del linter en CI.
- Qué haría cambiar la decisión: un módulo con requisitos de escala o de seguridad muy distintos (medidos), o equipos separados con ritmos de despliegue incompatibles.
