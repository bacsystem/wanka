"use client";

import { Columns3, Rows3, RotateCcw } from "lucide-react";
import * as React from "react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuGroup, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { EmptyState } from "./states";

export interface Column<T> {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  headClassName?: string;
  /** Plain label for the "Columnas" menu when `header` is not a string. */
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
  rowClassName?: (row: T) => string | undefined;
  /** Column visibility + density controls (Stitch "Columnas / Restablecer"); on by default. */
  controls?: boolean;
  /** Extra content on the left of the controls bar (counts, filters). */
  controlsSlot?: ReactNode;
  /** Totals row: cell content keyed by column key (missing keys render empty). */
  footer?: Partial<Record<string, ReactNode>>;
}

const alignClass = { left: "text-left", right: "text-right", center: "text-center" };

export function DataTable<T>({
  columns: allColumns,
  rows,
  rowKey,
  mobileCard,
  minWidth = "760px",
  emptyMessage = "Sin resultados para los filtros seleccionados.",
  className,
  onRowClick,
  rowClassName,
  controls = true,
  controlsSlot,
  footer,
}: DataTableProps<T>) {
  const defaults = React.useMemo(() => Object.fromEntries(allColumns.map((c) => [c.key, !c.hidden])) as Record<string, boolean>, [allColumns]);
  const [visible, setVisible] = React.useState<Record<string, boolean>>({});
  const [dense, setDense] = React.useState(false);
  const isLocked = (c: Column<T>) => c.locked ?? ["select", "sel", "actions"].includes(c.key);
  const isVisible = (c: Column<T>) => isLocked(c) || (visible[c.key] ?? defaults[c.key] ?? true);
  const columns = allColumns.filter(isVisible);
  const visibleDataColumns = columns.filter((c) => !isLocked(c)).length;
  const dirty = dense || Object.keys(visible).some((k) => visible[k] !== defaults[k]);
  const labelOf = (c: Column<T>) => c.label ?? (typeof c.header === "string" ? c.header : c.key);
  const reset = () => { setVisible({}); setDense(false); };

  const bar = controls ? (
    <div className={cn("flex flex-wrap items-center justify-between gap-2 border-b bg-muted/30 px-3 py-1.5 text-xs text-muted-foreground", mobileCard && "hidden md:flex")}>
      <div className="min-w-0">{controlsSlot ?? <span>{rows.length} registro{rows.length === 1 ? "" : "s"} · {columns.length}/{allColumns.length} columnas</span>}</div>
      <div className="flex items-center gap-1">
        <Button variant="ghost" size="sm" className="h-7 text-xs" aria-pressed={dense} onClick={() => setDense((d) => !d)}><Rows3 data-icon="inline-start" className={cn("transition-transform", dense && "scale-y-75")} /> {dense ? "Compacta" : "Cómoda"}</Button>
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="h-7 text-xs" />}><Columns3 data-icon="inline-start" /> Columnas</DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-60">
            <DropdownMenuGroup>
              <DropdownMenuLabel className="flex items-center justify-between text-xs"><span className="font-semibold text-foreground">Personalizar columnas</span><button type="button" className="flex items-center gap-1 font-medium text-primary hover:underline disabled:opacity-50" disabled={!dirty} onClick={reset}><RotateCcw className="size-3" /> Restablecer</button></DropdownMenuLabel>
              {allColumns.map((c) => isLocked(c) ? (
                <DropdownMenuCheckboxItem key={c.key} checked disabled closeOnClick={false} className="justify-between">{labelOf(c) === c.key ? "Acciones" : labelOf(c)}<span className="ml-auto text-[10px] text-muted-foreground">Fijo</span></DropdownMenuCheckboxItem>
              ) : (
                <DropdownMenuCheckboxItem key={c.key} checked={isVisible(c)} disabled={isVisible(c) && visibleDataColumns === 1} onCheckedChange={(v) => setVisible((s) => ({ ...s, [c.key]: Boolean(v) }))} closeOnClick={false}>{labelOf(c)}</DropdownMenuCheckboxItem>
              ))}
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuGroup><DropdownMenuLabel className="text-[11px] font-normal text-muted-foreground">{columns.length} activas de {allColumns.length}</DropdownMenuLabel></DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        {dirty ? <Button variant="ghost" size="sm" className="h-7 text-xs text-primary" onClick={reset}><RotateCcw data-icon="inline-start" /> Restablecer</Button> : null}
      </div>
    </div>
  ) : null;

  if (rows.length === 0) {
    return <div className={className}>{bar}<EmptyState title="Sin resultados" text={emptyMessage} className="py-10" /></div>;
  }
  return (
    <div className={className}>
      {bar}
      <div className={cn("overflow-x-auto", mobileCard && "hidden md:block")}>
        <Table className={cn(dense && "[&_td]:py-1 [&_th]:h-8")} style={{ minWidth }}>
          <TableHeader className="bg-card">
            <TableRow>
              {columns.map((c, i) => (
                <TableHead
                  key={c.key}
                  className={cn(
                    "whitespace-nowrap",
                    alignClass[c.align ?? "left"],
                    i === 0 && "pl-4",
                    i === columns.length - 1 && "pr-4",
                    c.headClassName,
                  )}
                >
                  {c.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={rowKey(row)} onClick={onRowClick ? () => onRowClick(row) : undefined} className={rowClassName?.(row)}>
                {columns.map((c, i) => (
                  <TableCell
                    key={c.key}
                    className={cn(
                      alignClass[c.align ?? "left"],
                      i === 0 && "pl-4",
                      i === columns.length - 1 && "pr-4",
                      c.className,
                    )}
                  >
                    {c.cell(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
          {footer ? (
            <tfoot className="border-t bg-muted/40 font-semibold">
              <tr>{columns.map((c, i) => <td key={c.key} className={cn("px-2 py-2", alignClass[c.align ?? "left"], i === 0 && "pl-4", i === columns.length - 1 && "pr-4", c.className)}>{footer[c.key]}</td>)}</tr>
            </tfoot>
          ) : null}
        </Table>
      </div>
      {mobileCard ? (
        <ul className="divide-y md:hidden">
          {rows.map((row) => (
            <li key={rowKey(row)} className={cn("px-4 py-3", rowClassName?.(row))} onClick={onRowClick ? () => onRowClick(row) : undefined}>
              {mobileCard(row)}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
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
