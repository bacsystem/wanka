import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { ManagementReports } from "@/features/reports/components/management-reports";

export const metadata: Metadata = { title: "Reportes de gestión" };

export default function ManagementReportsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Reportes · Gestión comercial" title="Reporte de gestión comercial y ventas" description="Ventas por período, servicio, profesional, sede y medio de pago con comparación contra el período anterior." />
      <ManagementReports />
    </div>
  );
}
