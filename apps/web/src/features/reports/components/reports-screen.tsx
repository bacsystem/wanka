"use client";

import { ArrowLeftRight, Calculator, Calendar, ClipboardList, Clock, Download, FileCode, FileSpreadsheet, RefreshCw, ShieldCheck, Warehouse } from "lucide-react";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable, type Column } from "@/components/shared/data-table";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { SalesRegisterRow } from "../mocks/reports";
import { FilterBar } from "@/components/shared/filter-bar";

const typeIcon = { rvie: ClipboardList, kardex: Warehouse, f621: Calculator, export: ArrowLeftRight } as const;
const sireMeta: Record<SalesRegisterRow["sire"], { label: string; tone: BadgeTone }> = {
  coincidente: { label: "Coincidente", tone: "success" },
  observado: { label: "Observado", tone: "warning" },
  faltante: { label: "Faltante en SIRE", tone: "danger" },
};
const periods = [{ value: "2026-09", label: "Setiembre 2026 (2026-09)" }, { value: "2026-08", label: "Agosto 2026 (2026-08)" }, { value: "2026-07", label: "Julio 2026 (2026-07)" }];
const ops = [{ value: "all", label: "Todas las operaciones" }, { value: "gravadas", label: "Solo gravadas IGV (18%)" }, { value: "exo", label: "Exoneradas / inafectas" }];
const docs = [{ value: "all", label: "Todos (01, 03, 07)" }, { value: "01", label: "Facturas (01)" }, { value: "03", label: "Boletas (03)" }, { value: "07", label: "Notas de crédito (07)" }];

interface Props {
  types: { id: string; label: string; count: string }[];
  rows: SalesRegisterRow[];
  schedule: { label: string; date: string }[];
}

