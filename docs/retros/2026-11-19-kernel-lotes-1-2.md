# Retro: kernel, lotes 1 y 2 (2026-11-19)

Primera retrospectiva por lote, hecha con el skill `retro` de `bacsystem/skills`. No hay una retro anterior con la que comparar: la lista «a vigilar» del final es la base de la próxima.

- **Lote 1:** PR #201 a #205 (historias #196, #168, #25, #167 y #172).
- **Lote 2:** PR #206 a #213 (historias #171, #175, #170, #187, #164, #169, #173 y #177), más el PR de proceso #214.
- Los dos lotes se trabajaron como una **pila por versión**: cada PR partía del anterior para no chocar en `package.json` y `CHANGELOG.md`. Los PR de historia se mergearon entre las 13:38 y las 15:58 (UTC) del mismo día.

## Métricas

Fuente: `retro/scripts/collect.sh`, corregido a mano donde no cuadraba (ver la nota debajo de la tabla).

| PR | Historia | Rondas de revisión | Informes de QA | Bloq. | Imp. | Men. | Commits (de ellos, merges de la base) | Minutos de abierto a merge | CI fallida | Minutos de CI |
|---|---|---|---|---|---|---|---|---|---|---|
| #201 | #196 sesiones | 2 | 2 | 0 | 0 | 3 | 3 (0) | 66 | 0 | 28 |
| #202 | #168 Mailer | 3 | 3 | 0 | 2 | 6 | 6 (3) | 63 | 0 | 30 |
| #203 | #25 código por correo (docs) | 3 | 0 | 0 | 4 | 6 | 5 (2) | 63 | 0 | 24 |
| #204 | #167 precios e IGV | 3 | 3 | 1 | 2 | 4 | 5 (2) | 63 | 0 | 30 |
| #205 | #172 catálogo de permisos | 3 | 2 | 0 | 1 | 3 | 5 (2) | 60 | 1 | 33 |
| **Lote 1** | | **14** (2,8/PR) | **10** | **1** | **9** | **22** | **24 (9)** | | **1** | **145** |
| #206 | #171 entitlements | 3 | 1 | 0 | 2 | 5 | 6 (1) | 113 | 0 | 35 |
| #207 | #175 middleware de entitlements | 2 | 1 | 0 | 1 | 2 | 7 (4) | 128 | 0 | 33 |
| #208 | #170 tipo de cambio | 3 | 1 | 0 | 1 | 3 | 6 (3) | 125 | 1 | 40 |
| #209 | #187 ex miembros sin RLS | 2 | 1 | 0 | 1 | 1 | 5 (3) | 122 | 0 | 43 |
| #210 | #164 catálogo RLS | 3 | 1 | 0 | 3 | 1 | 8 (3) | 120 | 0 | 62 |
| #211 | #169 jobs en River | 3 | 1 | 0 | 1 | 5 | 8 (3) | 111 | 1 | 52 |
| #212 | #173 auditoría | 3 | 1 | 0 | 2 | 6 | 7 (2) | 93 | 1 | 29 |
| #213 | #177 idempotencia | 2 | 1 | 0 | 0 | 4 | 8 (4) | 89 | 0 | 26 |
| **Lote 2** | | **21** (2,6/PR) | **8** | **0** | **11** | **27** | **55 (23)** | | **3** | **320** |
| #214 | proceso por lotes | 3 | 0 | 0 | 1 | 4 | 8 (1, de `main`) | 81 | 0 | 26 |

Notas sobre los números:

- **Rondas.** `collect.sh` daba entre 4 y 12 rondas por PR porque cuenta cada línea que dice «Veredicto», también dentro de las tablas. La tabla cuenta los comentarios publicados cuya primera línea es un veredicto, una re-revisión o un cierre de revisión. #210 publicó la segunda ronda y QA en un mismo comentario: cuenta en las dos columnas.
- **Hallazgos.** Son la suma de las cabeceras de cada veredicto (`N bloqueantes · N importantes · N menores`), y coinciden con lo que da `collect.sh`.
- **Minutos hasta el merge.** `collect.sh` los da en horas enteras (1 o 2), que no distinguen nada en un lote de un día. Miden sobre todo la espera del merge del lote, no el trabajo: el dueño mergea todo el lote junto.
- **Commits tras abrir.** `collect.sh` cuenta como trabajo los merges de la rama base. En la tabla van aparte: 32 de los 79 commits de los PR de historia (40 %) son merges de la base.

