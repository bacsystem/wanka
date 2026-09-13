import { cn } from "@/lib/utils";
import type { DocumentType } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";

const short: Record<DocumentType, string> = { boleta: "Boleta", factura: "Factura", nota_credito: "N. Crédito", proforma: "Proforma", guia_remision: "GRE" };
const cls: Record<DocumentType, string> = {
  boleta: "border-teal-200 bg-teal-50 text-teal-800 dark:border-teal-900 dark:bg-teal-950/60 dark:text-teal-300",
  factura: "border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900 dark:bg-blue-950/60 dark:text-blue-300",
  nota_credito: "border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/60 dark:text-rose-300",
  proforma: "border-border bg-muted text-foreground/80",
  guia_remision: "border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
};

/** Stitch document-type chip: uppercase, bold, tinted with border. */
export function DocumentTypeBadge({ type, className }: { type: DocumentType; className?: string }) {
  return (
    <span className={cn("inline-flex rounded border px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase", cls[type], className)} title={documentTypeLabel[type]} data-type={type}>
      {short[type]}
    </span>
  );
}
