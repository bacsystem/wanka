# ADR 0009: Contrato de API spec-first con OpenAPI 3.1

- Estado: propuesto
- Fecha: 2026-11-17

## Contexto

El frontend existe y consume mocks cuya forma "refleja la que devolverá la API" (CLAUDE.md). Frontend y backend van a trabajar en paralelo; hace falta un árbitro único del contrato. Las convenciones están en [`api/conventions.md`](../api/conventions.md).

## Opciones

1. **Code-first** (p. ej. huma: el esquema se genera desde los tipos Go): una sola fuente, pero el frontend espera a que exista el código para tener tipos.
2. **Spec-first**: `apps/api/openapi/openapi.yaml` es la fuente; `oapi-codegen` genera la interfaz del servidor Go (strict server) y `openapi-typescript` los tipos del cliente del BFF.
3. Sin esquema formal (documentación a mano): diverge en semanas.

## Decisión

Opción 2.

- El borrador de cada endpoint se publica en su issue (endpoint, método, rol/permiso, request/response con tipos y opcionalidad, errores, paginación, idempotencia, límites); al implementarse se incorpora a `openapi.yaml`, que pasa a ser el árbitro.
- Extensiones obligatorias por operación: `x-module` (ADR 0005) y `x-permission` (ADR 0004); una prueba recorre el router y falla si alguna ruta no las tiene o si existe una ruta no declarada.
- **Cómo se declara «ninguno»** (architect, 2026-11-19, #171). Las dos extensiones son siempre un string. El valor `none` (literal, en minúsculas) dice que la ruta no exige módulo o permiso; cualquier otro valor es un código del catálogo: `x-module`, de `platform.modules` (`none` está reservado y no puede ser código de módulo); `x-permission`, de [`api/resources.md`](../api/resources.md) (formato `modulo.recurso:accion`, que nunca coincide con `none`). La prueba del router (#166) falla si la extensión falta, es `null`, está vacía (`x-module:` sin valor es `null` en YAML), no es string o no está en el catálogo: nada de eso se lee como `none`, para que un olvido cierre en vez de abrir.
  - `x-module: none` vale en las rutas `Public` y `NoTenant` ([`api/conventions.md` §2](../api/conventions.md); no pasan por entitlements), en todo el plano plataforma (`/platform/v1`) y en las rutas `Tenant` de la lista fija que pasa con la suscripción inactiva (`GET /v1/me/entitlements` y `GET /v1/exports`, ADR 0005, «Suscripción inactiva»): con la suscripción inactiva no hay ningún módulo habilitado, y esas rutas con un módulo darían `403 module_not_enabled`. `none` solo salta la comprobación de módulo y de límite; no salta `tenant_read_only` ni `402 subscription_inactive`, que siguen la lista fija del middleware. Cualquier otra ruta `Tenant` con `none` hace fallar la prueba.
  - `x-permission: none` vale en las rutas `Public` y `NoTenant`, y en las `Tenant` que `resources.md` marca «sin permiso de catálogo» (las protege la sesión del propio usuario o la membresía), como `GET /v1/me/entitlements`.
  - En Go, la ruta se registra con las constantes `NoModule` y `NoPermission`, que valen `none`; un string vacío al registrar hace fallar el arranque. La prueba compara, ruta por ruta, lo que declara el router con el spec.
- CI: lint del spec (Redocly), generación de código sin diff, y detección de cambios incompatibles contra `main` (`oasdiff breaking`) que bloquea el merge salvo nueva versión.
- Los tipos generados reemplazan gradualmente a `types/domain.ts` y a los tipos de `features/*/mocks`; los mocks pueden seguir existiendo como fixtures validados contra el esquema.
- **Clientes del frontend: funciones a mano, tipos generados.** El portal de Khipu (`bacsystem/khipu`, `portal/src/lib/api/*.ts`) usa tipos escritos a mano y solo usa los generados como guarda de contrato (`contrato-openapi.ts`). Lo hace porque su spec sale de springdoc, que marca todos los campos como opcionales, y los tipos generados "dirían menos". En Wanka el spec es la fuente (spec-first) y declara `required` y `nullable` explícitamente, así que los tipos generados por `openapi-typescript` son exactos y se usan **directamente**. Se mantiene la forma del portal: una función a mano por operación en `lib/api/*.ts`, que fija URL, cabeceras, errores y reintentos, tipada con `components["schemas"]`. Mismo patrón, sin la duplicación de tipos. Si el spec perdiera precisión, se volvería a la guarda de tipos del portal.

## Consecuencias

- Buenas: frontend y backend avanzan en paralelo; los cambios incompatibles se detectan en CI.
- Malas: el YAML es verboso; dos generadores que mantener.
- Qué haría cambiar la decisión: si el mantenimiento del YAML frena al equipo, migrar a code-first conservando el mismo esquema publicado.
