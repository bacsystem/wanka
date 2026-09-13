"use client";

import { ArrowLeft, FileDown, Mail, Phone, PlusCircle } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { EntityHeader } from "@/components/shared/entity-header";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { initials } from "@/lib/tenant";
import { CalendarCheck, ClipboardCheck, Wallet, ShoppingBag } from "lucide-react";
import type { PurchaseOrder, Supplier } from "../mocks/suppliers";
import type { Purchase } from "../mocks/purchases";

const orderMeta: Record<PurchaseOrder["status"], { label: string; tone: BadgeTone }> = { pendiente: { label: "Pendiente", tone: "warning" }, parcial: { label: "Recepción parcial", tone: "info" }, recibida: { label: "Recibida", tone: "success" }, cancelada: { label: "Cancelada", tone: "neutral" } };

export function SupplierDetail({ supplier, orders, purchases }: { supplier: Supplier; orders: PurchaseOrder[]; purchases: Purchase[] }) {
  const [tab, setTab] = React.useState("ordenes");
  const orderCols: Column<PurchaseOrder>[] = [
    { key: "n", header: "Orden", cell: (o) => <span className="font-mono text-xs font-medium text-primary">{o.number}</span> },
    { key: "d", header: "Emisión / entrega", cell: (o) => <TwoLine primary={formatDate(o.issuedAt)} secondary={`Entrega ${formatDate(o.expectedAt)}`} mono /> },
    { key: "t", header: "Total", align: "right", cell: (o) => <span className="font-mono font-medium">{formatCurrency(o.total)}</span> },
    { key: "s", header: "Estado", cell: (o) => <StatusBadge tone={orderMeta[o.status].tone} dot label={orderMeta[o.status].label} /> },
  ];
  const docCols: Column<Purchase>[] = [
    { key: "n", header: "Comprobante", cell: (p) => <TwoLine primary={<span className="font-mono text-xs">{p.number}</span>} secondary={`Tipo ${p.type}`} /> },
    { key: "d", header: "Emisión", cell: (p) => <span className="font-mono">{formatDate(p.issuedAt)}</span> },
    { key: "t", header: "Total", align: "right", cell: (p) => <span className="font-mono font-medium">{formatCurrency(p.total)}</span> },
    { key: "pay", header: "Pago", cell: (p) => <StatusBadge tone={p.payStatus === "pagado" ? "success" : p.payStatus === "por_vencer" ? "warning" : "neutral"} dot label={p.payStatus === "pagado" ? "Pagado" : p.payStatus === "por_vencer" ? "Por vencer" : "Pendiente"} /> },
    { key: "sire", header: "SIRE", cell: (p) => <StatusBadge tone={p.sire === "aceptado" ? "success" : p.sire === "discrepancia" ? "warning" : "neutral"} label={p.sire === "aceptado" ? "Aceptado" : p.sire === "discrepancia" ? "Discrepancia" : "Excluido"} /> },
  ];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/compras/proveedores" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Proveedores</Button>
      <EntityHeader
        avatar={<InitialsAvatar initials={initials(supplier.name)} size="xl" shape="square" />}
        title={supplier.name}
        chips={<><StatusBadge tone={supplier.sunat === "habido" ? "success" : "danger"} dot label={supplier.sunat === "habido" ? "Habido" : "No habido"} /><Badge variant="outline">{supplier.category}</Badge></>}
        meta={<><p className="flex flex-wrap gap-x-3"><span className="font-mono">RUC {supplier.ruc}</span><span>{supplier.terms}</span><span>Cta. detracciones BN 00-068-124859</span></p><p className="flex flex-wrap gap-x-3"><span>{supplier.contact}</span><span>{supplier.phone}</span><span>{supplier.email}</span></p></>}
        actions={<><Button render={<Link href="/compras/ordenes/nueva" />} nativeButton={false}><PlusCircle data-icon="inline-start" /> Nueva orden</Button><Button variant="outline"><Mail data-icon="inline-start" /> Correo</Button><Button variant="outline"><Phone data-icon="inline-start" /> Llamar</Button><Button variant="outline"><FileDown data-icon="inline-start" /> Estado de cuenta</Button></>}
      />
      <StatGrid>
        <StatCard label="Compras 2026" value={formatCurrency(supplier.yearTotal)} icon={ShoppingBag} hint="Acumulado enero–setiembre" />
        <StatCard label="Por pagar" value={formatCurrency(supplier.pending)} icon={Wallet} tone={supplier.pending ? "warning" : "success"} hint={supplier.pending ? "Facturas a crédito vigentes" : "Sin saldo pendiente"} />
        <StatCard label="Órdenes abiertas" value={String(orders.filter((o) => o.status === "pendiente" || o.status === "parcial").length)} icon={ClipboardCheck} hint="Pendientes de recepción" />
        <StatCard label="Puntualidad de entrega" value="92%" icon={CalendarCheck} tone="success" hint="Últimas 12 órdenes" />
      </StatGrid>
      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList><TabsTrigger value="ordenes">Órdenes <span className="ml-1 font-mono text-[11px] opacity-70">{orders.length}</span></TabsTrigger><TabsTrigger value="comprobantes">Comprobantes <span className="ml-1 font-mono text-[11px] opacity-70">{purchases.length}</span></TabsTrigger><TabsTrigger value="pagos">Pagos</TabsTrigger><TabsTrigger value="docs">Documentos</TabsTrigger></TabsList></Tabs>
      {tab === "ordenes" ? <SectionCard title="Órdenes recientes" contentClassName="p-0"><DataTable columns={orderCols} rows={orders} rowKey={(o) => o.id} minWidth="640px" mobileCard={(o) => <div className="flex items-center justify-between gap-3"><div><p className="font-mono text-xs text-primary">{o.number}</p><p className="text-xs text-muted-foreground">Entrega {formatDate(o.expectedAt)}</p></div><div className="flex flex-col items-end gap-1"><span className="font-mono text-sm font-medium">{formatCurrency(o.total)}</span><StatusBadge tone={orderMeta[o.status].tone} dot label={orderMeta[o.status].label} /></div></div>} /></SectionCard> : null}
      {tab === "comprobantes" ? <SectionCard title="Comprobantes del proveedor" contentClassName="p-0"><DataTable columns={docCols} rows={purchases} rowKey={(p) => p.id} minWidth="720px" mobileCard={(p) => <div className="flex items-center justify-between gap-3"><div><p className="font-mono text-xs">{p.number}</p><p className="text-xs text-muted-foreground">{formatDate(p.issuedAt)}</p></div><span className="font-mono text-sm font-medium">{formatCurrency(p.total)}</span></div>} /></SectionCard> : null}
      {tab === "pagos" ? <SectionCard title="Pagos realizados" contentClassName="p-0"><ul className="divide-y text-sm">{[["2026-09-10", "Transferencia BCP · F002-011820", 2537], ["2026-08-15", "Transferencia BCP · F002-011702", 3180], ["2026-07-12", "Cheque diferido · F002-011590", 4410]].map(([d, c, a]) => <li key={String(c)} className="flex items-center gap-3 px-4 py-2.5"><span className="font-mono text-xs">{formatDate(String(d))}</span><span className="flex-1">{c}</span><span className="font-mono font-medium text-success">{formatCurrency(Number(a))}</span></li>)}</ul></SectionCard> : null}
      {tab === "docs" ? <SectionCard title="Documentos" contentClassName="p-0"><ul className="divide-y text-sm">{["Ficha RUC.pdf", "Contrato marco 2026.pdf", "Certificado DIGEMID.pdf", "Cuenta de detracciones.pdf"].map((f) => <li key={f} className="flex items-center gap-3 px-4 py-2.5"><span className="flex-1">{f}</span><Button variant="ghost" size="icon-sm" aria-label="Descargar"><FileDown /></Button></li>)}</ul></SectionCard> : null}
    </div>
  );
}
