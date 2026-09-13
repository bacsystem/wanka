"use client";

import { CalendarDays, CircleDollarSign, Code2, Download, Eye, FileDown, Filter, Mail, MessageCircle, MoreHorizontal, RefreshCw, Send, X } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { cn } from "@/lib/utils";
import { formatCurrency, formatDate, formatDocumentNumber } from "@/lib/format";
import type { DocumentType, SalesDocument, SunatStatus } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";
import { DocumentTypeBadge } from "./document-type-badge";
import { SunatStatusBadge } from "./sunat-status-badge";
import { FilterBar } from "@/components/shared/filter-bar";

const typeOptions: { value: DocumentType | "all"; label: string }[] = [
  { value: "all", label: "Todos los tipos" },
  { value: "factura", label: "Factura (01)" },
  { value: "boleta", label: "Boleta de venta (03)" },
  { value: "nota_credito", label: "Nota de crédito (07)" },
];
const statusOptions: { value: SunatStatus | "all"; label: string }[] = [
  { value: "all", label: "Todos los estados" },
  { value: "aceptado", label: "Aceptado SUNAT" },
  { value: "pendiente", label: "Pendiente / OSE" },
  { value: "rechazado", label: "Rechazado" },
];

function timeOf(iso: string) {
  return iso.slice(11, 16);
}
const docLabel = (doc: string) => (doc.length === 11 ? `RUC ${doc}` : doc.length === 8 ? `DNI ${doc}` : doc);
const dateRanges = [{ value: "month", label: "Este mes (01/09 – 30/09/2026)" }, { value: "prev", label: "Mes anterior (agosto)" }, { value: "week", label: "Últimos 7 días" }, { value: "custom", label: "Rango personalizado…" }];

