# Retro: kernel, lote 3 (release v0.19.0)

Segunda retrospectiva por lote, hecha con el skill `retro` de `bacsystem/skills`, y la primera con el proceso nuevo: PR independientes desde `main`, sin bump, más un PR de release (#222). Compara contra la lista «a vigilar» de [la retro de los lotes 1 y 2](2026-11-19-kernel-lotes-1-2.md).

- **PR de historia:** #219 (#166, contrato OpenAPI por módulo y código generado) y #218 (#176, RBAC y alcance de la impersonada).
- **PR de proceso:** #214 (entrega por lotes, tag automático, CI liviana) y #217 (retro anterior).
- Con este lote queda cerrado el kernel de la ola 0 (épica #154).

## Métricas

Fuente: `retro/scripts/collect.sh` (v0.3.13).

| PR | Historia | Rondas de revisión | Informes de QA | Bloq. | Imp. | Men. | Commits de trabajo | Merges de la base | Hasta el merge | CI fallida | Minutos de CI |
|---|---|---|---|---|---|---|---|---|---|---|---|
| #214 | proceso | 3 | 0 | 0 | 1 | 4 | 6 | 1 | 1,4 h | 0 | 26 |
| #217 | retro 1 y 2 | 2 | 0 | 0 | 1 | 4 | 1 | 0 | 14 min | 0 | 3 |
| #218 | #176 RBAC | 5 | 3 | 0 | 4 | 4 | 4 | 2 | 1,3 h | 0 | 20 |
| #219 | #166 contrato | 3 | 0 | 1 | 2 | 5 | 2 | 1 | 54 min | 0 | 14 |
| **Lote 3** | | **13** (3,25/PR) | **3** | **1** | **8** | **17** | **13** | **4** | | **0** | **63** |

#219 no figura con informes de QA porque su revisión y su QA fueron en un mismo informe (PR de riesgo bajo). Las horas hasta el merge cuentan desde que se abrió el PR, no desde que empezó la implementación.

## Contra la lista «a vigilar» anterior

| Métrica | Lotes 1 y 2 | Objetivo | Lote 3 | ¿Mejoró? |
|---|---|---|---|---|
| Rondas de revisión por PR | 2,8 y 2,6 | ≤ 2; ninguno con más de 3 | 3,25; #218 con 5 | No |
| Decisiones del architect después de implementar | 6 | 0 | 1 (#218: corrección de los ADR 0001 y 0004) | Sí |
| Merges de la base en PR de historia | 34 (43 % de los commits) | 0 en PR independientes | 4, todos por el CHANGELOG | Sí, mucho |
| Defectos introducidos por un merge | 4 | 0 | 0 | Sí |
| Corridas de CI fallidas por tests de tiempo | 4 de 4 | 0 | 0 | Sí |
| Criterios no funcionales sin número en el PR | 4 PR | 0 | 0 (p95 de 14 a 16 µs en #218) | Sí |
| Hallazgos importantes por PR | 1,8 y 1,4 | ≤ 1 | 2,0 (#218 con 4) | No |
| Contaminación entre worktrees reportada por QA | 3 PR | 0 | 0 | Sí |
| PR de skills sin `/pr-review` | 8 de 16 | 0 | 0 (#32 revisado) | Sí |

## Cuellos de botella

1. **Un criterio que dependía de otra historia se escribió al final, y deprisa (#218: 5 rondas, 4 importantes).** El criterio 10 de #176 (paridad entre el router y el contrato) necesitaba el código de #166. Se implementó después del merge de #219, en minutos, y se llevó dos rondas más:
   - H5: el test armaba el router con `wiring{}` y no con el `defaultWiring` de `serve`;
   - H6: no comparaba la clase de ruta que el architect había pedido;
   - H8: una lista duplicada entre `apigen.Class` y `auth`, que pasó a #221.

   De los 4 importantes de #218, 2 salieron de ese criterio. El otro par fue el hueco de seguridad H1 y el ADR reescrito en su lugar (H3).
2. **Un PR abierto en conflicto gastó una ronda entera (#219, H1 bloqueante).** #219 se abrió `DIRTY` por el CHANGELOG, así que la CI no corrió. La primera revisión tuvo que marcar como bloqueante que el cuerpo decía «corre en la CI del PR» cuando no había corrido nada. El coordinador abrió #218 y #219 con `gh pr create` a mano, saltándose el `git merge-tree` de `/git-flow`, contra la regla de `CLAUDE.md`. Los 4 merges de la base del lote salieron todos del mismo punto de inserción en `[Unreleased]`.
3. **Un test que calcula lo esperado con la misma fórmula del código fijó el defecto (#218, H1).** `TestImpersonatedPermissionsAreTheGrantScope` armaba el conjunto `read_only` con la misma regla que `impersonationSet`. Por eso, que la exclusión se aplicara solo con `full` quedó protegido por el test en vez de detectado. El architect había pedido una lista escrita a mano. Una revisión automática de seguridad lo marcó primero.

## Clases de defecto nuevas

- **Lo esperado se deriva con la fórmula del código** (#218): un test de conjuntos de permisos, alcances o catálogos compara contra una lista escrita a mano.
- **Un test de cableado no usa el cableado de producción** (#218): la paridad o el smoke del router montan exactamente lo que monta `serve`.
- **Un chequeo cubre una sola dimensión de las que pide el criterio** (#218): la paridad comparaba ruta y permiso, pero no la clase.
- **Se reescribe en su lugar la decisión de un ADR aceptado** (#218): una corrección lleva la nota fechada que exige el §9 del README de arquitectura, y la decide el architect.
- **Guarda con lista incompleta** (#219): el regex prometía cubrir `Handler` y no lo cubría.
- **Herramienta de CI sin fijar o recién publicada** (#219): Node sin `.nvmrc`, `npx` sin lockfile, versiones de 0 y 2 días.
- **Se confunde el exit code de una herramienta** (#219): un oasdiff que no pudo cargar el spec contaba como «breaking» y pasaba con la versión subida.
- **El cuerpo del PR afirma una CI que no corrió** (#219).

Todas quedaron en [`review-checklist.md`](../architecture/review-checklist.md).

## Mejoras

| Mejora | Dónde | Qué elimina |
|---|---|---|
| `CHANGELOG.md merge=union` en `.gitattributes`: dos PR que agregan líneas a `[Unreleased]` se integran sin conflicto. Probado con `git merge` local; falta ver si GitHub lo respeta al calcular `mergeable`, y eso se vigila en el próximo lote | este PR (`.gitattributes`, `delivery-process.md`) | Cuello 2: los 4 merges de la base del lote |
| Los PR se abren solo con `/git-flow`, también los que abre quien coordina; nunca con `gh pr create` a mano | ya está en `CLAUDE.md`; se refuerza en el `tech-lead` de `bacsystem/skills` | Cuello 2: el PR abierto en conflicto |
| Un criterio que depende de otra historia se implementa en el PR de esa historia, o se separa en una issue propia antes de empezar; no se agrega al final | `tech-lead` y `architect` en `bacsystem/skills` | Cuello 1 |
| Los tests de conjuntos comparan contra listas escritas a mano, y los de cableado usan el cableado de producción | `go-dev` y `qa` en `bacsystem/skills`, más el checklist | Cuello 3 y las clases de #218 |
| Herramientas de CI fijadas por lockfile o checksum, con al menos 14 días desde su publicación, y sus exit codes distinguidos | `devops` y `go-dev` en `bacsystem/skills`, más el checklist | Las clases de #219 |

## A vigilar en el próximo lote

| Métrica | Lote 3 | Objetivo |
|---|---|---|
| Rondas de revisión por PR | 3,25 | ≤ 2; ninguno con más de 3 |
| Hallazgos importantes por PR | 2,0 | ≤ 1 |
| Merges de la base en PR de historia | 4 | 0, con `merge=union` |
| PR abiertos en conflicto o fuera de `/git-flow` | 2 | 0 |
| Criterios agregados después de la primera revisión | 1 (#218, criterio 10) | 0 |
| Corridas de CI fallidas | 0 | 0 |
