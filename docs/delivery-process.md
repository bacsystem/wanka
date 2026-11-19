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
- Después del merge del release, el dueño crea un tag por lote: `git tag -a vX.Y.Z -m "vX.Y.Z" <merge commit del release>`, sobre un commit de `main` con la CI en verde.
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

## Tags automáticos

`.github/workflows/release-tag.yml` crea el tag `vX.Y.Z` cuando el dueño mergea en `main` un PR de release (`chore(release): vX.Y.Z`), y solo después de que los workflows `CI` y `API` pasen sobre ese commit. Lee la versión del `package.json` del commit. Si el tag ya existe no hace nada, y en cualquier otro commit de `main` tampoco. El merge del release es la aprobación del dueño. Ninguna persona ni agente empuja tags a mano.

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
