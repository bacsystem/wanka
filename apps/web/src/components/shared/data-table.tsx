"use client";

import * as React from "react";
import type { ReactNode } from "react";
import { TablaListado, type ColumnaListado } from "khipu-design-system/datos/tabla-listado";
import { cn } from "@/lib/utils";

export interface Column<T> {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  /** The DS table aligns left or right: `center` falls back to left. */
  align?: "left" | "right" | "center";
  className?: string;
  /** @deprecated Ignored: the DS table owns header styling. */
  headClassName?: string;
  /** Plain label for the "Columnas" menu and the CSV when `header` is not a string. */
  label?: string;
  /** Hidden by default (user can enable it from the menu). */
  hidden?: boolean;
  /** Cannot be hidden (selection, actions…). */
  locked?: boolean;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  /** Card renderer for ≤767px; when omitted the table also renders on mobile with horizontal scroll. */
  mobileCard?: (row: T) => ReactNode;
  minWidth?: string;
  emptyMessage?: string;
  className?: string;
  onRowClick?: (row: T) => void;
  /** @deprecated Ignored: the DS table has no per-row class hook (use the DS `filaActiva` when the module migrates). */
  rowClassName?: (row: T) => string | undefined;
  /** Column menu, search and export bar; on by default. `false` renders the bare table. */
  controls?: boolean;
  /** Extra content under the controls bar (counts, filters). */
  controlsSlot?: ReactNode;
  /** Totals row: cell content keyed by column key (missing keys render empty). */
  footer?: Partial<Record<string, ReactNode>>;
}

/** Plain text of a rendered cell (strings and numbers found in the element tree), for search and CSV. */
function nodeText(node: ReactNode): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(nodeText).join(" ").trim();
  if (React.isValidElement<{ children?: ReactNode }>(node)) return nodeText(node.props.children);
  return "";
}

const LOCKED_KEYS = ["select", "sel", "actions"];

/**
 * @deprecated Adapter over the DS `TablaListado` (search, CSV, print, column menu, pagination). Modules move to
 * `khipu-design-system/datos/tabla-listado` directly; W8 deletes this file.
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  mobileCard,
  minWidth = "760px",
  emptyMessage = "Sin resultados para los filtros seleccionados.",
  className,
  onRowClick,
  controls = true,
  controlsSlot,
  footer,
}: DataTableProps<T>) {
  const cols: ColumnaListado<T>[] = columns.map((c) => ({
    id: c.key,
    titulo: c.header,
    render: c.cell,
    alinear: c.align === "right" ? "derecha" : "izquierda",
    className: c.className,
    tituloTexto: c.label,
    texto: (row) => nodeText(c.cell(row)),
    ocultable: controls && !(c.locked ?? LOCKED_KEYS.includes(c.key)),
    ocultaInicial: c.hidden,
  }));
  return (
    <TablaListado
      columnas={cols}
      filas={rows}
      clave={rowKey}
      titulo="Listado"
      buscar={controls}
      exportar={controls}
      filtros={controls ? controlsSlot : undefined}
      vacio={emptyMessage}
      anchoMinimo={minWidth}
      onFila={onRowClick}
      tarjetaMovil={mobileCard}
      pie={footer}
      className={className}
    />
  );
}

/** Small helper for the two-line "primary / secondary" cell used across lists. */
export function TwoLine({ primary, secondary, mono }: { primary: ReactNode; secondary?: ReactNode; mono?: boolean }) {
  return (
    <div className="flex min-w-0 flex-col">
      <span className="truncate">{primary}</span>
      {secondary ? (
        <span className={cn("truncate text-xs text-muted-foreground", mono && "font-mono tabular-nums")}>{secondary}</span>
      ) : null}
    </div>
  );
}
