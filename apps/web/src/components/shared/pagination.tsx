"use client";

import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** Page numbers to render: always first and last, a window around the current page, "…" for gaps. */
export function pageWindow(page: number, pages: number, radius = 1): (number | "…")[] {
  if (pages <= 5) return Array.from({ length: pages }, (_, i) => i + 1);
  const wanted = new Set<number>([1, pages]);
  for (let p = page - radius; p <= page + radius; p++) if (p >= 1 && p <= pages) wanted.add(p);
  const sorted = [...wanted].sort((a, b) => a - b);
  const out: (number | "…")[] = [];
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push("…");
    out.push(p);
  });
  return out;
}

interface PaginationBarProps {
  from: number;
  to: number;
  total: number;
  label?: string;
  pageSize?: number;
  /** Controlled page; when omitted the bar keeps its own page (mock screens without a real data source). */
  page?: number;
  onPageChange?: (page: number) => void;
}

/**
 * Stitch pagination: "Mostrando 1 a 10 de 148 · Filas 25" + numbered pages. `from`/`to` are what the caller
 * is actually showing; until the API exists most screens leave `page` uncontrolled, so the numbers are visual.
 */
export function PaginationBar({ from, to, total, label = "registros", pageSize = 25, page: controlled, onPageChange }: PaginationBarProps) {
  const [internal, setInternal] = React.useState(1);
  const page = controlled ?? internal;
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const setPage = (next: number | ((p: number) => number)) => {
    const value = Math.min(pages, Math.max(1, typeof next === "function" ? next(page) : next));
    if (controlled === undefined) setInternal(value);
    onPageChange?.(value);
  };
  const shown = pageWindow(page, pages);
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-t bg-muted/30 px-4 py-2 text-xs text-muted-foreground">
      <span className="flex items-center gap-3">
        <span>Mostrando <span className="font-semibold text-foreground tabular-nums">{from}</span> a <span className="font-semibold text-foreground tabular-nums">{to}</span> de <span className="font-semibold text-foreground tabular-nums">{total}</span> {label}</span>
        <span className="hidden items-center gap-1.5 sm:flex">Filas: <span className="rounded border bg-card px-2 py-0.5 font-medium text-foreground">{pageSize}</span></span>
      </span>
      <nav className="flex items-center gap-1" aria-label="Paginación">
        <Button variant="ghost" size="icon-xs" aria-label="Primera página" disabled={page === 1} onClick={() => setPage(1)}><ChevronsLeft /></Button>
        <Button variant="ghost" size="icon-xs" aria-label="Anterior" disabled={page === 1} onClick={() => setPage((p) => p - 1)}><ChevronLeft /></Button>
        {shown.map((p, i) => typeof p === "string" ? <span key={`e${i}`} className="px-1">…</span> : (
          <button key={p} type="button" aria-current={page === p ? "page" : undefined} onClick={() => setPage(p)} className={cn("size-7 rounded-md text-xs font-medium tabular-nums transition-colors hover:bg-muted", page === p && "bg-primary text-primary-foreground hover:bg-primary")}>{p}</button>
        ))}
        <Button variant="ghost" size="icon-xs" aria-label="Siguiente" disabled={page === pages} onClick={() => setPage((p) => p + 1)}><ChevronRight /></Button>
        <Button variant="ghost" size="icon-xs" aria-label="Última página" disabled={page === pages} onClick={() => setPage(pages)}><ChevronsRight /></Button>
      </nav>
    </div>
  );
}
