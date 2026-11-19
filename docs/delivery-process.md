# Proceso de entrega: lotes, versión y definición de terminado

Decisión del dueño, 2026-11-19. Reemplaza la regla anterior de que cada PR subía la versión. Esa regla obligaba a encadenar los PR (cada uno partía del anterior para no chocar en `package.json` y `CHANGELOG.md`), y un arreglo en un PR de abajo había que mergearlo y volver a verificarlo en todos los de arriba.

## Lotes

- Cada historia sale en su propio PR **desde `main`**, en paralelo y sin depender de los demás PR del lote. Un PR se apila sobre otro (`gh stack`) solo si su código depende de él.
- Un PR **no toca la versión**: agrega su entrada en `CHANGELOG.md`, bajo `## [Unreleased]`, en la sección que corresponda (`Added`, `Changed`, `Fixed`…), en inglés y con el número de la issue.
- El dueño mergea el lote cuando todos sus PR cumplen la definición de terminado. Mergea en el orden que le pasa el coordinador, que pone primero los PR que otros necesitan.
- Al cerrar el lote, un **PR de release** (`chore(release): vX.Y.Z`) sube la versión una sola vez:
  - `version` en `package.json` (raíz) y `apps/web/package.json`;
  - `## [Unreleased]` pasa a `## [X.Y.Z] - YYYY-MM-DD`.

  La versión sigue SemVer, aun en 0.x: **minor** si el lote tiene algún `feat` o un cambio incompatible; si no, **patch**.
- Después del merge del release, `release-tag.yml` crea el tag del lote sobre ese commit de `main`, en cuanto la CI pasa en verde (ver «Tags automáticos»).
- Commits y PR van con `/git-flow`. En este proceso, `/git-flow` no calcula ni aplica la versión en los PR de historia; solo lo hace en el PR de release.

## Definición de terminado (antes de pedir el merge)

1. `/pr-review` aprobado y publicado como comentario del PR, sin hallazgos abiertos (las re-revisiones también se publican).
2. QA sobre el **SHA final** del PR, con su informe publicado. Si hubo commits después de QA, QA repite al menos lo que esos commits tocaron.
3. CI en verde sobre ese SHA.
4. Checkboxes de la issue tildados, con la evidencia de cada criterio (test o comando y su resultado) en un comentario de la issue.

## Revisión y QA

- **PR de riesgo:** auth y sesiones, RLS, roles y grants, dinero e impuestos, idempotencia, migraciones de datos sensibles, o cualquier cosa que cruce un límite de seguridad. Llevan revisión y QA **separadas**, cada una hecha por un agente distinto y en su propio worktree.
- **Demás PR:** docs, tests, CI, o código sin esas superficies. Un solo agente hace la revisión (`/pr-review`) y QA (los criterios de la issue y los casos extra) en una pasada.
- Una corrección chica después de una revisión (solo tests, docs o una o dos líneas) la puede re-revisar el coordinador sin lanzar otro agente, siempre que publique la re-revisión en el PR.
- La verificación local completa (suite con PostgreSQL y `rls-check`) se corre una vez por PR. Cuando se integra `main` o una rama base en una rama ya verificada, alcanza con compilar y dejar que la CI lo verifique.

## CI por PR y por release (decisión del dueño, 2026-11-19)

- **Cada PR** corre la parte liviana: `gofmt`, `vet`, migraciones solo hacia adelante, `migrate up`, `go test -race` con PostgreSQL, el paso de cotas de tiempo sin `-race`, `build` y `arch-lint` (unos 3 min, en GitHub y en paralelo). Además corre `checks` web.
- **`rls-check`**, con sus casos de referencia y todas sus mutaciones, corre solo en los PR que tocan `apps/api/migrations/`, `apps/api/bootstrap/`, `apps/api/scripts/rls-check.sh`, `docs/architecture/data/rls-check/` o `api.yml`, y siempre en el PR de release (rama `chore/release…`) y en cada push a `main`.
- **El e2e (Playwright)** corre en los PR que tocan `apps/web` o sus dependencias. El PR de release siempre lo corre, porque sube la versión de `apps/web/package.json`.
- Así cada historia llega a la revisión con su CI en verde, los errores aparecen en el PR que los causa y la validación completa queda en el release.

