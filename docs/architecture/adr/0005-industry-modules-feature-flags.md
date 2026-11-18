# ADR 0005: Módulos de rubro enchufables, planes y feature flags

- Estado: propuesto
- Fecha: 2026-11-17

## Contexto

El frontend ya varía por rubro: `getNavigation(industry)` en `apps/web/src/config/navigation.ts` agrega Agenda (odontología), Mascotas/Hospitalización/Grooming (veterinaria) y Salón/Cocina/Delivery/Reservas (restaurante); `Industry` en `types/domain.ts` tiene cuatro valores. El onboarding ofrece módulos por rubro (`modulesByIndustry`) y la consola admin gestiona flags por plan y rollout (`platform-admin/mocks/ops.ts`, `FeatureFlag` con `plans` y `rollout`). Los primeros rubros objetivo son **lubricentro** y **librería**, que hoy no existen en el código.

## Opciones

1. **Un `if industry == ...` repartido por el código**: lo más simple hoy, inmantenible con seis rubros.
2. **Un servicio o despliegue por rubro**: aislamiento que nadie pidió; duplica el core.
3. **Core + módulos registrados en el arranque, habilitados por tenant** según plan, overrides y flags.

## Decisión

Opción 3.

- **Módulo** = unidad de producto que se enciende o apaga por tenant: core (`ventas`, `inventario`, `compras`...), rubro (`rubro.lubricentro`, `rubro.libreria`, `rubro.restaurante`, `rubro.veterinaria`, `rubro.odontologia`, `rubro.retail`) o addon (`agenda`, `integraciones.whatsapp`...). Catálogo en `platform.modules`.
- Cada módulo Go implementa `Module { Code(); Routes(mux); Jobs(registry); Permissions(); NavHints() }` y se registra en `cmd/wanka`. Todos se despliegan siempre; habilitar es un dato, no un despliegue.
- **Entitlements de un tenant** = módulos del plan (`plan_modules`) ± overrides del tenant (`tenant_module_overrides`, desde la consola) + feature flags (`feature_flags` con `rollout_pct` determinista por `hash(tenant_id, flag)` y `tenant_flag_overrides`) + límites del plan (`plans.limits` vs. `platform.tenant_usage`).
- El rubro del tenant (`org.tenants.industry`) decide **qué módulos de rubro se sugieren y habilitan al aprovisionar**, no qué código corre: un lubricentro que también vende repuestos puede tener `rubro.retail` si su plan lo permite.
- **Aplicación en cada petición** ([`sequences/08-entitlement-enforcement.mmd`](../sequences/08-entitlement-enforcement.mmd)): cada ruta declara su módulo (`x-module` en OpenAPI); middleware: empresa no cerrada (`403 tenant_read_only` si no es `GET`) → suscripción activa (`402 subscription_inactive`) → módulo habilitado (`403 module_not_enabled`) → límite (`403 plan_limit_reached`) → permiso RBAC. Caché en memoria por tenant con TTL 30 s, invalidada por `LISTEN/NOTIFY entitlements_changed`. Los jobs comprueban lo mismo antes de ejecutar.
- La navegación del frontend pasa de `getNavigation(industry)` a `getNavigation(entitlements)` (`GET /v1/me/entitlements`); el ocultamiento en la UI es cosmético, la API es la que aplica.
- Extensión de datos: cada rubro tiene su esquema (`lube`, `books`, `resto`, `vet`, `dental`) y referencia entidades core por FK compuesta; el core no conoce a los rubros. Atributos simples (viscosidad, talla) en `catalog.item_variants.attributes` validado por el JSON Schema que registra el rubro. Ver `data/data-model.md` §7.
- Industry pasa de 4 a 6 valores: `lubricentro | libreria | restaurante | veterinaria | odontologia | retail` (cambio compatible en `types/domain.ts` cuando se integre).

Desactivar un módulo **no borra datos**: sus endpoints responden 403 y sus jobs no corren; reactivarlo recupera todo.

**Empresa cerrada** (decisión del usuario: datos conservados 5 años en solo lectura): `org.tenants.status = cerrada`. Los entitlements pasan a modo lectura: el middleware rechaza todo método distinto de `GET` con `403 tenant_read_only` (los exportes son `GET /v1/exports` y siguen disponibles), y no corre ningún job del tenant. A los 5 años de `closed_at`, un job de plataforma purga los datos de forma auditada.

**Suscripción inactiva** (decisión del dueño, 2026-11-18: bloqueo total, salvo la exportación de datos). Con `platform.subscriptions.status` en `morosa`, `suspendida` o `cancelada`, el middleware responde `402 subscription_inactive` a toda ruta del plano tenant, también a los `GET`; no hay solo lectura durante la mora. Pasan solo `GET /v1/exports` (con su permiso `*:export`) y las rutas que no dependen del tenant o que muestran el motivo: `/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships` y `GET /v1/me/entitlements`. Es una lista fija en el código del middleware, no configuración. A una empresa cerrada no se le evalúa la suscripción: su único bloqueo es `tenant_read_only`, que queda solo para ese caso. Los jobs del tenant no inician operaciones nuevas (recordatorios, sincronización SIRE, envíos al cliente), pero sí terminan las que ya empezaron antes del bloqueo: el envío a Khipu de un CPE ya emitido y la recepción de sus webhooks siguen, porque el comprobante ya se entregó al cliente y tiene que llegar a SUNAT. Detalle del contrato: [`api/conventions.md` §4](../api/conventions.md#suscripción-inactiva-bloqueo-total-salvo-exportar-decisión-del-dueño-2026-11-18).

## Consecuencias

- Buenas: rubros nuevos sin tocar el core; planes y flags operables desde la consola sin desplegar; pruebas por módulo.
- Malas: todos los módulos viven en el binario (tamaño y superficie); hay que mantener coherentes `x-module` y `x-permission` en todas las rutas (prueba automática).
- Qué haría cambiar la decisión: un rubro con requisitos regulatorios propios (p. ej. historia clínica con exigencias de residencia o cifrado distintas) podría justificar aislarlo.
