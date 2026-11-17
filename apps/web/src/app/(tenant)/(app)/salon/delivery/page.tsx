import type { Metadata } from "next";
import { Bike, Clock, Receipt, Users } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { DeliveryScreen } from "@/features/restaurant/components/delivery-screen";
import { deliveryOrders, deliverySummary as s, riders } from "@/features/restaurant/mocks/delivery";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Delivery" };

export default function DeliveryPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Restaurante" title="Delivery" description="Pedidos por WhatsApp, Rappi, PedidosYa y web con asignación de repartidores." actions={<Button variant="outline" render={<Link href="/salon/reservas" />} nativeButton={false}>Reservas</Button>} />
      <StatGrid>
        <StatCard label="Pedidos activos" value={String(s.active)} icon={Receipt} hint="Nuevo, en cocina, listo y en ruta" />
        <StatCard label="Tiempo medio de entrega" value={`${s.avgMinutes} min`} icon={Clock} tone="success" hint="Meta: 35 min" />
        <StatCard label="Repartidores en ruta" value={`${s.ridersOnRoute} / ${s.ridersTotal}`} icon={Bike} hint="1 disponible" />
        <StatCard label="Ventas delivery del turno" value={formatCurrency(s.shiftSales)} icon={Users} hint="16% de las ventas del turno" />
      </StatGrid>
      <DeliveryScreen orders={deliveryOrders} riders={riders} />
    </div>
  );
}
