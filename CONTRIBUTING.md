# Cómo contribuir a Wanka

Esta guía explica cómo tomar una épica del backlog y cómo se controla su avance. Las reglas técnicas del repositorio (comandos, arquitectura, convenciones de UI, design system, reglas del equipo) están en [`CLAUDE.md`](CLAUDE.md), y el diseño del backend, en [`docs/architecture/`](docs/architecture/README.md). Aquí no se repiten.

El orden de construcción (prioridad P0–P3, dependencias y olas) está en el [mapa de módulos](docs/modules/README.md). El avance vivo está en el Project público [Wanka · Mapa de módulos](https://github.com/users/bacsystem/projects/8).

## Cómo tomar una épica

1. **Elige una épica disponible.** Está en «Por hacer» en el Project, y todas sus relaciones *Blocked by* están cerradas o el contrato que necesita ya está publicado en la épica (ver «Olas y contratos que se fijan antes de abrirlas» en el [mapa de módulos](docs/modules/README.md)). Empieza por la prioridad más alta (P0 antes que P1) y el orden más bajo (`M01` antes que `M02`).
2. **Anúnciala.** Comenta «Tomo esta épica» en el issue, asígnate y mueve su Estado a «En curso» en el Project.
3. **Una historia, una rama, un PR.** Cada historia (sub-issue de la épica) se trabaja en su propia rama y en su propio worktree (`git worktree add ../wanka-wt/<historia> -b <tipo>/<historia>-<tema> origin/main`), y se entrega en un PR con `Closes #<historia>` en el cuerpo. No mezcles historias en un PR.
4. **Verifica antes de pedir revisión.** Corre las verificaciones de [`CLAUDE.md`](CLAUDE.md#commands) que correspondan al cambio y pega su salida real en el PR, con la línea de resumen. Lo que no se pudo correr se dice, no se omite.
5. **Revisión.** El PR pasa por `/pr-review` y su informe se publica como comentario. Después de cada commit nuevo se vuelve a revisar, hasta que el veredicto quede limpio.
6. **Merge.** Solo con la aprobación explícita del dueño, y con *merge commit* (sin squash ni rebase).

## Cómo se controla el avance

- **Estado en el Project:** «Por hacer», «Bloqueado», «En curso», «En revisión» y «Hecho». Quien tiene la épica mantiene su Estado al día; los workflows del Project la pasan a «Hecho» al cerrarse.
- **Barra de sub-issues:** cada épica muestra cuántas de sus historias están cerradas. Una historia se cierra al mergear su PR.
- **Épicas sin movimiento:** si una épica pasa 14 días sin commits, PR ni comentarios, se avisa en el issue. Si nadie responde, vuelve a «Por hacer» y se desasigna, para que otra persona la tome.
- **Cierre de la épica:** cuando qa valida sus criterios de aceptación, no solo cuando se cierran todas sus historias.

## Dudas

Se preguntan en el issue de la épica o de la historia: producto, al po; arquitectura, datos y contratos de API, al architect; el resto de lo técnico, al tech-lead.