## Retrospectiva por lote (decisión del dueño, 2026-11-19)

Al cerrar cada lote, junto con el PR de release, quien coordina (con Claude o con Gemini) corre el skill `retro` de `bacsystem/skills`, que produce tres cosas:

1. `docs/retros/AAAA-MM-DD-<lote>.md`, con las métricas del lote tomadas de GitHub (rondas de revisión, hallazgos por severidad, horas hasta el merge y minutos y fallos de CI), los cuellos de botella con su evidencia, las clases de defecto y las mejoras.
2. Los tipos de defecto nuevos se agregan a [`architecture/review-checklist.md`](architecture/review-checklist.md). Si cambia una regla del equipo, se edita `CLAUDE.md` (nunca `GEMINI.md`, que solo lo importa).
3. Un PR a `bacsystem/skills` con las mejoras genéricas para agentes y skills.

Cada retro compara contra la lista «a vigilar» de la anterior. El dueño aprueba las mejoras mergeando esos PR.

## Tags automáticos

`.github/workflows/release-tag.yml` crea el tag `vX.Y.Z` cuando el dueño mergea en `main` un PR de release (`chore(release): vX.Y.Z`), y solo después de que los workflows `CI` y `API` pasen sobre ese commit. Lee la versión del `package.json` del commit. Si el tag ya existe no hace nada, y en cualquier otro commit de `main` tampoco. El merge del release es la aprobación del dueño. Ninguna persona ni agente empuja tags a mano. Si la CI de ese commit falla y el arreglo llega en otro commit, el dueño lanza el workflow a mano (`workflow_dispatch`) con el SHA del **primer** commit en verde después del release. Si elige uno posterior, el tag arrastraría historias que siguen en `[Unreleased]`. El workflow comprueba que la versión de ese commit sea la del release y que su CI esté en verde. Si el tag ya existe sobre otro commit, falla y no lo mueve.

## Rendimiento (lecciones del lote 2 del kernel, 2026-11-19)

- **La suite completa se corre una vez por PR.** Mientras trabajan, el desarrollador, la revisión y QA corren solo los paquetes afectados y sus mutaciones. Después de integrar `main` o una base ya verificada, basta con compilar: la verificación la hace la CI. `rls-check` se corre solo si cambian SQL, grants, políticas o funciones definer.
- **Revisión previa del architect.** Antes de implementar una historia, el architect la recorre con su lista de 8 puntos: dato ausente, «ninguno», tiempo y zona, principal, grants y particiones, representación, estados degradados y orden de la cadena. Deja las decisiones en la issue.
- **Errores conocidos.** El desarrollador lee [`architecture/review-checklist.md`](architecture/review-checklist.md) antes de escribir. Cada revisión con hallazgos suma ahí el tipo de error.
- **Como mucho 4 agentes con base de datos a la vez.**
- **Informes de 15 líneas como máximo.** El detalle va a un archivo del scratchpad.
- **Nombres únicos** en el scratchpad compartido (prefijo de la historia o del PR) y en los contenedores. Nadie borra lo que no creó.

## Entorno local de agentes

Para los tests de integración hay un PostgreSQL 16 compartido de larga vida: el contenedor `wanka-shared-pg` en `127.0.0.1:55400`, con el bootstrap de roles aplicado. Se usa con:

```sh
export WANKA_TEST_ADMIN_URL=postgres://postgres:<clave local del superusuario>@127.0.0.1:55400/wanka
export WANKA_TEST_ROLE_PASSWORD=<clave local de los roles>
```

Las claves son solo locales y no se guardan en el repo: el coordinador las pasa en el archivo `shared-pg.env` del scratchpad. `dbtest` crea una base por test, y los tests que tocan roles (`NewExclusive`) se serializan con un lock de clúster, así que varios agentes pueden usarlo a la vez. `rls-check/run.sh` usa su propio contenedor, con un nombre único por corrida.
