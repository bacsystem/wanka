"use client";

import { ArrowUpDown, Eye, FileCode, FileDown, Filter, MoreHorizontal, Receipt, RefreshCw } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { formatCurrency, formatDate, formatDocumentNumber } from "@/lib/format";
import type { SalesDocument } from "@/types/domain";
import { DocumentTypeBadge } from "@/features/documents/components/document-type-badge";
import { SunatStatusBadge } from "@/features/documents/components/sunat-status-badge";

const time = (iso: string) => iso.slice(11, 16);

export function RecentDocumentsTable({ documents }: { documents: SalesDocument[] }) {
  const columns: Column<SalesDocument>[] = [
    { key: "select", header: <Checkbox aria-label="Seleccionar todos" />, cell: (doc) => <Checkbox aria-label={`Seleccionar ${formatDocumentNumber(doc.series, doc.number)}`} />, className: "w-8" },
    { key: "type", header: "Tipo", cell: (doc) => <DocumentTypeBadge type={doc.type} /> },
    { key: "number", header: "Serie - Número", cell: (doc) => { const n = formatDocumentNumber(doc.series, doc.number); return <Link href={`/ventas/comprobantes/${n}`} className="font-mono font-semibold tabular-nums hover:text-primary hover:underline">{n}</Link>; } },
    { key: "date", header: "Fecha / Hora", cell: (doc) => <span className="whitespace-nowrap text-muted-foreground">{formatDate(doc.issuedAt)} {time(doc.issuedAt)}</span> },
    { key: "customer", header: "Cliente (Razón social / Doc)", cell: (doc) => <TwoLine primary={<span className="font-medium">{doc.customerName}</span>} secondary={`${doc.customerDocument.length === 11 ? "RUC" : doc.customerDocument.length === 8 ? "DNI" : ""} ${doc.customerDocument}`} mono />, className: "max-w-60" },
    { key: "currency", header: "Moneda", hidden: true, cell: (doc) => <span className="font-mono text-muted-foreground">{doc.currency}</span> },
    { key: "total", header: "Total", align: "right", cell: (doc) => <span className="font-mono font-bold whitespace-nowrap tabular-nums">{formatCurrency(doc.total, doc.currency)}</span> },
    { key: "status", header: "Estado SUNAT", align: "center", cell: (doc) => <SunatStatusBadge status={doc.sunatStatus} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (doc) => { const number = formatDocumentNumber(doc.series, doc.number); return (
      <div className="flex justify-end gap-0.5 text-muted-foreground">
        {doc.sunatStatus === "pendiente" ? <Button variant="ghost" size="icon-xs" aria-label="Reintentar envío ahora" className="text-primary" onClick={() => toast.success(`${number} reenviado a SUNAT`)}><RefreshCw /></Button> : <Button variant="ghost" size="icon-xs" aria-label="Ver comprobante" render={<Link href={`/ventas/comprobantes/${number}`} />} nativeButton={false}><Eye /></Button>}
        <Button variant="ghost" size="icon-xs" aria-label="Descargar PDF" onClick={() => toast.success(`${number}.pdf descargado`)}><FileDown /></Button>
        <Button variant="ghost" size="icon-xs" aria-label="Descargar XML" onClick={() => toast.success(`${number}.xml descargado`)}><FileCode /></Button>
        <Button variant="ghost" size="icon-xs" aria-label="Más opciones"><MoreHorizontal /></Button>
      </div>
    ); } },
  ];
  return (
    <SectionCard
      title="Últimos comprobantes emitidos"
      description="Sincronización directa con SUNAT / OSE"
      icon={Receipt}
      action={
        <>
          <Button variant="outline" size="sm" className="text-xs" onClick={() => toast.info("Filtros de comprobantes")}><Filter data-icon="inline-start" /> Filtrar</Button>
          <Button variant="outline" size="sm" className="text-xs" onClick={() => toast.info("Ordenar por fecha")}><ArrowUpDown data-icon="inline-start" /> Ordenar</Button>
          <Link href="/ventas/comprobantes" className="pl-1 text-xs font-semibold text-primary hover:underline">Ver todos →</Link>
        </>
      }
    >
      <DataTable
        columns={columns}
        rows={documents}
        rowKey={(d) => d.id}
        minWidth="720px"
        controlsSlot={<span>Últimos {documents.length} comprobantes del día</span>}
        mobileCard={(doc) => {
          const number = formatDocumentNumber(doc.series, doc.number);
          return (
            <Link href={`/ventas/comprobantes/${number}`} className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2"><DocumentTypeBadge type={doc.type} /><span className="font-mono text-xs font-semibold tabular-nums">{number}</span></div>
                <p className="mt-1 truncate text-sm">{doc.customerName}</p>
                <p className="text-[11px] text-muted-foreground">{formatDate(doc.issuedAt)} {time(doc.issuedAt)}</p>
              </div>
              <div className="flex flex-col items-end gap-1">
                <span className="font-mono text-sm font-bold tabular-nums">{formatCurrency(doc.total, doc.currency)}</span>
                <SunatStatusBadge status={doc.sunatStatus} />
              </div>
            </Link>
          );
        }}
      />
      <div className="flex flex-wrap items-center justify-between gap-2 border-t bg-muted/40 px-4 py-2.5 text-xs text-muted-foreground">
        <span>Mostrando {documents.length} de 38 comprobantes emitidos hoy</span>
        <Button variant="outline" size="sm" className="font-semibold" render={<Link href="/ventas/comprobantes" />} nativeButton={false}>Ver todos los comprobantes →</Button>
      </div>
    </SectionCard>
  );
}
