"use client";

import * as React from "react";
import { PieTabla } from "khipu-design-system/patrones/pie-tabla";

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
 * "Mostrando x–y de N" + page size + numbered pages. `from`/`to` are what the caller is actually showing;
 * until the API exists most screens leave `page` uncontrolled, so the numbers are visual.
 * @deprecated Temporary W5a adapter over `PieTabla`. Use it directly; deleted in W8.
 */
export function PaginationBar({ from, to, total, label = "registros", pageSize = 25, page: controlled, onPageChange }: PaginationBarProps) {
  const [internal, setInternal] = React.useState(1);
  const [size, setSize] = React.useState(pageSize);
  const page = controlled ?? internal;
  const pages = Math.max(1, Math.ceil(total / size));
  const setPage = (next: number) => {
    const value = Math.min(pages, Math.max(1, next));
    if (controlled === undefined) setInternal(value);
    onPageChange?.(value);
  };
  return (
    <PieTabla
      desde={from}
      hasta={to}
      total={total}
      unidad={label}
      porPagina={size}
      onPorPagina={(n) => {
        setSize(n);
        setInternal(1);
      }}
      pagina={page}
      ultimaPagina={pages}
      onPagina={setPage}
    />
  );
}
