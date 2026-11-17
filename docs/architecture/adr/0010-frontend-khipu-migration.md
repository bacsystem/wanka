# ADR 0010: Migración total del frontend al design system khipu

- Estado: propuesto
- Fecha: 2026-11-17

## Contexto

Wanka adoptó tokens, tema y apariencia del paquete `khipu-design-system` (v0.5.23), pero sigue renderizando con sus copias de shadcn (`components/ui`, 29 archivos) y sus bloques propios (`components/shared`), con Lucide, sonner y Recharts. Decisión del dueño (estándar bacsystem): la app usa siempre los componentes del DS, sin excepciones, y lo que falte se agrega al DS. El plan completo y los contratos de API están en [`docs/frontend-migration-khipu.md`](../../frontend-migration-khipu.md).

## Opciones

1. Migración directa de todo (primitivos y bloques compartidos) módulo por módulo: cada PR de módulo cambia también la API de `DataTable`/`StatCard`, y choca con todos los demás.
2. Adaptadores temporales para todo, incluidos los primitivos: indirección sin beneficio, porque los primitivos ya comparten base (Base UI + cva) y nombres de variantes.
3. Codemod directo para los primitivos; adaptadores finos y temporales solo para `components/shared/*` (mismas props, solo componentes del DS adentro); módulos migrados uno por uno a la API del DS; adaptadores borrados al final. Guardas con baseline que solo puede bajar.

## Decisión

Opción 3. Además, se agrega al DS lo que el contrato responsive de Wanka necesita: rail de 64 px, `BarraInferior` con FAB, tarjetas móviles en tablas, toast imperativo, entrada decimal con coma, gráficos apilado/área/dona, centro de notificaciones, pestañas de ruta y `pie`/`valorConTono` en las métricas, para que ningún contenido de `StatCard` se pierda.

### Decisiones del dueño (confirmadas el 2026-11-17)

Comportamientos de Wanka que cambian o desaparecen al adoptar el DS. El dueño los confirmó explícitamente:

1. Un clic fuera no cierra un diálogo (el valor por defecto del DS). Se cierra con Escape, la X o el pie.
2. Se quita el selector de densidad de filas de las tablas.
3. Los iconos de los KPI van sin caja teñida.
4. NotFound, Forbidden, SessionExpired y Error se componen en Wanka con `EstadoVacio` + `Alerta` del DS. No son componentes nuevos del DS.
5. En móvil no hay hamburguesa. El selector de empresa y el menú de usuario/apariencia pasan al sidebar.
6. Los montos van en Inter `tabular-nums`, no en mono.
7. Los toasts toman el aspecto y la posición del DS (abajo, en vez de arriba a la derecha).

Estos dos se siguen de la regla permanente del dueño «todo lo que tiene khipu, sin excepciones»:

- Los listados ganan la búsqueda, el CSV y la impresión de `TablaListado`.
- `StatCompact` pasa a ser `Metrica`.

## Consecuencias

- Buenas: la app toma el aspecto del DS por familia en pocos PR; los módulos avanzan en paralelo sin pisarse; las guardas impiden volver atrás.
- Malas: durante semanas conviven adaptadores y API del DS; varios PR de Wanka esperan tags del DS; cambios visibles para el usuario (diálogos, toasts, menú lateral, tipografía de montos). Sin CI en el repo, las guardas dependen de que se corra `pnpm lint`.
- Qué haría cambiar la decisión: si el DS no puede publicar a tiempo los tags que bloquean (DS-A, DS-B), se reordena para migrar primero los módulos sin tablas ni gráficos; nunca se resuelve con copias locales.
