import type { Metadata } from "next";
import { ChefHat, Clock, Receipt, Utensils } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { FloorScreen } from "@/features/restaurant/components/floor-screen";
import { floorSummary as s, tablesMock } from "@/features/restaurant/mocks/floor";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Salón y mesas" };

export default function FloorPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Restaurante" title="Salón y mesas" description={`${s.shift} · ocupación ${Math.round((s.occupied / s.total) * 100)}% (${s.occupied}/${s.total})`} status={<StatusBadge tone="success" dot label="Caja salón 01 abierta" />} actions={<Button variant="outline" render={<Link href="/salon/cocina" />} nativeButton={false}><ChefHat data-icon="inline-start" /> Tablero de cocina (KDS)</Button>} />
      <StatGrid>
        <StatCard label="Mesas ocupadas" value={`${s.occupied} / ${s.total}`} icon={Utensils} hint={`${Math.round((s.occupied / s.total) * 100)}% del salón`} />
        <StatCard label="Ticket promedio" value={formatCurrency(s.avgTicket)} icon={Receipt} tone="success" hint="Turno almuerzo" />
        <StatCard label="Comandas en cocina" value={String(s.kitchenTickets)} icon={ChefHat} tone={s.critical ? "warning" : "default"} hint={`${s.critical} crítica (> 15 min)`} />
        <StatCard label="Tiempo medio de atención" value={`${s.avgMinutes} min`} icon={Clock} hint="Desde apertura hasta cobro" />
      </StatGrid>
      <FloorScreen tables={tablesMock} />
    </div>
  );
}
