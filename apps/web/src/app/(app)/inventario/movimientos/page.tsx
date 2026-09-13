import type { Metadata } from "next";
import { ArrowDownToLine, ArrowUpFromLine, PackageMinus, Truck } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { MovementsScreen } from "@/features/inventory/components/movements-screen";
import { movementsMock, movementsSummary as s } from "@/features/inventory/mocks/movements";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Movimientos de almacén" };

export default function MovementsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <MovementsScreen movements={movementsMock} total={s.total} kpis={<StatGrid>
        <StatCard label="Entradas del mes" value={String(s.entries)} icon={ArrowDownToLine} deltaPercent={s.entriesDelta} deltaLabel="vs set" footer={<span className="flex w-full flex-col"><span className="flex justify-between"><span>Total valorizado:</span><strong className="font-mono text-sm text-primary">{formatCurrency(s.entriesValue)}</strong></span><span>Facturas locales y remisiones</span></span>} />
        <StatCard label="Salidas / consumo" value={String(s.exits)} icon={ArrowUpFromLine} tone="info" hint={<span className="font-semibold text-primary">{s.exitsProcedures} procedimientos</span>} footer={<span className="flex w-full flex-col"><span className="flex justify-between"><span>Insumos odontológicos:</span><strong className="font-mono text-sm text-foreground">{formatCurrency(s.exitsValue)}</strong></span><span>Consumos en boxes 1, 2 y quirófano</span></span>} />
        <StatCard label="Mermas y devoluciones" value={String(s.losses)} icon={PackageMinus} tone="danger" emphasizeValue hint="Incidencias" footer={<span className="flex w-full flex-col"><span className="flex justify-between"><span>Pérdida neta estimada:</span><strong className="font-mono text-sm text-destructive">{formatCurrency(s.lossesValue)}</strong></span><span>3 caducidades anestesia, 2 ampollas</span></span>} />
        <StatCard label="En tránsito / traslados" value={String(s.inTransit)} icon={Truck} tone="info" emphasizeValue hint="Guías activas" footer={<span className="flex w-full flex-col"><span className="flex justify-between"><span>San Isidro ⇄ Miraflores:</span><strong className="font-mono text-sm text-primary">{s.transitGuide}</strong></span><span className="flex items-center gap-1 text-primary"><span className="size-1.5 rounded-full bg-emerald-500" /> Validado SUNAT en tiempo real</span></span>} />
      </StatGrid>} />
    </div>
  );
}
