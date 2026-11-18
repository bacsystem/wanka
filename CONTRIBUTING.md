# Cómo contribuir a Wanka

Esta guía explica cómo tomar una épica del backlog y cómo se controla su avance. Las reglas técnicas del repositorio (comandos, arquitectura, convenciones de UI, design system, reglas del equipo) están en [`CLAUDE.md`](CLAUDE.md), y el diseño del backend, en [`docs/architecture/`](docs/architecture/README.md). Aquí no se repiten.

El orden de construcción (prioridad P0–P3, dependencias y olas) está en el [mapa de módulos](docs/modules/README.md). El avance vivo está en el Project público [Wanka · Mapa de módulos](https://github.com/users/bacsystem/projects/8).

## Cómo tomar una épica

1. **Elige una épica disponible.** Su `Status` en el Project es «Por hacer», y todas sus relaciones *Blocked by* están cerradas o el contrato que necesita ya está publicado en la épica (ver «Olas y contratos que se fijan antes de abrirlas» en el [mapa de módulos](docs/modules/README.md)). Empieza por la prioridad más alta (P0 antes que P1) y el orden más bajo (`M01` antes que `M02`).
2. **Anúnciala.** Comenta «Tomo esta épica» en el issue, asígnate y cambia su `Status` a «En curso» en el Project.
3. **Una historia, una rama, un PR.** Cada historia (sub-issue de la épica) se trabaja en su propia rama y en su propio worktree (`git worktree add ../wanka-wt/<historia> -b <tipo>/<historia>-<tema> origin/main`), y se entrega en un PR con `Closes #<historia>` en el cuerpo. No mezcles historias en un PR.
4. **Verifica, revisa y pide el merge según [`CLAUDE.md`](CLAUDE.md).** Los comandos de verificación están en [Commands](CLAUDE.md#commands); la revisión con `/pr-review` y la aprobación del dueño para el merge, en [Team rules](CLAUDE.md#team-rules). En el PR pega la salida real de las verificaciones, con su línea de resumen.

## Reglas del flujo de colaboración

Estas reglas son propias de esta guía (no están en `CLAUDE.md`):

- **Merge commit.** El dueño mergea con *merge commit*, sin squash ni rebase, para que cada historia conserve sus commits.
- **Épica sin movimiento.** Si una épica pasa 14 días sin commits, PR ni comentarios, se avisa en el issue. Si nadie responde, su `Status` vuelve a «Por hacer» y se desasigna, para que otra persona la tome.
- **Cierre de la épica.** Una épica se cierra cuando qa valida sus criterios de aceptación, no solo cuando se cierran todas sus historias.

## Cómo se controla el avance

- **Campo `Status` del Project**, con las opciones «Por hacer», «Bloqueado», «En curso», «En revisión» y «Hecho». Quien tiene la épica mantiene su `Status` al día; los workflows del Project lo pasan a «Hecho» al cerrarse el issue.
- **Barra de sub-issues:** cada épica muestra cuántas de sus historias están cerradas. Una historia se cierra al mergear su PR.

## Roles y dudas

Roles: **po** = producto, **architect** = arquitectura y contratos de API, **tech-lead** = coordinación y revisión, **qa** = validación.

Las dudas se preguntan en el issue de la historia o en la épica, mencionando el rol al que van dirigidas (por ejemplo, «Pregunta para architect: …»).
