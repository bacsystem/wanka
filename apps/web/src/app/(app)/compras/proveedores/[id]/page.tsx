import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SupplierDetail } from "@/features/purchases/components/supplier-detail";
import { purchasesMock } from "@/features/purchases/mocks/purchases";
import { purchaseOrdersMock, suppliersMock } from "@/features/purchases/mocks/suppliers";

export const metadata: Metadata = { title: "Proveedor" };

export default async function SupplierPage({ params }: PageProps<"/compras/proveedores/[id]">) {
  const { id } = await params;
  const supplier = suppliersMock.find((s) => s.id === id);
  if (!supplier) notFound();
  return <SupplierDetail supplier={supplier} orders={purchaseOrdersMock.filter((o) => o.supplier === supplier.name)} purchases={purchasesMock.filter((p) => p.supplierRuc === supplier.ruc)} />;
}
