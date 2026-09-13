"use client";

import { Eye, FileDown, PlusCircle } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import type { PurchaseOrder } from "../mocks/suppliers";

const statusMeta: Record<PurchaseOrder["status"], { label: string; tone: BadgeTone }> = { pendiente: { label: "Pendiente", tone: "warning" }, parcial: { label: "Recepción parcial", tone: "info" }, recibida: { label: "Recibida", tone: "success" }, cancelada: { label: "Cancelada", tone: "neutral" } };

export function OrdersScreen({ orders }: { orders: PurchaseOrder[] }) {
  const columns: Column<PurchaseOrder>[] = [
    { key: "number", header: "Orden", cell: (o) => <span className="font-mono text-xs font-medium text-primary">{o.number}</span> },
    { key: "supplier", header: "Proveedor", cell: (o) => <span className="font-medium">{o.supplier}</span> },
    { key: "dates", header: "Emisión / entrega", cell: (o) => <TwoLine primary={formatDate(o.issuedAt)} secondary={`Entrega ${formatDate(o.expectedAt)}`} mono /> },
    { key: "total", header: "Total", align: "right", cell: (o) => <span className="font-mono font-medium tabular-nums">{formatCurrency(o.total)}</span> },
    { key: "spot", header: "Detracción / SPOT", cell: (o) => o.detraction ? <TwoLine primary={<span className="font-mono tabular-nums">{o.detraction.rate}% · {formatCurrency(o.detraction.amount)}</span>} secondary={o.detraction.deposited ? "Constancia registrada" : "Pendiente de depósito"} /> : <span className="text-muted-foreground">—</span> },
    { key: "status", header: "Estado", cell: (o) => <StatusBadge tone={statusMeta[o.status].tone} dot label={statusMeta[o.status].label} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (o) => (<div className="flex justify-end gap-0.5"><Button variant="ghost" size="icon-sm" aria-label="Ver orden"><Eye /></Button><Button variant="ghost" size="icon-sm" aria-label="PDF" onClick={() => toast.info(`${o.number}.pdf`)}><FileDown /></Button></div>) },
  ];
  return (
    <>
      <div className="flex justify-end"><Button render={<Link href="/compras/ordenes/nueva" />} nativeButton={false}><PlusCircle data-icon="inline-start" /> Nueva orden de compra</Button></div>
      <SectionCard title="Órdenes de compra y detracciones" contentClassName="p-0">
        <DataTable columns={columns} rows={orders} rowKey={(o) => o.id} minWidth="960px" mobileCard={(o) => (
          <div className="flex items-start gap-3"><div className="min-w-0 flex-1"><p className="font-mono text-xs text-primary">{o.number}</p><p className="truncate text-sm font-medium">{o.supplier}</p><p className="text-xs text-muted-foreground">Entrega {formatDate(o.expectedAt)}</p></div><div className="flex flex-col items-end gap-1"><span className="font-mono text-sm font-medium tabular-nums">{formatCurrency(o.total)}</span><StatusBadge tone={statusMeta[o.status].tone} dot label={statusMeta[o.status].label} /></div></div>
        )} />
        <PaginationBar from={1} to={orders.length} total={orders.length} label="órdenes" />
      </SectionCard>
    </>
  );
}
