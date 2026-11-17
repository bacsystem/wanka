import type { Metadata } from "next";
import { ArrowLeftRight, Route, ShieldCheck, Truck } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { GuidesScreen } from "@/features/dispatch-guides/components/guides-table";
import { guidesMock, guidesSummary as s } from "@/features/dispatch-guides/mocks/guides";

export const metadata: Metadata = { title: "Guías de remisión" };

export default function GuidesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <GuidesScreen
        guides={guidesMock}
        kpis={
          <StatGrid>
            <StatCard label="GRE emitidas (mes)" value={String(s.monthCount)} icon={Truck} deltaPercent={s.monthDelta} footer="Autorizadas vía OSE / SUNAT" />
            <StatCard label="Traslados entre sedes" value={String(s.betweenSites)} suffix="operaciones" icon={ArrowLeftRight} footer="Miraflores ↔ San Isidro ↔ Surquillo" />
            <StatCard label="En tránsito / despacho" value={String(s.inTransit)} icon={Route} hint={<StatusBadge tone="info" label="En ruta" />} footer="Guías con monitoreo activo" />
            <StatCard label="Validación SUNAT / QR" value={`${s.cdrRate}%`} icon={ShieldCheck} tone="success" emphasizeValue hint="CDR OK" footer="Cero observaciones fiscales" />
          </StatGrid>
        }
      />
    </div>
  );
}
