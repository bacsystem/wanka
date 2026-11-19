# Trabajar en Wanka con Claude Code o con Gemini

Cualquier colaborador puede trabajar con **Claude Code** o con **Gemini** (Gemini CLI o Antigravity). Esta guía cubre tres cosas: dónde viven las reglas, cómo se prepara cada herramienta y cómo se retoma un trabajo empezado por otra persona o por otra IA, sin perder nada.

## 1. Dónde vive cada cosa

Ninguna regla del equipo depende de la memoria privada de una IA. Las fuentes de verdad son estas:

| Qué | Dónde | Lo leen |
|---|---|---|
| Reglas del proyecto y del equipo | `CLAUDE.md` (la única que se edita) | Claude Code lo carga solo. Gemini lo carga a través de `GEMINI.md`, que solo lo importa. Ver [`agent-context.md`](agent-context.md) |
| Proceso de entrega: lotes, versión, definición de terminado, revisión, QA y CI | [`delivery-process.md`](delivery-process.md) | todas |
| Errores ya cometidos que no hay que repetir | [`architecture/review-checklist.md`](architecture/review-checklist.md) | todas, antes de escribir código |
| Arquitectura, contratos y decisiones | `docs/architecture/` (ADR, `api/`, `data/`) | todas |
| Qué hay que hacer y su estado | issues de GitHub (épicas, historias con criterios y etiquetas `capa:backend`/`capa:frontend`) | todas |
| Qué se hizo y por qué | PR, sus revisiones y QA publicados como comentarios, y el `CHANGELOG.md` | todas |
| Bitácora del proyecto y decisiones de sesión | `wanka-memory`, el cuaderno de NotebookLM del dueño ([`CLAUDE.md`](../CLAUDE.md), «Project memory»), con fuentes `sesion-*` y `semana-*` | quien tenga acceso al cuaderno |
| Agentes (tech-lead, architect, po, go-dev, nextjs-dev, qa, devops) y skills (`git-flow`, `pr-review`) | repo [`bacsystem/skills`](https://github.com/bacsystem/skills) | Claude y Gemini, con el mismo contenido |

Cada IA puede guardar su propia memoria local, como la *auto-memory* de Claude Code. Esa memoria es solo una ayuda y nunca la fuente de verdad. Toda regla que el dueño fija en una conversación se pasa a `CLAUDE.md` o a `docs/` en el mismo lote de trabajo.

## 2. Preparar la herramienta

1. Clona `bacsystem/skills` e instala los agentes y skills para tu herramienta:
   ```sh
   ./scripts/instalar.sh --claude   # Claude Code
   ./scripts/instalar.sh --gemini   # Gemini CLI / Antigravity (~/.gemini/skills y ~/.gemini/config/agents)
   ```
   En Windows: `powershell -ExecutionPolicy Bypass -File .\scripts\instalar.ps1 -Target gemini`. Los agentes son genéricos y toman lo propio de Wanka de `CLAUDE.md` y de estos docs.
   **Mantenerlos al día:** al empezar cada jornada, corre `git pull` en el clon de `bacsystem/skills`. Los skills son enlaces y se actualizan solos. Los agentes de Gemini son copias adaptadas, así que vuelve a correr `./scripts/instalar.sh --gemini`. El `CHANGELOG.md` de ese repo dice qué cambió.
2. Abre la sesión en la raíz del repo de Wanka, para que se cargue `CLAUDE.md` o `GEMINI.md`.
3. `wanka-memory` (opcional, solo si tienes acceso): el servidor MCP `gemini-notebook-mcp` o la CLI `nlm` (`nlm login`). Si no tienes acceso, sáltalo. El estado del proyecto también está en GitHub.
4. Base de datos local para los tests: sigue «Base local» en `apps/api/README.md`. Si varios agentes van a correr tests a la vez, levanta un solo PostgreSQL compartido (ver [`delivery-process.md`](delivery-process.md), «Entorno local de agentes»), con tus propias claves locales. Nunca se suben al repo.

## 3. Cómo se implementa (igual con cualquier IA)

1. **Antes de implementar.** Lee la historia y sus criterios. El agente architect hace la revisión previa de la lista de su definición (hoy 11 puntos) y deja sus decisiones en la issue. Después lee `review-checklist.md`.
2. **Una historia, un worktree y una rama desde `main`.** Usa `git worktree add -b feat/<n>-<tema> ../wanka-wt/<n>-<tema> origin/main`. No cambies de rama en una carpeta que usa otro.
3. **Durante el trabajo.** TDD y una mutación por cada protección importante. Corre solo los paquetes que tocas. La suite completa se corre una vez, al final.
4. **Commit y PR con `/git-flow`.** Commits en inglés y Conventional Commits. No subas la versión: agrega tu entrada en `CHANGELOG.md` bajo `[Unreleased]`.
5. **Revisión con `/pr-review`**, publicada en el PR. **QA** se hace sobre el SHA final, en otro worktree. CI en verde. Los checkboxes de la issue se tildan con evidencia.
6. **El dueño mergea por lotes y después llega el release.** El tag lo crea `release-tag.yml`. Al cerrar el lote, quien coordina corre el skill `retro` (ver «Retrospectiva por lote» en [`delivery-process.md`](delivery-process.md)).
7. **Frontend (`apps/web`):** antes de empezar, avisa al dueño. Él fija una regla para el frontend que todavía no está escrita.

## 4. Tomar una épica

Una épica se toma si **no tiene *blocked by* abiertos** en GitHub. Al 2026-11-19, las épicas de la ola 1 son #3 (identidad) y #4 (empresas). Dos personas pueden tomar las dos en paralelo: el único punto que comparten es el DDL de `iam` y `org`, y lo fija el architect antes de empezar ([`modules/README.md`](modules/README.md), «Olas y contratos»).

1. Comenta «Tomo esta épica» en la issue, asígnatela y pon su `Status` en «En curso» en el Project «Wanka · Mapa de módulos».
2. **Contrato antes que código.** Con el agente architect, publica en la épica el contrato de la ola que te toca, en su sección de [`modules/README.md`](modules/README.md) (DDL, rutas, firmas que usan los otros módulos), y hazle la revisión previa a cada historia. Lo que cambie un contrato congelado (`api/resources.md`, `api/conventions.md`, un ADR) va primero en su propio PR.
3. **Historias en orden de dependencia,** una por PR, cada una en su worktree (sección 3). Mueve cada historia a «En curso» cuando la empiezas. El Project la pasa solo a «En revisión» cuando el PR la enlaza y a «Hecho» cuando se cierra.
4. **Frontend:** las partes con `capa:frontend` esperan la regla del dueño (sección 3, punto 7). Mientras tanto se avanza con el backend.
5. **Al terminar cada lote:** release, `retro` y `docs-sync` sobre los docs de estado ([`delivery-process.md`](delivery-process.md), «Tablero de GitHub siempre al día»).

Las preguntas de estado («¿qué falta?», «¿está hecha?») se responden con GitHub (`gh issue view`, `gh pr list`), no con lo que diga un doc o la memoria de la IA. Si no coinciden, se corre `docs-sync` y se corrige el doc.

## 5. Retomar un trabajo empezado por otro

Lo que una IA dejó hecho tiene que poder retomarlo otra sin la conversación original:

- **Estado:** `gh pr list` (cada PR dice su base, su versión y si está apilado) y los comentarios del PR (revisiones, re-revisiones y QA con sus IDs de hallazgo). En la issue están los checkboxes y la evidencia.
- **Por qué se decidió algo:** en los comentarios de la issue o del PR, en los ADR y en `wanka-memory`. Una decisión del dueño que todavía no está en un doc se escribe como comentario en la issue, con la fecha.
- **Al cortar una sesión a mitad de camino:** no dejes cambios sin commitear en un worktree compartido. Commitea en tu rama, haz push y deja un comentario en el PR o la issue con lo que falta. Una conversación de IA no es un lugar donde dejar estado.
- **Trabajo en paralelo con otra persona:** cada una en su propio worktree y rama. Los archivos del scratchpad y los contenedores llevan el prefijo de tu historia, y nadie borra lo que no creó.

## 6. Reglas que el dueño fijó en conversación y no estaban escritas en otro lado (2026-11-19)

- **Pedido de merge:** a quien pide el merge le basta un «mergea el #N». Si son varios, se dan en orden: «Mergea en este orden: #A, #B». El dueño mergea desde GitHub con merge commit; squash y rebase están deshabilitados. Nadie más mergea ni usa `--admin`.
- **Por ahora todo es local:** no se despliega en Railway.
- **Etiquetas:** las épicas y las historias llevan `capa:backend` y/o `capa:frontend`.
- **Historia lista para implementar:** criterios precisos y verificables, con un diagrama de flujo.
