"use client";

import { Banknote, Clock, Download, FileMinus, FilePlus, Landmark, RefreshCw, Wallet } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { PageHeader } from "@/components/shared/page-header";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { DocumentTypeBadge } from "@/features/documents/components/document-type-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { billingSummary as s, saasInvoices, type InvoiceStatus } from "../mocks/billing";

const statusMeta: Record<InvoiceStatus, { label: string; tone: BadgeTone }> = { pagado: { label: "Pagado", tone: "success" }, pendiente: { label: "Pendiente", tone: "warning" }, moroso: { label: "Moroso / reintentos", tone: "danger" }, anulado: { label: "Anulado", tone: "neutral" } };
const typeMap = { "01": "factura", "03": "boleta", "07": "nota_credito" } as const;

export function BillingScreen() {
  const [tab, setTab] = React.useState("all");
  const [status, setStatus] = React.useState("all");
  const [gateway, setGateway] = React.useState("all");
  const rows = saasInvoices.filter((i) => (tab === "all" || (tab === "nc" ? i.type === "07" : tab === "retry" ? i.status === "moroso" : i.status === tab)) && (status === "all" || i.status === status) && (gateway === "all" || i.gateway.toLowerCase().includes(gateway)));
  const count = (k: string) => k === "all" ? 1482 : k === "pagado" ? 1379 : k === "pendiente" ? 84 : k === "retry" ? 19 : 4;
  const columns: Column<(typeof saasInvoices)[number]>[] = [
    { key: "number", header: "N° comprobante y fecha", cell: (i) => <div className="flex items-center gap-2"><DocumentTypeBadge type={typeMap[i.type]} /><TwoLine primary={<span className="font-mono text-sm font-bold">{i.number}</span>} secondary={`${formatDate(i.issuedAt)} ${i.issuedAt.slice(11, 16)}`} /></div> },
    { key: "tenant", header: "Empresa cliente / RUC", cell: (i) => <TwoLine primary={<span className="font-semibold">{i.tenant}</span>} secondary={`RUC: ${i.ruc}`} />, className: "max-w-64" },
    { key: "concept", header: "Plan y concepto SaaS", cell: (i) => <span className="text-xs">{i.concept}</span> },
    { key: "total", header: "Total S/ (inc. IGV)", align: "right", cell: (i) => <span className={cn("font-mono text-sm font-bold", i.total < 0 && "text-destructive")}>{formatCurrency(i.total)}</span> },
    { key: "gateway", header: "Pasarela y conciliación", cell: (i) => <div className="flex items-center gap-2"><StateIcon state={i.reconciled ? "ok" : i.status === "moroso" ? "error" : "pending"} /><TwoLine primary={i.gateway} secondary={i.reconciled ? "Conciliado automáticamente" : i.retries ? `${i.retries} reintentos fallidos` : "Pendiente de conciliación"} /></div> },
    { key: "status", header: "Estado y OSE", cell: (i) => <div className="flex flex-col gap-1"><StatusBadge tone={statusMeta[i.status].tone} dot label={statusMeta[i.status].label} /><span className="text-[11px] text-muted-foreground">CDR aceptado · Bizlinks</span></div> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (i) => <div className="flex justify-end gap-1">{i.status === "moroso" ? <Button size="sm" className="font-semibold" onClick={() => toast.success(`${i.number}: reintento de cobro enviado a Niubiz`)}><RefreshCw data-icon="inline-start" /> Reintentar cobro</Button> : i.status === "pendiente" ? <Button size="sm" variant="outline" className="font-semibold" onClick={() => toast.success("Recordatorio de pago enviado")}>Recordar</Button> : null}<Button size="icon-sm" variant="ghost" aria-label="Descargar PDF" onClick={() => toast.info(`${i.number}.pdf`)}><Download /></Button></div> },
  ];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Facturación y núcleo · Facturación y conciliación SaaS" title="Facturación y cobranzas de la plataforma" description="Gestión de comprobantes electrónicos B2B emitidos a tenants, pagos por pasarelas (Niubiz, Izipay, BCP), conciliación automática y recuperación de cobros." status={<StatusBadge tone="info" label="Periodo: setiembre 2026" className="rounded-md" />} actions={<><Button variant="outline" className="font-semibold" onClick={() => toast.success("Conciliación bancaria ejecutada · 3 pendientes manuales")}><Landmark data-icon="inline-start" /> Conciliación bancaria automática</Button><Button variant="outline" className="font-semibold" onClick={() => toast.success("Descargando PLE / SIRE…")}><Download data-icon="inline-start" /> Reporte SUNAT (PLE/SIRE)</Button><Button className="font-semibold" onClick={() => toast.success("Facturación masiva del ciclo programada")}><FilePlus data-icon="inline-start" /> Emitir factura masiva</Button></>} />
      <StatGrid columns={5}>
        <StatCard label="Cobrado del mes (MRR)" value={formatCurrency(s.collected)} icon={Wallet} tone="success" deltaPercent={s.collectedDelta} deltaLabel="vs mes anterior" footer={<span className="flex w-full justify-between"><span>Niubiz: {s.niubizShare}%</span><span>BCP transf: {s.bcpShare}%</span></span>} />
        <StatCard label="Por vencer (ciclo 7d)" value={formatCurrency(s.dueSoon)} icon={Clock} tone="info" footer={<span className="flex w-full flex-col"><span>{s.dueCompanies} empresas en ciclo</span><span>Débito auto activo: {s.autoDebit}</span></span>} />
        <StatCard label="Morosidad crítica" value={formatCurrency(s.overdue)} icon={Banknote} tone="danger" emphasizeValue footer={<span className="flex w-full flex-col"><span>{s.overdueTenants} tenants en riesgo</span><span className="text-destructive">Riesgo suspensión OSE inmediata</span></span>} />
        <StatCard label="Notas de crédito (NC)" value={formatCurrency(s.creditNotes)} icon={FileMinus} tone="warning" footer={<span className="flex w-full flex-col"><span>{s.creditNotesCount} ajustes por RUC / downgrade</span><span>0.5% del volumen facturado</span></span>} />
        <StatCard label="Auto-conciliación" value={`${s.autoReconcile}%`} icon={RefreshCw} tone="success" emphasizeValue footer={<span className="flex w-full flex-col"><span>Niubiz y BCP API activo</span><span className="text-amber-700">{s.manualPending} manuales pendientes</span></span>} />
      </StatGrid>

      <SectionCard title={<span className="sr-only">Comprobantes emitidos</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <div className="flex flex-col gap-2 border-b p-3">
          <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="min-w-0 max-w-full"><TabsList variant="pills">{([["all", "Todos"], ["pagado", "Pagados"], ["pendiente", "Pendientes"], ["retry", "Reintentos fallidos"], ["nc", "Notas de crédito"]] as const).map(([v, l]) => <TabsTrigger key={v} value={v}>{l} <span className="ml-1 rounded-full bg-muted px-1.5 font-mono text-[10px] text-muted-foreground group-data-active/tabs-trigger:bg-primary-foreground/20">{count(v).toLocaleString("es-PE")}</span></TabsTrigger>)}</TabsList></Tabs>
          <div className="flex flex-wrap items-center gap-2">
            <Select defaultValue="all" items={[{ value: "all", label: "Tipo: todos" }, { value: "01", label: "Factura electrónica (01)" }, { value: "03", label: "Boleta de venta (03)" }, { value: "07", label: "Nota de crédito (07)" }]}><SelectTrigger className="w-52 bg-muted/50" aria-label="Tipo"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Tipo: todos</SelectItem><SelectItem value="01">Factura electrónica (01)</SelectItem><SelectItem value="03">Boleta de venta (03)</SelectItem><SelectItem value="07">Nota de crédito (07)</SelectItem></SelectContent></Select>
            <Select value={status} onValueChange={(v) => setStatus(String(v))} items={[{ value: "all", label: "Estado: todos" }, { value: "pagado", label: "Pagado" }, { value: "pendiente", label: "Pendiente" }, { value: "moroso", label: "Moroso / reintentos" }, { value: "anulado", label: "Anulado" }]}><SelectTrigger className="w-48 bg-muted/50" aria-label="Estado"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Estado: todos</SelectItem><SelectItem value="pagado">Pagado</SelectItem><SelectItem value="pendiente">Pendiente</SelectItem><SelectItem value="moroso">Moroso / reintentos</SelectItem><SelectItem value="anulado">Anulado</SelectItem></SelectContent></Select>
            <Select value={gateway} onValueChange={(v) => setGateway(String(v))} items={[{ value: "all", label: "Método: todos" }, { value: "niubiz", label: "Niubiz débito auto" }, { value: "bcp", label: "Transferencia BCP" }, { value: "izipay", label: "Izipay checkout" }, { value: "pagoefectivo", label: "PagoEfectivo" }]}><SelectTrigger className="w-48 bg-muted/50" aria-label="Método"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Método: todos</SelectItem><SelectItem value="niubiz">Niubiz débito auto</SelectItem><SelectItem value="bcp">Transferencia BCP</SelectItem><SelectItem value="izipay">Izipay checkout</SelectItem><SelectItem value="pagoefectivo">PagoEfectivo</SelectItem></SelectContent></Select>
          </div>
        </div>
        <DataTable columns={columns} rows={rows} rowKey={(i) => i.id} minWidth="1100px" controlsSlot={<span>Mostrando registros del tenant cluster principal</span>} mobileCard={(i) => <div className="flex items-center justify-between gap-3"><div className="min-w-0"><p className="font-mono text-xs font-bold">{i.number}</p><p className="truncate text-sm">{i.tenant}</p><StatusBadge tone={statusMeta[i.status].tone} dot label={statusMeta[i.status].label} className="mt-1" /></div><span className="font-mono font-bold">{formatCurrency(i.total)}</span></div>} />
        <PaginationBar from={1} to={rows.length} total={1482} label="comprobantes" />
      </SectionCard>
    </div>
  );
}
