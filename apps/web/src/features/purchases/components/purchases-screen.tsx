"use client";

import { AlertTriangle, CloudDownload, Eye, FileArchive, History, Landmark, MoreHorizontal, Phone, RefreshCw, Send, SlidersHorizontal, Store, Upload, UserPlus, Wand2 } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { PurchasesTabs } from "./purchases-tabs";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PayStatus, Purchase, SireStatus } from "../mocks/purchases";
import { FilterBar } from "@/components/shared/filter-bar";

const typeLabel = { "01": "Factura", "03": "Boleta", "07": "N. crédito", "14": "Serv. públicos" } as const;
const sireMeta: Record<SireStatus, { label: string; tone: BadgeTone }> = {
  aceptado: { label: "Aceptado en propuesta", tone: "success" },
  discrepancia: { label: "Con discrepancia", tone: "warning" },
  excluido: { label: "Excluido", tone: "neutral" },
};
const payMeta: Record<PayStatus, { label: string; tone: BadgeTone }> = {
  pagado: { label: "Pagado", tone: "success" },
  pendiente: { label: "Pendiente", tone: "neutral" },
  por_vencer: { label: "Por vencer", tone: "warning" },
};
const periods = [{ value: "2026-09", label: "Setiembre 2026 (2026-09)" }, { value: "2026-08", label: "Agosto 2026 (2026-08)" }, { value: "2026-07", label: "Julio 2026 (2026-07)" }];
const sireOptions = [{ value: "all", label: "Estado SIRE: todos" }, ...Object.entries(sireMeta).map(([value, m]) => ({ value, label: m.label }))];