## Cuellos de botella

### 1. El apilado por versión: re-mergear y re-verificar toda la pila

- **32 merges de la base** en 13 PR (9 en el lote 1 y 23 en el lote 2): el 40 % de los commits.
- En el lote 2 hubo **tres cascadas**, en las que cada arreglo de abajo se propagó por toda la pila: a las 14:08 (#208, #209, #210, #211, en el mismo minuto), a las 14:43 (#208, #209, #210) y a las 15:09 (#209, #210, #211). Cada cascada obliga a volver a compilar, a correr tests y a re-revisar el PR de arriba.
- **Defectos que causó el propio merge**, no la historia:
  - IDs de caso de `rls-check` repetidos tras el merge: #206 H6 y #208 H2.
  - Un `ROLLBACK` perdido en un merge de `05_app.sql`: #213 H2.
  - Un PR en conflicto con su base (`DIRTY`), así que la CI no corrió sobre su SHA: #212 H5, importante.
  - Un commit solo para que los tests de otra historia siguieran en verde «on the stack»: #211.
- Los PR del lote 2 tardaron entre 89 y 128 minutos en mergearse, contra 60 a 66 en el lote 1: la pila era más alta (8 PR contra 5).

### 2. La suite completa se corría hasta 4 o 5 veces por PR

- Comentarios de revisión y QA que reportan una corrida completa (`go test … ./...`): **3, 5, 4 y 4** en #201, #202, #204 y #205, sin contar la del desarrollador ni la de la CI.
- En el lote 2, tras la regla de una suite por PR, bajaron a **2 por PR** en #208, #210, #211, #212 y #213 (y a 1 en #209).
- `rls-check` se corrió completo hasta **3 veces** en un PR (#205 y #209), aunque solo cambiaba Go en los arreglos.
- Los minutos de CI subieron de 145 en el lote 1 a 320 en el lote 2. El motivo es que cada merge de la base dispara la CI otra vez, no que la suite crezca: #210 (62 min) y #211 (52 min) son los que más merges de la base recibieron.

### 3. Un contenedor de PostgreSQL por agente saturaba la máquina

- En las pruebas de carga de #201 había **entre 12 y 16 contenedores** corriendo a la vez y una carga de **5,7 a 10,4** en una máquina de 16 núcleos. El informe dice que el generador de carga competía por la CPU con los demás agentes, así que los números de carga quedaron contaminados.
- Contenedores propios citados en los informes del lote 2: `qa206-pg`, `pr207-review-pg`, `pr207-rereview-pg`, `qa207-pg`, `rev208-pg`, `qa208-pg`, `rev210-pg`, `rev211-pg`, `rev211-rls`, `rev212-pg`, `rev213-rls`, además de los `wanka-rls-check-*` de cada corrida.
- `wanka-rls-check` estaba ocupado por otra sesión en #203 y #205: hubo que copiar el árbol al scratchpad o esperar.
- Desde #211, la revisión y QA usan la base compartida (`shared-pg.env`) y no levantan contenedor propio.

### 4. Las decisiones de diseño aparecían en la revisión, después de implementar

- **6 decisiones del architect** tomadas con el código ya escrito, cada una con su commit de arreglo:
  - #206: un límite ausente cierra, `none` es un literal reservado y `tenant_usage` es legible por la app (commit de las 12:59, diez minutos después del primero).
  - #207: `HEAD` cuenta como lectura con la empresa cerrada (13:34, siete minutos después).
  - #212: las reglas de forma del actor como `CHECK` con nombre (13:54).
  - #213: la clave de idempotencia ligada al principal, con la PK `(tenant_id, principal_id, key)`, y el ADR 0012 aprobado por el dueño (14:04).
- #203, el contrato del código por correo, escrito sin revisión previa: **4 importantes y 8 menores**, el máximo del lote. Son hallazgos de diseño (techo de fallos, precedencia `401`/`429`, grants que no impiden lo que el texto dice), no de código.
- #204: el único **bloqueante** de los dos lotes (el descuento global recalculado a nivel de documento) era una regla de dominio que nadie había fijado antes de escribir `Compute`.

### 5. Revisión y QA en el mismo worktree se contaminaban

- #202, QA (nota T1): `git status` mostró `M apps/api/.go-arch-lint.yml` y luego `M apps/api/cmd/wanka/main.go`, sin que QA los tocara. Eran las mutaciones de la revisión, que trabajaba en el mismo árbol.
- #205, QA: un archivo de migración del worktree cambió durante unos segundos por un proceso ajeno, y un binario compilado en ese instante embebió la migración mutada.
- #201, QA: el árbol tenía `apps/api/testdata/loadtest/session/` de otro agente.
- Desde #206, cada informe de QA declara un worktree propio en modo detached (`qa-206`, `qa-208`, `qa-209`…) y no se repitió.

### 6. Tests con cotas de tiempo: las 4 fallas de CI fueron falsas alarmas

- Las **4 corridas de CI fallidas** de los dos lotes fueron tests de tiempo o de concurrencia, ninguna un defecto:
  - `TestCompute200LinesUnder5ms` en #205 y #208 (bajo `-race`);
  - `TestPoolWaitStaysUnder50msWith44ConcurrentJobs` en #211;
  - `TestConcurrentRecords` en #212, con más concurrencia que el pool.
- Cada falla costó un commit de arreglo y una corrida más de CI (#206 «skip the pricing timing bound under -race», #212 «bound the concurrent audit records test to the pool size»).

### 7. Los criterios no funcionales se descubrían sin medir en la revisión

- 4 PR cerraban su issue sin el número que el criterio pedía «medido y reportado»: #206 H2 (lectura caliente y fría), #211 H1 (el test no mide lo que dice medir), #212 H4 (costo de `Record`) y #213 H4 (los documentos fijan mediciones que la PR no hace).
- En los cuatro, la revisión tuvo que correr los benchmarks ella misma y pedir otra ronda.

### 8. CodeRabbit sin cupo en `bacsystem/skills`

- Desde el 2026-11-14, **8 de los 16 PR** de `bacsystem/skills` (#15 a #31) recibieron «Review limit reached» en lugar de una revisión, entre ellos los tres últimos (#29, #30 y #31).
- La revisión externa de los cambios a los agentes depende de un cupo que se agota justo cuando más PR se abren.

## Clases de defecto

Las que ya estaban en [`architecture/review-checklist.md`](../architecture/review-checklist.md) (fallar cerrado, principal, particiones, políticas exactas, zona horaria, escala, ida y vuelta, mutaciones, oráculo independiente, cableado, `WWW-Authenticate`, IDs de caso) aparecieron en #201, #204, #206, #207, #208, #209, #210 y #213. Clases nuevas, agregadas a la lista con su PR:

| Clase | Hallazgos |
|---|---|
| Afirmaciones de documentación que nada comprueba o que se contradicen | #203 H2, H5, H10; #205 H4; #206 H5; #209 H2; #213 H4; #214 H1 |
| Criterio no funcional sin medir, o un test que no mide lo que dice | #206 H2; #211 H1; #212 H4; #213 H4 |
| Grants por columna que no impiden lo que el texto dice | #203 H3; #213 H3 |
| Límites de abuso por ventana, sin techo absoluto | #203 H1, H6 |
| Enmascarado de secretos que no llega a valores anidados o a JSON en `[]byte` | #202 H4, H7; #212 H1, H2 |
| Variable de entorno o valor de configuración que no está en la documentación de despliegue | #202 H1, H3; #211 H6 |
| Límite de arquitectura sin guarda en `go-arch-lint` | #202 H2; #211 H5 |
| Error permanente marcado como reintentable | #202 H6 |
| Topes de `numeric` y escala de columna fuera de algunos importes | #204 H3, H4 |
| Filtro de rutas de la CI que salta un test cuya entrada cambió | #205 H1 |
| Regla de formato que solo vive en Go y no en un `CHECK` | #212 H3 |
| Guarda exportada que no se monta en producción | #213 H1 |
| Invalidación de caché que no corta la lectura en vuelo | #206 H3 |
| Lista blanca que cita un ADR inexistente | #210 H4 |
| Timeout implícito en el default de una librería | #211 H2 |
| Test de tiempo o concurrencia que falla en CI | CI de #205, #208, #211, #212 |
| Defectos introducidos por el merge de la base | #212 H5; #213 H2 |
| Workflow idempotente por existencia, sin comparar el objeto | #214 H3, H4, H5 |

## Mejoras ya aplicadas

| Mejora | Cuello de botella que quita | Dónde vive |
|---|---|---|
| Historias en PR independientes desde `main`, sin tocar la versión; un PR de release por lote; tag automático con `release-tag.yml` | 1 | PR #214: `docs/delivery-process.md`, `CLAUDE.md`, `.github/workflows/release-tag.yml` |
| Una corrida completa de la suite por PR; tras integrar una base, compilar y dejar que verifique la CI; `rls-check` solo con cambios de SQL y en el release | 2 | PR #214: `docs/delivery-process.md` («Rendimiento», «CI por PR y por release»), `.github/workflows/api.yml`; PR #31 de skills: `agents/go-dev.md`, `agents/qa.md`, `pr-review` («Running cost») |
| Base de datos de tests compartida (`wanka-shared-pg`), como mucho 4 agentes con base a la vez, nombres únicos de contenedor | 3 | PR #214: `docs/delivery-process.md` («Entorno local de agentes»); PR #31: `agents/tech-lead.md` |
| Revisión previa del architect con 8 puntos antes de implementar | 4 | PR #214: `docs/delivery-process.md`; PR #31: `agents/architect.md` |
| QA en un worktree propio en modo detached; revisión y QA en una sola pasada en los PR sin riesgo | 5 | PR #214: `docs/delivery-process.md` («Revisión y QA»); PR #31: `agents/qa.md`, `agents/tech-lead.md` |
| Lista de errores conocidos, leída antes de escribir código | clases repetidas | PR #214: `docs/architecture/review-checklist.md` |
| Retro por lote con datos de GitHub | todos | PR #214: `docs/delivery-process.md`; PR #31: skill `retro` |
| Informes de 15 líneas como máximo; el detalle va a un archivo | contexto del coordinador | PR #214; PR #31 |

## Mejoras nuevas

Del proyecto (esta retro):

- **La lista de errores conocidos suma las 18 clases nuevas** de la tabla anterior, cada una con su PR (`docs/architecture/review-checklist.md`). Ataca los cuellos 6 y 7: los tests de tiempo con cotas que la CI pueda cumplir y los criterios no funcionales medidos antes de pedir la revisión, con el número en el cuerpo del PR.
- **Los contratos de diseño (PR solo de docs bajo `docs/architecture/`) pasan por la revisión previa del architect como cualquier historia.** #203 fue el PR con más hallazgos del lote, y todos eran de diseño. No hace falta una regla nueva: la de «Revisión previa del architect» ya lo cubre si se aplica también a las historias de docs. Se vigila abajo.

Genéricas, para `bacsystem/skills` (las sube el coordinador; no están en el PR #31):

- `retro/scripts/collect.sh`: contar las rondas por comentario y no por línea, separar los merges de la base de los commits de trabajo, dar minutos en vez de horas enteras y clasificar las fallas de CI por test.
- `pr-review`: comprobar que la CI corrió sobre el SHA revisado (un PR `DIRTY` no tiene CI) y leer el diff de cada merge de la base en los archivos compartidos.
- `pr-review` y `go-dev`: un criterio no funcional que pide «medido y reportado» no se aprueba sin el número en el PR; un test de tiempo tiene una cota que la CI cumpla con `-race` o corre aparte.
- Repo de skills: no depender de CodeRabbit como revisión de los cambios a agentes; `/pr-review` es la puerta y conviene un PR de skills por retro.

## A vigilar en el próximo lote

| Métrica | Lotes 1 y 2 | Objetivo |
|---|---|---|
| Rondas de revisión por PR | 2,8 y 2,6 | ≤ 2 de media; ningún PR con más de 3 |
| Decisiones del architect después de implementar | 6 | 0: todas quedan en la issue antes de escribir código |
| Merges de la base en PR de historia | 32 (40 % de los commits) | 0 en PR independientes; solo en los apilados por dependencia real |
| Defectos introducidos por un merge | 4 | 0 |
| Corridas completas de la suite por PR (revisión + QA) | hasta 5 | 1 |
| Corridas de CI fallidas por tests de tiempo | 4 de 4 | 0 |
| Criterios no funcionales sin número en el PR | 4 PR | 0 |
| Hallazgos importantes por PR | 1,8 y 1,4 | ≤ 1 |
| Contaminación entre worktrees reportada por QA | 3 PR | 0 |
| PR de skills sin revisión externa por falta de cupo | 8 de 16 | revisados con `/pr-review` |
