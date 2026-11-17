import type { Metadata } from "next";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { KardexScreen } from "@/features/inventory/components/kardex-screen";
import { kardexItem, kardexRows } from "@/features/inventory/mocks/kardex";
import { Download, FileSpreadsheet } from "lucide-react";

export const metadata: Metadata = { title: "Kardex valorizado" };

export default function KardexPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader
        eyebrow="Inventario"
        title="Kardex físico y valorizado (SUNAT 13.1)"
        description="Inventario permanente valorizado por artículo y establecimiento · RUC 20608941235 · costeo promedio ponderado móvil."
        status={<StatusBadge tone="success" dot label="PLE 13.1 oficial" />}
        actions={<><Button variant="outline"><FileSpreadsheet data-icon="inline-start" /> Excel</Button><Button><Download data-icon="inline-start" /> PLE 13.1 TXT</Button></>}
      />
      <KardexScreen item={kardexItem} rows={kardexRows} />
    </div>
  );
}
