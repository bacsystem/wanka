import { cn } from "@/lib/utils";
import type { DocumentType } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";

const short: Record<DocumentType, string> = { boleta: "Boleta", factura: "Factura", nota_credito: "N. Crédito", proforma: "Proforma", guia_remision: "GRE" };
const cls: Record<DocumentType, string> = {
  boleta: "bg-acento-suave text-acento-superficie-texto",
  factura: "bg-info text-info-foreground",
  nota_credito: "bg-destructive-suave text-destructive-texto",
  proforma: "bg-secondary text-foreground/80",
  guia_remision: "bg-warning text-warning-foreground",
};

/** Chip de tipo de comprobante en estilo pastel compacto (sin borde). */
export function DocumentTypeBadge({ type, className }: { type: DocumentType; className?: string }) {
  return (
    <span className={cn("inline-flex rounded-full px-2.5 py-0.5 text-[10px] font-bold tracking-wide uppercase", cls[type], className)} title={documentTypeLabel[type]} data-type={type}>
      {short[type]}
    </span>
  );
}
