import type { Metadata } from "next";
import { PosScreen } from "@/features/pos/components/pos-screen";
import { catalogCategories, catalogMock } from "@/features/pos/mocks/catalog";
import { customersMock, genericCustomer } from "@/features/customers/mocks/customers";

export const metadata: Metadata = { title: "Nueva venta" };

export default async function NewSalePage({ searchParams }: PageProps<"/ventas/nueva">) {
  const sp = await searchParams;
  const cliente = typeof sp.cliente === "string" ? sp.cliente : undefined;
  const items = typeof sp.items === "string" ? sp.items.split(",") : Array.isArray(sp.items) ? sp.items : [];
  return (
    <PosScreen items={catalogMock} categories={catalogCategories} customers={customersMock} genericCustomer={genericCustomer} initialCustomerId={cliente} initialItemIds={items} />
  );
}
