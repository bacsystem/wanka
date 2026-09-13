"use client";

import { ArrowRight, Building2, CheckCircle2, ClipboardCheck, Clock, Download, FileCode, FilterX, GitBranch, PackageCheck, Printer, QrCode, RefreshCw, Route, ScanLine, Send, Truck, XCircle } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { cn } from "@/lib/utils";
import { formatDate } from "@/lib/format";
import type { DispatchGuide, GuideReason, GuideStatus } from "../mocks/guides";

export const reasonLabel: Record<GuideReason, string> = { venta: "Venta sujeta a entrega", traslado_sedes: "Traslado entre sedes", devolucion: "Devolución / rechazo" };
const reasonTone: Record<GuideReason, BadgeTone> = { venta: "info", traslado_sedes: "success", devolucion: "danger" };
const statusMeta: Record<GuideStatus, { label: string; tone: BadgeTone; icon: typeof Clock }> = {
  aceptado: { label: "Aceptado CDR", tone: "success", icon: CheckCircle2 },
  pendiente: { label: "Pendiente", tone: "warning", icon: Clock },
  rechazado: { label: "Rechazado", tone: "danger", icon: XCircle },
  en_transito: { label: "En tránsito", tone: "info", icon: Route },
  entregado: { label: "Entregado", tone: "neutral", icon: CheckCircle2 },
};

const reasonOptions = [{ value: "all", label: "Motivo: todos" }, ...Object.entries(reasonLabel).map(([value, label]) => ({ value, label }))];
const statusOptions = [{ value: "all", label: "Estado: todos" }, ...Object.entries(statusMeta).map(([value, m]) => ({ value, label: m.label }))];