export function DocumentsList({ documents }: { documents: SalesDocument[] }) {
  const [query, setQuery] = React.useState("");
  const [type, setType] = React.useState<string>("all");
  const [status, setStatus] = React.useState<string>("all");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());

  const rows = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    return documents.filter(
      (d) =>
        (type === "all" || d.type === type) &&
        (status === "all" || d.sunatStatus === status) &&
        (!q || d.id.toLowerCase().includes(q) || d.customerName.toLowerCase().includes(q) || d.customerDocument.includes(q)),
    );
  }, [documents, query, type, status]);

  const selectedTotal = rows.filter((r) => selected.has(r.id)).reduce((s, r) => s + r.total, 0);
  const allSelected = rows.length > 0 && rows.every((r) => selected.has(r.id));

  function toggle(id: string, on: boolean) {
    setSelected((s) => {
      const n = new Set(s);
      if (on) n.add(id);
      else n.delete(id);
      return n;
    });
  }

  const columns: Column<SalesDocument>[] = [
    {
      key: "select",
      header: (
        <Checkbox
          aria-label="Seleccionar todos"
          checked={allSelected}
          onCheckedChange={(v) => setSelected(v ? new Set(rows.map((r) => r.id)) : new Set())}
        />
      ),
      cell: (d) => (
        <Checkbox aria-label={`Seleccionar ${d.id}`} checked={selected.has(d.id)} onCheckedChange={(v) => toggle(d.id, Boolean(v))} />
      ),
      className: "w-8",
    },
    { key: "type", header: "Tipo", cell: (d) => <DocumentTypeBadge type={d.type} /> },
    {
      key: "number",
      header: "Serie-Número",
      cell: (d) => (
        <Link href={`/ventas/comprobantes/${d.id}`} className={cn("font-mono text-xs font-semibold tabular-nums hover:underline", d.sunatStatus === "rechazado" ? "text-destructive" : "text-primary")}>
          {formatDocumentNumber(d.series, d.number)}
        </Link>
      ),
    },
    {
      key: "date",
      header: "Fecha y hora",
      cell: (d) => <span className="whitespace-nowrap"><span className="font-medium">{formatDate(d.issuedAt)}</span> <span className="text-muted-foreground">{timeOf(d.issuedAt)}</span></span>,
      className: "whitespace-nowrap",
    },
    { key: "customer", header: "Cliente / adquirente", cell: (d) => <TwoLine primary={d.customerName} secondary={d.sunatStatus === "rechazado" ? <span className="text-destructive">{docLabel(d.customerDocument)} · Error 2023: Doc. inválido</span> : docLabel(d.customerDocument)} /> },
    { key: "currency", header: "Moneda", align: "center", headClassName: "hidden 2xl:table-cell", className: "hidden 2xl:table-cell", cell: (d) => <span className="text-muted-foreground">{d.currency}</span> },
    {
      key: "base",
      header: "Op. gravada",
      align: "right",
      cell: (d) => <span className="font-mono tabular-nums">{formatCurrency(d.total / 1.18, d.currency)}</span>,
    },
    { key: "total", header: "Total", align: "right", cell: (d) => <span className={cn("font-mono font-bold tabular-nums", d.sunatStatus === "rechazado" && "text-destructive")}>{d.type === "nota_credito" ? "-" : ""}{formatCurrency(d.total, d.currency)}</span> },
    { key: "status", header: "Estado SUNAT / CDR", cell: (d) => <SunatStatusBadge status={d.sunatStatus} /> },
    {
      key: "actions",
      header: <span className="sr-only">Acciones</span>,
      align: "right",
      cell: (d) => <RowActions doc={d} />,
    },
  ];

  const anyFilter = query || type !== "all" || status !== "all";
  const taxable = rows.reduce((t, r) => t + r.total / 1.18, 0);
  const total = rows.reduce((t, r) => t + r.total, 0);
  return (
    <div className="flex flex-col gap-4">
      <FilterBar>
        <SearchInput placeholder="Buscar por serie-número, RUC, DNI o cliente…" value={query} onChange={setQuery} className="sm:max-w-md" />
        <Select defaultValue="month" items={dateRanges}>
          <SelectTrigger className="w-64 bg-muted/50" aria-label="Periodo"><CalendarDays className="size-4 text-muted-foreground" /><SelectValue /></SelectTrigger>
          <SelectContent>{dateRanges.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={type} onValueChange={(v) => setType(String(v))} items={typeOptions}>
          <SelectTrigger className="w-44 bg-muted/50" aria-label="Tipo de comprobante"><SelectValue /></SelectTrigger>
          <SelectContent>{typeOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={status} onValueChange={(v) => setStatus(String(v))} items={statusOptions}>
          <SelectTrigger className="w-44 bg-muted/50" aria-label="Estado SUNAT"><SelectValue /></SelectTrigger>
          <SelectContent>{statusOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
        </Select>
        <div className="ml-auto flex items-center gap-1">
          <Button variant="outline" size="icon-sm" aria-label="Más filtros" onClick={() => toast.info("Filtros avanzados")}><Filter /></Button>
          <Button variant="outline" size="icon-sm" aria-label="Limpiar filtros" disabled={!anyFilter} onClick={() => { setQuery(""); setType("all"); setStatus("all"); }}><X /></Button>
        </div>
      </FilterBar>

      {selected.size > 0 ? (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-blue-200 bg-blue-50 px-3 py-2 text-sm dark:border-blue-900 dark:bg-blue-950/40">
          <Checkbox checked aria-label="Selección activa" onCheckedChange={() => setSelected(new Set())} />
          <span className="font-semibold">{selected.size} comprobante{selected.size === 1 ? "" : "s"} seleccionado{selected.size === 1 ? "" : "s"}</span>
          <span className="text-xs text-muted-foreground">({formatCurrency(selectedTotal)} total selec.)</span>
          <div className="ml-auto flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" className="bg-card text-xs" onClick={() => toast.info("Generando ZIP (PDF + XML)…")}><Download data-icon="inline-start" /> Descargar ZIP (PDF+XML)</Button>
            <Button size="sm" variant="outline" className="bg-card text-xs" onClick={() => toast.success(`${selected.size} comprobantes reenviados a SUNAT`)}><Send data-icon="inline-start" /> Reenviar a SUNAT</Button>
            <Button size="sm" variant="outline" className="bg-card text-xs" onClick={() => toast.success("Correos enviados")}><Mail data-icon="inline-start" /> Enviar por correo</Button>
            <Button size="sm" variant="ghost" className="text-xs" onClick={() => setSelected(new Set())}>Limpiar</Button>
          </div>
        </div>
      ) : null}

      <SectionCard title={<span className="sr-only">Listado de comprobantes</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(d) => d.id}
        minWidth="900px"
        mobileCard={(d) => (
          <Link href={`/ventas/comprobantes/${d.id}`} className="flex items-center gap-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{documentTypeLabel[d.type]}</span>
                <span className="font-mono text-xs text-muted-foreground tabular-nums">{formatDocumentNumber(d.series, d.number)}</span>
              </div>
              <p className="truncate text-sm text-muted-foreground">{d.customerName}</p>
              <p className="text-xs text-muted-foreground">{formatDate(d.issuedAt)} · {timeOf(d.issuedAt)}</p>
            </div>
            <div className="flex flex-col items-end gap-1">
              <span className="font-mono text-sm font-medium tabular-nums">{formatCurrency(d.total, d.currency)}</span>
              <SunatStatusBadge status={d.sunatStatus} />
            </div>
          </Link>
        )}
      />
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1 border-t bg-muted/40 px-4 py-2.5 text-xs">
        <span className="flex items-center gap-2 font-semibold text-muted-foreground"><CircleDollarSign className="size-4" /> Resumen tributario de selección / vista:</span>
        <span className="flex flex-wrap items-center gap-x-5 gap-y-1">
          <span className="text-muted-foreground">OP. GRAVADA: <strong className="font-mono text-foreground">{formatCurrency(taxable)}</strong></span>
          <span className="text-muted-foreground">IGV (18%): <strong className="font-mono text-foreground">{formatCurrency(total - taxable)}</strong></span>
          <span className="rounded-md border bg-card px-2 py-1 text-muted-foreground shadow-xs">TOTAL FACTURADO: <strong className="font-mono text-sm text-foreground">{formatCurrency(total)}</strong></span>
        </span>
      </div>
      <PaginationBar from={1} to={rows.length} total={148} label="comprobantes" />
      </SectionCard>
    </div>
  );
}

function RowActions({ doc }: { doc: SalesDocument }) {
  return (
    <div className="flex items-center justify-end gap-0.5 text-muted-foreground">
      {doc.sunatStatus === "pendiente" ? (
        <Button size="xs" variant="secondary" className="bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-950/60 dark:text-blue-300" aria-label="Reintentar envío" onClick={() => toast.success(`${doc.id} reenviado a SUNAT`)}><RefreshCw data-icon="inline-start" /> Reintentar</Button>
      ) : doc.sunatStatus === "rechazado" ? (
        <Button size="xs" variant="destructive" aria-label="Corregir" render={<Link href={`/ventas/comprobantes/${doc.id}`} />} nativeButton={false}>Corregir</Button>
      ) : null}
      <Button variant="ghost" size="icon-sm" aria-label="Ver comprobante" render={<Link href={`/ventas/comprobantes/${doc.id}`} />} nativeButton={false}>
        <Eye />
      </Button>
      <Button variant="ghost" size="icon-sm" aria-label="Descargar PDF" onClick={() => toast.info(`Descargando ${doc.id}.pdf`)}>
        <FileDown />
      </Button>
      <Button variant="ghost" size="icon-sm" aria-label="Descargar XML" className="hidden 2xl:inline-flex" onClick={() => toast.info(`Descargando ${doc.id}.xml`)}>
        <Code2 />
      </Button>
      <Button variant="ghost" size="icon-sm" aria-label="Enviar por WhatsApp" className="hidden 2xl:inline-flex" onClick={() => toast.success("Enviado por WhatsApp")}>
        <MessageCircle />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Más acciones" />}>
          <MoreHorizontal />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => toast.success("Correo enviado")}><Mail /> Enviar por correo</DropdownMenuItem>
          <DropdownMenuItem><Download /> Descargar XML</DropdownMenuItem>
          <DropdownMenuItem><Download /> Descargar CDR</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem>Emitir nota de crédito</DropdownMenuItem>
          <DropdownMenuItem variant="destructive">Anular / comunicar baja</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
