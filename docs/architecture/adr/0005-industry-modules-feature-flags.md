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
- **Aplicación en cada petición** ([`sequences/08-entitlement-enforcement.mmd`](../sequences/08-entitlement-enforcement.mmd)): cada ruta declara su módulo (`x-module` en OpenAPI; `none` solo en las rutas que fija el [ADR 0009](0009-api-contract-openapi.md), «Cómo se declara ninguno»); middleware: empresa no cerrada (`403 tenant_read_only` si no es `GET` ni `HEAD`) → suscripción activa (`402 subscription_inactive`) → módulo habilitado (`403 module_not_enabled`) → límite (`403 plan_limit_reached`; todo plan declara todas las claves de `plans.limits` y una clave ausente cuenta como tope 0, nunca como «sin tope»: architect, 2026-11-19, #171. Un límite solo se aplica si tiene medición de uso: hoy `users` y `cpe_per_month`, en `platform.tenant_usage`. `sites` está en el plan pero **no se mide**, y registrar una ruta que lo consuma hace fallar el arranque hasta que la historia de empresas que crea `org.sites` agregue su medición: el número de sedes activas, contado en la misma lectura de los entitlements, no un contador mensual, porque las sedes son un stock y no un consumo del mes; architect, 2026-11-19, #175) → permiso RBAC. Caché en memoria por tenant con TTL 30 s, invalidada por `LISTEN/NOTIFY entitlements_changed`. Los jobs comprueban lo mismo antes de ejecutar.
- La navegación del frontend pasa de `getNavigation(industry)` a `getNavigation(entitlements)` (`GET /v1/me/entitlements`); el ocultamiento en la UI es cosmético, la API es la que aplica.
- Extensión de datos: cada rubro tiene su esquema (`lube`, `books`, `resto`, `vet`, `dental`) y referencia entidades core por FK compuesta; el core no conoce a los rubros. Atributos simples (viscosidad, talla) en `catalog.item_variants.attributes` validado por el JSON Schema que registra el rubro. Ver `data/data-model.md` §7.
- Industry pasa de 4 a 6 valores: `lubricentro | libreria | restaurante | veterinaria | odontologia | retail` (cambio compatible en `types/domain.ts` cuando se integre).

Desactivar un módulo **no borra datos**: sus endpoints responden 403 y sus jobs no corren; reactivarlo recupera todo.

**Empresa cerrada** (decisión del usuario: datos conservados 5 años en solo lectura): `org.tenants.status = cerrada`. Los entitlements pasan a modo lectura: el middleware rechaza todo método distinto de `GET` y `HEAD` con `403 tenant_read_only` (los exportes son `GET /v1/exports` y siguen disponibles), y no corre ningún job del tenant. **Qué módulos lee** (architect, 2026-11-19, #175): los de su último plan con los overrides encima, **aunque la suscripción esté morosa, suspendida o cancelada**, porque su lectura no depende del pago (`api/conventions.md` §4). La suscripción que cuenta es la misma que para una empresa abierta: la viva o, si solo hay canceladas, la más reciente. Así la empresa cerrada lee lo mismo que leía el día que cerró, y `GET /v1/me/entitlements` le muestra los mismos menús. Una empresa cerrada **sin ninguna suscripción** solo tiene los módulos que un override le encienda: nunca pudo escribir en un módulo, así que no tiene nada que leer en ellos. El piso de la conservación son los exportes (`GET /v1/exports`, `x-module: none`), que no dependen de los módulos: un módulo que el plan perdió antes del cierre no se lee por sus `GET`, pero sus datos se exportan, y plataforma puede encender su lectura con un override. Opciones descartadas: saltar la regla de módulo en todo `GET` de una empresa cerrada (la API dejaría leer lo que `GET /v1/me/entitlements` dice que no está, y la UI ocultaría menús que sí responden) y abrirle el catálogo entero (menús de rubros que nunca tuvo). Qué la haría cambiar: una obligación legal de servir por `GET` datos de un módulo perdido antes del cierre, que el exporte no cubra. A los 5 años de `closed_at`, un job de plataforma purga los datos de forma auditada.

**Suscripción inactiva** (decisión del dueño, 2026-11-18: bloqueo total, salvo la exportación de datos). Con `platform.subscriptions.status` en `morosa`, `suspendida` o `cancelada`, el middleware responde `402 subscription_inactive` a toda ruta del plano tenant, también a los `GET`; no hay solo lectura durante la mora. Pasan solo `GET /v1/exports` (con su permiso `*:export`) y las rutas que no dependen del tenant o que muestran el motivo: `/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships` y `GET /v1/me/entitlements`. Es una lista fija en el código del middleware, no configuración. A una empresa cerrada no se le evalúa la suscripción: su único bloqueo es `tenant_read_only`, que queda solo para ese caso. Los jobs del tenant no inician operaciones nuevas (recordatorios, sincronización SIRE, envíos al cliente), pero sí terminan las que ya empezaron antes del bloqueo: el envío a Khipu de un CPE ya emitido y la recepción de sus webhooks siguen, porque el comprobante ya se entregó al cliente y tiene que llegar a SUNAT. Detalle del contrato: [`api/conventions.md` §4](../api/conventions.md#suscripción-inactiva-bloqueo-total-salvo-exportar-decisión-del-dueño-2026-11-18).

## Consecuencias

- Buenas: rubros nuevos sin tocar el core; planes y flags operables desde la consola sin desplegar; pruebas por módulo.
- Malas: todos los módulos viven en el binario (tamaño y superficie); hay que mantener coherentes `x-module` y `x-permission` en todas las rutas (prueba automática).
- Qué haría cambiar la decisión: un rubro con requisitos regulatorios propios (p. ej. historia clínica con exigencias de residencia o cifrado distintas) podría justificar aislarlo.

## Nota del 2026-11-20 (architect, decisión del dueño en #4: prueba vencida en solo lectura)

La empresa que se registra sola (#31) arranca con una suscripción `trial` del plan `prueba`: los módulos `core` más el de su rubro por override, por 30 días: `period_end = period_start + 29`, que es el último día con escritura, inclusive ([data-model §6](../data/data-model.md); la crea `platform_fn.start_self_service_trial`, ADR 0001, nota del 2026-11-20).

- **Al vencer** (`status = 'trial'` y `period_end` anterior a hoy en Lima, es decir, desde el día 31), la empresa queda **en solo lectura**, igual que una empresa cerrada. El paso de entitlements marca `readOnly`, rechaza todo método distinto de `GET` y `HEAD` con `403 tenant_read_only` (`api/conventions.md` §4) y sigue leyendo los módulos de su plan y sus overrides.
- **No es una suscripción inactiva:** no responde `402` y deja ver los datos. Lo que se pierde es solo escribir.
- **Los jobs** del tenant con la prueba vencida siguen la regla de la empresa cerrada. Una historia de cobros del SaaS decidirá qué pasa después: pagar, o pasar a `morosa` o `cancelada`.
- **Quién lo aplica:** #31, que crea las pruebas. Es un cambio chico en `internal/platform/entitlements` (leer `period_end` de la suscripción viva) y en el paso de empresa cerrada de `auth`, con su test: el último día (`period_end` = hoy), que escribe; una prueba creada hace 30 días (`period_end` = ayer), que da `403` en un `POST` y `200` en un `GET`; y vencida con override.
