import type { Metadata } from "next";
import { Building2, Gauge, ShieldCheck, UserCheck } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { InventoryTabs } from "@/features/inventory/components/inventory-tabs";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { WarehousesScreen } from "@/features/inventory/components/warehouses-screen";
import { custodians, sunatEstablishments, transfersInTransit, warehouseCards, warehousesSummary as s } from "@/features/inventory/mocks/warehouses";

export const metadata: Metadata = { title: "Almacenes y sedes" };

export default function WarehousesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Inventario" title="Almacenes y sucursales" description="Gestión centralizada de establecimientos anexos declarados ante SUNAT, depósitos de insumos, puntos de venta y control de responsables de custodia." status={<StatusBadge tone="success" dot label="SUNAT sincronizado" />} />
      <InventoryTabs active="/inventario/almacenes" />
      <StatGrid>
        <StatCard label="Sedes y almacenes" value={String(s.total)} suffix={`almacenes en ${s.sites} sedes`} icon={Building2} tone="info" footer={<span className="flex w-full justify-between"><span>{s.bySite}</span><span className="font-semibold text-primary">100% operativos</span></span>} />
        <StatCard label="Capacidad de ocupación" value={`${s.occupancy}%`} icon={Gauge} deltaPercent={s.occupancyDelta} deltaLabel="este mes" footer={<span className="h-1.5 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full rounded-full bg-primary" style={{ width: `${s.occupancy}%` }} /></span>} />
        <StatCard label="Declarados ante SUNAT" value={String(s.annexes)} suffix="establecimientos anexos" icon={ShieldCheck} tone="info" footer={<><Tag tone="info" label="0000 Domicilio fiscal" className="font-mono" /><Tag tone="info" label="0001 Sucursal" className="font-mono" /></>} />
        <StatCard label="Custodia autorizada" value={String(s.custodians)} suffix="colaboradores" icon={UserCheck} footer={<span className="flex w-full justify-between"><span className="flex items-center gap-1 text-primary"><span className="size-1.5 rounded-full bg-primary" /> Firma digital vigente</span><span>Kardex diario</span></span>} />
      </StatGrid>
      <WarehousesScreen cards={warehouseCards} establishments={sunatEstablishments} custodians={custodians} transfers={transfersInTransit} />
    </div>
  );
}