export function ReportsScreen({ types, rows: all, schedule }: Props) {
  const router = useRouter();
  const [active, setActive] = React.useState("rvie");
  function pickType(id: string) { if (id === "f621" || id === "kardex") router.push("/reportes/liquidacion"); else setActive(id); }
  const [docType, setDocType] = React.useState("all");
  const rows = all.filter((r) => docType === "all" || r.type === docType);
  const totals = rows.reduce((s, r) => ({ base: s.base + r.base, igv: s.igv + r.igv, exempt: s.exempt + r.exempt, total: s.total + r.total }), { base: 0, igv: 0, exempt: 0, total: 0 });

  const columns: Column<SalesRegisterRow>[] = [
    { key: "cuo", header: "N° CUO", cell: (r) => <span className="font-mono text-xs tabular-nums">{r.cuo}</span> },
    { key: "date", header: "Fecha emis.", cell: (r) => <span className="font-mono tabular-nums">{formatDate(r.issuedAt)}</span> },
    { key: "type", header: "Tipo", cell: (r) => <span className={cn("rounded px-1.5 py-0.5 font-mono text-[11px] font-semibold", r.type === "07" ? "bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300" : "bg-blue-50 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300")}>{r.type}</span> },
    { key: "number", header: "Serie - N°", cell: (r) => <span className={cn("font-mono text-xs font-bold tabular-nums", r.total < 0 && "text-destructive")}>{r.number}</span> },
    { key: "doc", header: "Doc. identidad", cell: (r) => <span className="text-xs"><span className="mr-1 text-[10px] text-muted-foreground uppercase">{r.docType}</span><span className="font-mono">{r.docNumber}</span></span> },
    { key: "customer", header: "Razón social / cliente", cell: (r) => <span className="font-medium">{r.customer}</span>, className: "max-w-56 truncate" },
    { key: "base", header: "Base gravada", align: "right", cell: (r) => <span className={cn("font-mono tabular-nums", r.total < 0 && "text-destructive")}>{formatCurrency(r.base)}</span> },
    { key: "igv", header: "IGV (18%)", align: "right", cell: (r) => <span className="font-mono tabular-nums">{formatCurrency(r.igv)}</span> },
    { key: "exempt", header: "Exo. / inaf.", align: "right", cell: (r) => <span className="font-mono tabular-nums">{formatCurrency(r.exempt)}</span> },
    { key: "total", header: "Importe total", align: "right", cell: (r) => <span className={cn("font-mono font-medium tabular-nums", r.total < 0 && "text-destructive")}>{formatCurrency(r.total)}</span> },
    { key: "sire", header: "Estado SIRE", cell: (r) => <StatusBadge tone={sireMeta[r.sire].tone} label={sireMeta[r.sire].label} /> },
    { key: "xml", header: "XML", align: "right", cell: () => <Button variant="ghost" size="icon-sm" aria-label="Descargar XML"><FileCode /></Button> },
  ];

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <FilterBar className="justify-between p-1.5">
        <nav className="no-scrollbar flex max-w-full gap-1 overflow-x-auto" aria-label="Reportes disponibles">
          {types.map((t) => {
            const Icon = typeIcon[t.id as keyof typeof typeIcon] ?? ClipboardList;
            const isActive = active === t.id;
            return (
              <button key={t.id} type="button" onClick={() => pickType(t.id)} aria-pressed={isActive} className={cn("flex shrink-0 items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold transition-colors", isActive ? "bg-accent text-primary shadow-xs dark:bg-muted" : "text-foreground/80 hover:bg-muted")}>
                <Icon className="size-4" strokeWidth={2} /> {t.label}
                <span className={cn("rounded-full px-2 py-0.5 font-mono text-[10px]", isActive ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300" : "bg-muted text-muted-foreground")}>{t.count}</span>
              </button>
            );
          })}
        </nav>
        <span className="hidden items-center gap-1.5 pr-3 text-xs text-muted-foreground lg:flex"><span className="size-1.5 rounded-full bg-primary" /> SIRE API en línea: v2.4.1</span>
      </FilterBar>

      <SectionCard
        title={types.find((t) => t.id === active)?.label ?? ""}
        action={
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => toast.success("Reconciliado con SIRE · 1 observación")}><RefreshCw data-icon="inline-start" /> Re-conciliar</Button>
            <Button variant="outline" size="sm"><FileSpreadsheet data-icon="inline-start" /> Excel RVIE</Button>
            <Button size="sm" onClick={() => toast.success("Archivo SIRE (TXT/ZIP) generado")}><ShieldCheck data-icon="inline-start" /> Generar archivo SIRE</Button>
          </div>
        }
        contentClassName="p-0"
      >
        <div className="flex flex-col gap-2 border-b p-3">
          <div className="flex flex-wrap items-center gap-2">
            <Select defaultValue="2026-09" items={periods}><SelectTrigger className="w-56 bg-muted/50" aria-label="Período tributario"><Calendar className="size-4 text-muted-foreground" /><SelectValue /></SelectTrigger><SelectContent>{periods.map((p) => <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>)}</SelectContent></Select>
            <Select defaultValue="all" items={ops}><SelectTrigger className="w-64 bg-muted/50" aria-label="Tipo de operación"><SelectValue /></SelectTrigger><SelectContent>{ops.map((p) => <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>)}</SelectContent></Select>
            <Select value={docType} onValueChange={(v) => setDocType(String(v))} items={docs}><SelectTrigger className="w-56 bg-muted/50" aria-label="Tipo de comprobante"><SelectValue /></SelectTrigger><SelectContent>{docs.map((p) => <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>)}</SelectContent></Select>
          </div>
          <SearchInput placeholder="Buscar por RUC/DNI, cliente o N° serie…" className="max-w-none" />
        </div>
        <DataTable
          columns={columns} rows={rows} rowKey={(r) => r.cuo} minWidth="1240px"
          mobileCard={(r) => (
            <div className="flex items-start gap-3">
              <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{r.customer}</p><p className="font-mono text-xs text-muted-foreground tabular-nums">{r.number} · {formatDate(r.issuedAt)}</p><StatusBadge className="mt-1" tone={sireMeta[r.sire].tone} label={sireMeta[r.sire].label} /></div>
              <div className="text-right"><p className="font-mono text-sm font-medium tabular-nums">{formatCurrency(r.total)}</p><p className="font-mono text-xs text-muted-foreground">IGV {formatCurrency(r.igv)}</p></div>
            </div>
          )}
        />
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-t bg-accent/40 px-4 py-3 text-xs dark:bg-muted/40">
          <span className="font-bold uppercase">Totales acumulados setiembre 2026 ({rows.length} comprobantes):</span>
          <span className="flex flex-wrap items-center gap-x-6 gap-y-1">
            <span className="font-mono font-bold text-primary">{formatCurrency(totals.base)}</span>
            <span className="font-mono font-bold text-primary">{formatCurrency(totals.igv)}</span>
            <span className="font-mono text-muted-foreground">{formatCurrency(totals.exempt)}</span>
            <span className="rounded-md bg-teal-100 px-2 py-1 font-mono text-sm font-bold text-teal-900 dark:bg-teal-900/50 dark:text-teal-100">{formatCurrency(totals.total)}</span>
            <span className="font-semibold text-primary">100% conciliado SIRE</span>
          </span>
        </div>
        <PaginationBar from={1} to={rows.length} total={148} label="registros tributarios" />
      </SectionCard>

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Cronograma SUNAT" icon={Calendar} action={<StatusBadge tone="info" label="Dígito 5" className="rounded-md" />} contentClassName="flex flex-col gap-3 p-4 text-xs">
          <div className="flex items-center justify-between rounded-lg bg-muted/50 p-3"><div><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Fecha límite improrrogable</p><p className="text-base font-bold">{formatDate(schedule[0].date)}</p></div><StatusBadge tone="info" icon={Clock} label="21 días restantes" className="rounded-md" /></div>
          <ul className="divide-y">{schedule.slice(1).map((sc) => <li key={sc.label} className="flex items-center justify-between py-2"><span className="text-muted-foreground">{sc.label}</span><span className="font-mono font-semibold">{formatDate(sc.date)}</span></li>)}</ul>
          <div className="flex items-center justify-between border-t pt-3"><span className="text-muted-foreground">Resolución de Superintendencia N° 000271-2023</span><button type="button" className="font-semibold text-primary hover:underline">Ver cronograma completo ↗</button></div>
        </SectionCard>
        <SectionCard title="Conciliación SIRE" icon={ShieldCheck} iconTone="success" action={<StatusBadge tone="success" label="100% conciliado" className="rounded-md" />} contentClassName="flex flex-col gap-3 p-4 text-xs">
          <div className="flex items-center justify-between"><span className="text-muted-foreground">Comprobantes cruzados:</span><span className="font-bold">148 de 148 (100%)</span></div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-muted"><div className="h-full w-full rounded-full bg-primary" /></div>
          <div className="flex items-center justify-between"><span className="text-muted-foreground">Discrepancias detectadas: <strong className="font-semibold text-primary">0 notas / 0 boletas</strong></span><span className="text-muted-foreground">Ticket SUNAT: <strong className="font-mono font-semibold text-foreground">20260912-09142</strong></span></div>
          <div className="flex items-center justify-between border-t pt-3"><span className="text-muted-foreground">Última conciliación automática: <strong className="font-semibold text-foreground">Hoy a las 11:30 AM</strong></span><button type="button" className="font-semibold text-primary hover:underline" onClick={() => toast.success("Reconciliado con SIRE · sin observaciones")}>Re-conciliar ahora</button></div>
        </SectionCard>
        <SectionCard title="Exportaciones contables" icon={FileSpreadsheet} action={<span className="text-xs text-muted-foreground">Concar / Siscont</span>} contentClassName="flex flex-col gap-3 p-4">
          <div className="grid grid-cols-2 gap-2">
            {[["Formato 14.1 SIRE", "TXT comprimido ZIP"], ["Estructura PLE SUNAT", "LE20608941235…"], ["Interfaz Siscont", "Ventas.DBF / Excel"], ["Asientos Concar", "Comprob. contable"]].map(([a, b]) => (
              <button key={a} type="button" onClick={() => toast.info(`Generando ${a}…`)} className="flex items-center gap-2 rounded-lg bg-muted/50 px-3 py-2.5 text-left text-xs hover:bg-muted">
                <span className="min-w-0 flex-1"><span className="block truncate font-semibold">{a}</span><span className="block truncate text-muted-foreground">{b}</span></span>
                <Download className="size-4 shrink-0 text-primary" />
              </button>
            ))}
          </div>
          <div className="flex items-center justify-between border-t pt-3 text-xs"><span className="text-muted-foreground">Codificación: <strong className="font-semibold text-foreground">UTF-8 sin BOM</strong></span><span className="font-semibold text-primary">Validado con Validador SUNAT v2.1</span></div>
        </SectionCard>
      </div>
    </div>
  );
}