export function PurchasesScreen({ purchases, discrepancy, tabCounts, kpis }: { purchases: Purchase[]; discrepancy: { doc: string; supplier: string; detail: string }; tabCounts: { registro: number; proveedores: number; ordenes: number }; kpis?: React.ReactNode }) {
  const [query, setQuery] = React.useState("");
  const [sire, setSire] = React.useState("all");
  const [uploadOpen, setUploadOpen] = React.useState(false);

  const rows = purchases.filter((p) => {
    const s = query.trim().toLowerCase();
    return (sire === "all" || p.sire === sire) && (!s || p.supplier.toLowerCase().includes(s) || p.supplierRuc.includes(s) || p.number.toLowerCase().includes(s));
  });

  const columns: Column<Purchase>[] = [
    { key: "cuo", header: "CUO", cell: (p) => <span className="font-mono text-xs tabular-nums">{p.cuo}</span> },
    { key: "dates", header: "F. emisión / venc.", cell: (p) => <TwoLine primary={formatDate(p.issuedAt)} secondary={p.dueAt ? `Vence ${formatDate(p.dueAt)}` : undefined} mono />, className: "whitespace-nowrap" },
    { key: "type", header: "Tipo", cell: (p) => <span className="flex items-center gap-1.5"><Tag label={p.type} className="font-mono" /><span className="text-xs">{typeLabel[p.type]}</span></span> },
    { key: "number", header: "Serie-N°", cell: (p) => <span className="font-mono text-xs tabular-nums">{p.number}</span> },
    { key: "supplier", header: "Proveedor / RUC", cell: (p) => <TwoLine primary={<span className="font-medium">{p.supplier}</span>} secondary={p.supplierRuc} mono />, className: "max-w-64" },
    { key: "base", header: "Base imponible", align: "right", cell: (p) => <span className={cn("font-mono tabular-nums", p.total < 0 && "text-destructive")}>{formatCurrency(p.base)}</span> },
    { key: "igv", header: "IGV (18%)", align: "right", cell: (p) => <span className="font-mono tabular-nums">{formatCurrency(p.igv)}</span> },
    { key: "total", header: "Total", align: "right", cell: (p) => <span className={cn("font-mono font-medium tabular-nums", p.total < 0 && "text-destructive")}>{formatCurrency(p.total)}</span> },
    { key: "spot", header: "Detracción / SPOT", cell: (p) => p.detraction ? <TwoLine primary={<span className="font-mono tabular-nums">{p.detraction.rate}% · {formatCurrency(p.detraction.amount)}</span>} secondary={p.detraction.paid ? "Constancia OK" : "Sin depositar"} /> : <span className="text-muted-foreground">—</span> },
    { key: "pay", header: "Pago", cell: (p) => <StatusBadge tone={payMeta[p.payStatus].tone} dot label={payMeta[p.payStatus].label} /> },
    { key: "sire", header: "Estado SIRE", cell: (p) => <StatusBadge tone={sireMeta[p.sire].tone} label={sireMeta[p.sire].label} /> },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (p) => (
        <div className="flex justify-end gap-0.5">
          <Button variant="ghost" size="icon-sm" aria-label="Ver detalle"><Eye /></Button>
          {p.sire === "discrepancia" ? <Button variant="ghost" size="icon-sm" aria-label="Ajustar a propuesta SIRE" onClick={() => toast.success(`${p.number} ajustado a la propuesta SIRE`)}><Wand2 /></Button> : null}
          <Button variant="ghost" size="icon-sm" aria-label="Más"><MoreHorizontal /></Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Compras y proveedores · Registro de compras electrónico (RCE - SIRE SUNAT)"
        title="Gestión de compras y SIRE"
        status={<StatusBadge tone="info" dot label="SUNAT token activo" />}
        actions={
          <>
            <Button variant="outline" className="font-semibold" render={<Link href="/compras/proveedores" />} nativeButton={false}><UserPlus data-icon="inline-start" /> Nuevo proveedor</Button>
            <Button variant="outline" className="font-semibold" onClick={() => toast.info("Descargando propuesta SIRE 2026-09…")}><CloudDownload data-icon="inline-start" /> Importar propuesta SIRE</Button>
            <Button variant="outline" className="font-semibold"><FileArchive data-icon="inline-start" /> Estructura RCE (ZIP)</Button>
            <Button className="font-semibold" onClick={() => setUploadOpen(true)}><Upload data-icon="inline-start" /> Registrar compra / XML</Button>
          </>
        }
      />
      <PurchasesTabs tab="registro" counts={tabCounts} />
      {kpis}

      <FilterBar className="gap-3">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-accent text-primary"><AlertTriangle className="size-4" /></span>
        <div className="min-w-0 flex-1 text-sm">
          <p className="flex flex-wrap items-center gap-2 font-bold">Discrepancia en propuesta RCE de SUNAT <Tag tone="info" label={`Comprobante ${discrepancy.doc}`} /></p>
          <p className="text-xs text-muted-foreground"><strong className="font-semibold text-foreground">{discrepancy.supplier}</strong> {discrepancy.detail}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" className="font-semibold">Excluir de propuesta</Button>
          <Button className="font-semibold" onClick={() => toast.success("Comprobante ajustado a la propuesta SIRE")}><Wand2 data-icon="inline-start" /> Ajustar a propuesta SIRE</Button>
        </div>
      </FilterBar>

      <FilterBar className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[1fr_1.3fr_1fr_1fr_auto]">
        <Field label="Período tributario"><Select defaultValue="2026-09" items={periods}><SelectTrigger className="w-full bg-muted/50" aria-label="Período tributario"><SelectValue /></SelectTrigger><SelectContent>{periods.map((p) => <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>)}</SelectContent></Select></Field>
        <Field label="Buscar proveedor / RUC"><SearchInput placeholder="Ej. Dental Corp o 2060…" value={query} onChange={setQuery} className="max-w-none" /></Field>
        <Field label="Tipo comprobante"><Select defaultValue="all" items={[{ value: "all", label: "Todos los tipos" }, { value: "01", label: "01 Factura" }, { value: "07", label: "07 Nota de crédito" }]}><SelectTrigger className="w-full bg-muted/50" aria-label="Tipo comprobante"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Todos los tipos</SelectItem><SelectItem value="01">01 Factura</SelectItem><SelectItem value="07">07 Nota de crédito</SelectItem></SelectContent></Select></Field>
        <Field label="Estado SIRE SUNAT"><Select value={sire} onValueChange={(v) => setSire(String(v))} items={sireOptions}><SelectTrigger className="w-full bg-muted/50" aria-label="Estado SIRE"><SelectValue /></SelectTrigger><SelectContent>{sireOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select></Field>
        <div className="flex items-end gap-2"><Button variant="secondary" className="font-semibold" onClick={() => { setQuery(""); setSire("all"); }}><RefreshCw data-icon="inline-start" /> Restablecer</Button><Button variant="secondary" className="bg-teal-100 font-semibold text-teal-900 hover:bg-teal-200 dark:bg-teal-900/50 dark:text-teal-100"><SlidersHorizontal data-icon="inline-start" /> Filtros (+)</Button></div>
      </FilterBar>

      <SectionCard
        title={<span className="flex items-center gap-2">Libro de compras electrónico <span className="text-xs font-normal text-muted-foreground">Generador RCE 2026-09-12 17:30</span></span>}
        action={<div className="flex flex-wrap items-center gap-3"><span className="flex items-center gap-1 text-xs font-medium"><span className="size-1.5 rounded-full bg-primary" /> 41 Aceptados</span><span className="flex items-center gap-1 text-xs font-medium text-destructive"><span className="size-1.5 rounded-full bg-rose-500" /> 1 Observado</span><Button size="sm" className="font-semibold" onClick={() => toast.success("RCE 2026-09 generado y enviado a SUNAT (SIRE)")}><Send data-icon="inline-start" /> Generar y enviar a SUNAT</Button></div>}
        contentClassName="p-0"
      >
        <DataTable
          columns={columns} rows={rows} rowKey={(p) => p.id} minWidth="1280px"
          mobileCard={(p) => (
            <div className="flex items-start gap-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{p.supplier}</p>
                <p className="font-mono text-xs text-muted-foreground tabular-nums">{p.number} · {formatDate(p.issuedAt)}</p>
                <div className="mt-1 flex gap-1"><StatusBadge tone={sireMeta[p.sire].tone} label={sireMeta[p.sire].label} /><StatusBadge tone={payMeta[p.payStatus].tone} dot label={payMeta[p.payStatus].label} /></div>
              </div>
              <span className={cn("font-mono text-sm font-medium tabular-nums", p.total < 0 && "text-destructive")}>{formatCurrency(p.total)}</span>
            </div>
          )}
        />
        <PaginationBar from={1} to={rows.length} total={42} label="comprobantes" />
      </SectionCard>

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="RCE Anexo 8 - Estructura SUNAT" icon={FileArchive} action={<StatusBadge tone="info" label="Versión 2.0" className="rounded-md" />} contentClassName="flex flex-col gap-3 p-4 text-xs">
          <p className="text-muted-foreground">Verificación de campos obligatorios: Código de Anotación (CAR), tipo de cambio de la fecha de emisión y régimen especial de percepciones.</p>
          <div className="flex items-center justify-between gap-2 rounded-md bg-muted/50 px-3 py-2"><span className="text-muted-foreground">Archivos generados:</span><span className="truncate font-mono font-semibold">LE20549812451202411000804000021112.txt</span></div>
          <div className="flex items-center justify-between gap-2 rounded-md bg-muted/50 px-3 py-2"><span className="text-muted-foreground">Hash ticket SUNAT:</span><span className="font-mono font-semibold text-primary">TK-20260912-9841-B</span></div>
          <Button className="font-semibold" onClick={() => toast.success("RCE 2026-09 generado y enviado a SUNAT (SIRE)")}><Send data-icon="inline-start" /> Generar y enviar a SUNAT</Button>
        </SectionCard>
        <SectionCard title="Proveedor clave del mes" icon={Store} action={<span className="text-xs font-semibold text-primary">Top #1 volumen</span>} contentClassName="flex flex-col gap-3 p-4 text-xs">
          <div className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-lg bg-accent text-sm font-bold text-primary">DC</span><div><p className="text-sm font-bold">Dental Corp Perú S.A.C.</p><p className="text-muted-foreground">RUC: 20549812451 • Condición: Habido</p></div></div>
          <div className="grid grid-cols-2 gap-2"><div className="rounded-md bg-muted/50 p-3"><p className="text-muted-foreground">Volumen acumulado</p><p className="font-mono text-base font-bold">{formatCurrency(14850)}</p></div><div className="rounded-md bg-muted/50 p-3"><p className="text-muted-foreground">Línea de crédito usada</p><p className="text-base font-bold">45% (30 días)</p></div></div>
          <div className="flex gap-2"><Button variant="secondary" className="flex-1 font-semibold" render={<Link href="/compras/proveedores/s1" />} nativeButton={false}><History data-icon="inline-start" /> Historial proveedor</Button><Button variant="secondary" size="icon" aria-label="Llamar"><Phone /></Button></div>
        </SectionCard>
        <SectionCard title="Control de detracciones SPOT" icon={Landmark} action={<Tag tone="info" label="Banco de la Nación" />} contentClassName="flex flex-col gap-3 p-4 text-xs">
          <p className="text-muted-foreground">Comprobantes sujetos al SPOT mayores a S/ 700.00 en servicios médicos dentales o prótesis odontológicas.</p>
          <div className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2"><span className="text-muted-foreground">Total retenido mes:</span><span className="font-mono text-base font-bold text-primary">{formatCurrency(1452)}</span></div>
          <div className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2"><span className="text-muted-foreground">Constancias cargadas:</span><span className="font-bold">3 de 4</span></div>
          <Button variant="secondary" className="font-semibold" onClick={() => toast.info("Subir constancias SPOT")}><Upload data-icon="inline-start" /> Subir constancia masiva</Button>
        </SectionCard>
      </div>

      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Subir factura electrónica (XML)</DialogTitle>
            <DialogDescription>El XML se valida contra SUNAT y se clasifica el gasto para el RCE.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <label className="flex cursor-pointer flex-col items-center justify-center gap-1 rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground hover:bg-muted/40">
              <Upload className="size-5" />
              Arrastra el XML/PDF o <span className="text-primary underline">examina tu equipo</span>
              <input type="file" className="sr-only" accept=".xml,.pdf,.zip" />
            </label>
            <Field label="Clasificación de gasto">
              <Select defaultValue="insumos" items={[{ value: "insumos", label: "Insumos clínicos" }, { value: "lab", label: "Laboratorio dental" }, { value: "servicios", label: "Servicios" }]}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value="insumos">Insumos clínicos</SelectItem><SelectItem value="lab">Laboratorio dental</SelectItem><SelectItem value="servicios">Servicios</SelectItem></SelectContent>
              </Select>
            </Field>
            <Field label="Forma de pago"><Input placeholder="Crédito 30 días" /></Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUploadOpen(false)}>Cancelar</Button>
            <Button onClick={() => { toast.success("Comprobante procesado e incluido en el RCE"); setUploadOpen(false); }}>Procesar comprobante</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
