import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { LiquidationScreen } from "@/features/reports/components/liquidation-screen";
import { kardexConsolidated, pdt621 } from "@/features/reports/mocks/pdt621";

export const metadata: Metadata = { title: "Liquidación IGV-Renta" };

export default function LiquidationPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Reportes SUNAT · Liquidación tributaria" title="Liquidación mensual IGV-Renta (PDT 621) y kardex consolidado" description="Cálculo preliminar a partir del RVIE y RCE, casillas del PDT 621, cronograma y kardex valorizado por almacén." status={<StatusBadge tone="success" dot label="Consolidado multisede (3 sedes)" />} />
      <LiquidationScreen pdt={pdt621} kardex={kardexConsolidated} />
    </div>
  );
}