export function GuidesTable({ guides, openNew, onOpenNewChange }: { guides: DispatchGuide[]; openNew: boolean; onOpenNewChange: (o: boolean) => void }) {
  const [query, setQuery] = React.useState("");
  const [reason, setReason] = React.useState("all");
  const [status, setStatus] = React.useState("all");
  const [selectedId, setSelectedId] = React.useState(guides[0]?.id);

  const rows = guides.filter((g) => {
    const s = query.trim().toLowerCase();
    return (reason === "all" || g.reason === reason) && (status === "all" || g.status === status) &&
      (!s || g.number.toLowerCase().includes(s) || g.plate.toLowerCase().includes(s) || g.destination.toLowerCase().includes(s) || g.carrier.toLowerCase().includes(s));
  });

  const columns: Column<DispatchGuide>[] = [
    { key: "number", header: "Serie y número", cell: (g) => <span className="flex items-center gap-2"><span className={cn("size-2 shrink-0 rounded-full", g.status === "en_transito" ? "bg-primary" : g.status === "aceptado" ? "bg-emerald-500" : g.status === "rechazado" ? "bg-rose-500" : "bg-slate-300")} /><TwoLine primary={<Link href={`/ventas/guias/${g.number}`} className="font-mono text-sm font-bold tabular-nums hover:text-primary hover:underline">{g.number}</Link>} secondary="GRE remitente" /></span> },
    { key: "dates", header: "Emisión / traslado", cell: (g) => <TwoLine primary={formatDate(g.issuedAt)} secondary={`Inicio ${formatDate(g.transferAt)}`} mono />, className: "whitespace-nowrap" },
    { key: "reason", header: "Motivo", cell: (g) => <StatusBadge tone={reasonTone[g.reason]} label={reasonLabel[g.reason]} /> },
    { key: "modality", header: "Modalidad", cell: (g) => <span className="flex items-center gap-1.5">{g.modality === "privado" ? <Building2 className="size-4 text-primary" /> : <Truck className="size-4 text-primary" />}<TwoLine primary={g.modality === "privado" ? "Privado (interno)" : "Público"} secondary={g.modality === "publico" ? g.carrierDoc : undefined} /></span> },
    { key: "route", header: "Ruta (partida → llegada)", cell: (g) => <TwoLine primary={g.origin} secondary={<span className="inline-flex items-center gap-1"><ArrowRight className="size-3" /> {g.destination}</span>} />, className: "max-w-72" },
    { key: "carrier", header: "Transporte y placa", cell: (g) => <TwoLine primary={<span className="font-mono font-bold tabular-nums">{g.plate}</span>} secondary={g.modality === "privado" ? `${g.carrierDoc} • ${g.carrier}` : g.carrier} /> },
    { key: "weight", header: "Peso (kg)", align: "right", cell: (g) => <span className="font-mono font-semibold tabular-nums">{g.weightKg.toFixed(2)}</span> },
    { key: "status", header: "Estado SUNAT", cell: (g) => { const m = statusMeta[g.status]; return <StatusBadge tone={m.tone} label={m.label} icon={m.icon} />; } },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (g) => (
        <div className="flex justify-end gap-0.5">
          <Button variant="ghost" size="icon-sm" className="text-primary" aria-label="Ver QR" onClick={(e) => { e.stopPropagation(); toast.info(`QR de ${g.number}`); }}><QrCode /></Button>
          <Button variant="ghost" size="icon-sm" className="text-primary" aria-label="Descargar XML" onClick={(e) => e.stopPropagation()}><FileCode /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Imprimir" onClick={(e) => e.stopPropagation()}><Printer /></Button>
        </div>
      ),
    },
  ];

  const selected = rows.find((g) => g.id === selectedId) ?? rows[0];
  return (
    <>
      <SectionCard title={<span className="sr-only">Registro de guías</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <div className="flex flex-wrap items-center gap-2 p-3">
          <SearchInput placeholder="Buscar por serie, RUC, conductor…" value={query} onChange={setQuery} className="sm:max-w-56" />
          <Select defaultValue="month" items={[{ value: "month", label: "Rango: Este mes (Set 2026)" }, { value: "prev", label: "Rango: Mes anterior" }]}>
            <SelectTrigger className="w-56 bg-muted/50" aria-label="Rango"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="month">Rango: Este mes (Set 2026)</SelectItem><SelectItem value="prev">Rango: Mes anterior</SelectItem></SelectContent>
          </Select>
          <Select value={reason} onValueChange={(v) => setReason(String(v))} items={reasonOptions}>
            <SelectTrigger className="w-52 bg-muted/50" aria-label="Motivo"><SelectValue /></SelectTrigger>
            <SelectContent>{reasonOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <Select defaultValue="all" items={[{ value: "all", label: "Partida: todas las sedes" }, { value: "mira", label: "Partida: Miraflores" }, { value: "surq", label: "Partida: Surquillo" }]}>
            <SelectTrigger className="w-52 bg-muted/50" aria-label="Partida"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="all">Partida: todas las sedes</SelectItem><SelectItem value="mira">Partida: Miraflores</SelectItem><SelectItem value="surq">Partida: Surquillo</SelectItem></SelectContent>
          </Select>
          <Select value={status} onValueChange={(v) => setStatus(String(v))} items={statusOptions}>
            <SelectTrigger className="w-48 bg-muted/50" aria-label="Estado SUNAT"><SelectValue /></SelectTrigger>
            <SelectContent>{statusOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <div className="ml-auto flex gap-1"><Button variant="secondary" size="icon-sm" aria-label="Limpiar filtros" onClick={() => { setQuery(""); setReason("all"); setStatus("all"); }}><FilterX /></Button><Button variant="secondary" size="icon-sm" aria-label="Actualizar" onClick={() => toast.success("Estados SUNAT actualizados")}><RefreshCw /></Button></div>
        </div>
        <DataTable
          columns={columns} rows={rows} rowKey={(g) => g.id} minWidth="1100px" onRowClick={(g) => setSelectedId(g.id)} rowClassName={(g) => cn("cursor-pointer", selected?.id === g.id && "bg-accent/30")}
          mobileCard={(g) => {
            const m = statusMeta[g.status];
            return (
              <Link href={`/ventas/guias/${g.number}`} className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs font-medium text-primary tabular-nums">{g.number}</span>
                  <StatusBadge tone={m.tone} label={m.label} icon={m.icon} />
                </div>
                <p className="text-sm">{reasonLabel[g.reason]} · <span className="font-mono">{g.plate}</span></p>
                <p className="truncate text-xs text-muted-foreground">{g.origin} → {g.destination}</p>
                <p className="text-xs text-muted-foreground">{formatDate(g.issuedAt)} · {g.weightKg.toFixed(2)} kg</p>
              </Link>
            );
          }}
        />
        <PaginationBar from={1} to={rows.length} total={48} label="guías" />
      </SectionCard>
      {selected ? <GuideTraceability guide={selected} /> : null}
      <NewGuideSheet open={openNew} onOpenChange={onOpenNewChange} />
    </>
  );
}

/** Stitch: inline traceability panel for the selected guide. */
function GuideTraceability({ guide: g }: { guide: DispatchGuide }) {
  const steps = [
    { title: "Emisión y autorización SUNAT", text: "GRE validada contra OSE. Hash fiscal generado y código QR activo para fiscalización en vía pública.", at: `${formatDate(g.issuedAt)} 08:30:14`, done: true },
    { title: `Despacho • ${g.origin.split(",")[0]}`, text: `Mercadería cargada en vehículo placa ${g.plate}. Conductor: ${g.carrier} (${g.carrierDoc}). Peso registrado: ${g.weightKg.toFixed(2)} kg (4 bultos sellados).`, at: `${formatDate(g.transferAt)} 09:15:00`, done: true },
    { title: `En tránsito • ruta ${g.destination.split(",")[0]}`, text: "Monitoreo GPS activo. Sin desvíos reportados.", at: `${formatDate(g.transferAt)} 09:40:22 (estimado llegada: 10:10)`, done: g.status !== "en_transito", current: g.status === "en_transito" },
    { title: `Recepción • ${g.destination.split(",")[0]}`, text: "Requiere conteo físico, verificación de precintos y firma digital del jefe de almacén receptor.", at: g.status === "entregado" ? `${formatDate(g.transferAt)} 10:08:51` : "Pendiente", done: g.status === "entregado" },
  ];
  const items = [["Resina compuesta dental 3M A2", "SKU: RES-3M-A2 • Lote: L2025-04", "20 und"], ["Anestesia dental tubos 2%", "SKU: ANE-200 • Lote: L2024-11", "5 cajas"], ["Baberos desechables impermeables", "SKU: BAB-100 • Lote: L2025-02", "2 packs"]];
  return (
    <SectionCard icon={GitBranch} title={<span className="flex flex-wrap items-center gap-2">Detalle de trazabilidad GRE • {g.number} <StatusBadge tone="success" label="SUNAT CDR: Constancia 20019238" /></span>} description={"Rastreo físico de despacho, verificación de pesos y conformidad de recepción en destino"} action={<><Button variant="secondary" className="font-semibold" render={<Link href={`/inventario/recepcion/${g.number}`} />} nativeButton={false}><ScanLine data-icon="inline-start" /> Escanear recepción</Button><Button className="font-semibold" onClick={() => toast.success(`Ingreso de ${g.number} confirmado en almacén`)}><PackageCheck data-icon="inline-start" /> Confirmar ingreso a almacén</Button></>} contentClassName="p-4">
      <div className="grid gap-4 lg:grid-cols-3">
        <ol className="flex flex-col gap-3 lg:col-span-2">
          {steps.map((st, i) => (
            <li key={i} className="flex gap-3">
              <span className={cn("mt-2 flex size-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold", st.done ? "bg-primary text-primary-foreground" : st.current ? "bg-blue-500 text-white" : "border bg-card text-muted-foreground")}>{st.done ? <CheckCircle2 className="size-3.5" /> : st.current ? "A" : <ClipboardCheck className="size-3" />}</span>
              <div className={cn("flex-1 rounded-lg p-3", st.current ? "bg-blue-50 dark:bg-blue-950/30" : "bg-muted/50")}>
                <div className="flex flex-wrap items-center justify-between gap-2"><p className={cn("text-sm font-bold", st.current && "text-blue-700 dark:text-blue-300")}>{st.title}</p><span className={cn("text-xs", st.current ? "font-semibold text-blue-700 dark:text-blue-300" : "text-muted-foreground")}>{st.at}</span></div>
                <p className="mt-0.5 text-xs text-muted-foreground">{st.text}</p>
              </div>
            </li>
          ))}
        </ol>
        <div className="rounded-lg bg-muted/50 p-3">
          <p className="text-sm font-bold">Resumen de ítems en guía</p>
          <ul className="mt-2 flex flex-col gap-2">{items.map(([n, d, q]) => <li key={n} className="flex items-center justify-between gap-2 rounded-md bg-card p-2.5 text-xs shadow-xs"><span><span className="block font-semibold">{n}</span><span className="text-muted-foreground">{d}</span></span><span className="shrink-0 font-mono text-sm font-bold text-primary">{q}</span></li>)}</ul>
          <div className="mt-3 flex items-center justify-between border-t pt-3 text-xs"><span><span className="block text-muted-foreground">Total bultos</span><span className="text-sm font-bold">4 bultos</span></span><span className="text-right"><span className="block text-muted-foreground">Peso bruto total</span><span className="font-mono text-sm font-bold text-primary">{g.weightKg.toFixed(2)} KG</span></span></div>
        </div>
      </div>
    </SectionCard>
  );
}

function NewGuideSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Emitir guía de remisión electrónica – remitente</SheetTitle>
          <SheetDescription>Serie T001 · correlativo 000146 · se firma y transmite a SUNAT con código QR.</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Motivo de traslado" className="sm:col-span-2">
            <Select defaultValue="traslado_sedes" items={Object.entries(reasonLabel).map(([value, label]) => ({ value, label }))}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>{Object.entries(reasonLabel).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Modalidad de transporte">
            <Select defaultValue="privado" items={[{ value: "privado", label: "Privado (vehículo propio)" }, { value: "publico", label: "Público (transportista)" }]}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="privado">Privado (vehículo propio)</SelectItem><SelectItem value="publico">Público (transportista)</SelectItem></SelectContent>
            </Select>
          </Field>
          <Field label="Fecha de inicio de traslado"><Input type="date" defaultValue="2026-09-12" /></Field>
          <Field label="Punto de partida (origen)"><Input defaultValue="Av. Larco 743, Miraflores" /></Field>
          <Field label="Punto de llegada (destino)"><Input placeholder="Dirección completa" /></Field>
          <Field label="DNI / brevete del conductor"><Input placeholder="Q45892101" className="font-mono" /></Field>
          <Field label="Placa del vehículo"><Input placeholder="BXY-842" className="font-mono uppercase" /></Field>
          <Field label="Peso total (KGM)"><Input type="number" inputMode="decimal" placeholder="0.00" className="font-mono" /></Field>
          <Field label="Número de bultos"><Input type="number" placeholder="1" className="font-mono" /></Field>
          <Field label="Observaciones" className="sm:col-span-2"><Textarea rows={3} placeholder="Descripción de los bienes trasladados…" /></Field>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={() => { toast.success("GRE T001-000146 firmada y transmitida a SUNAT"); onOpenChange(false); }}>
            <Send data-icon="inline-start" /> Firmar y transmitir
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export function GuidesScreen({ guides, kpis }: { guides: DispatchGuide[]; kpis: React.ReactNode }) {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <PageHeader
        eyebrow="Logística y almacén · Guías de remisión SUNAT"
        title="Guías de remisión electrónica (GRE)"
        status={<StatusBadge tone="info" dot label="SUNAT API en línea" />}
        actions={
          <>
            <Button variant="outline" onClick={() => toast.info("Selecciona una venta para importar sus ítems")}><Truck data-icon="inline-start" /> Importar de venta</Button>
            <Button variant="outline" onClick={() => toast.success("Registro exportado")}><Download data-icon="inline-start" /> Exportar registro</Button>
            <Button onClick={() => setOpen(true)}><Send data-icon="inline-start" /> Emitir nueva GRE remitente</Button>
          </>
        }
      />
      {kpis}
      <GuidesTable guides={guides} openNew={open} onOpenNewChange={setOpen} />
    </>
  );
}
