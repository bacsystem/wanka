import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { OrdersScreen } from "@/features/purchases/components/orders-screen";
import { PurchasesTabs } from "@/features/purchases/components/purchases-tabs";
import { purchasesSummary } from "@/features/purchases/mocks/purchases";
import { purchaseOrdersMock, suppliersSummary as s } from "@/features/purchases/mocks/suppliers";

export const metadata: Metadata = { title: "Órdenes de compra" };

export default function OrdersPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Compras y proveedores" title="Órdenes de compra y detracciones" description="Seguimiento de órdenes, recepciones parciales y constancias SPOT." />
      <PurchasesTabs tab="ordenes" counts={{ registro: purchasesSummary.monthCount, proveedores: s.active, ordenes: s.pendingOrders }} />
      <OrdersScreen orders={purchaseOrdersMock} />
    </div>
  );
}
