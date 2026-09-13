import type { Metadata } from "next";
import { PurchaseOrderEditor } from "@/features/purchases/components/purchase-order-editor";
import { suppliersMock } from "@/features/purchases/mocks/suppliers";

export const metadata: Metadata = { title: "Nueva orden de compra" };

export default function NewPurchaseOrderPage() {
  return <PurchaseOrderEditor suppliers={suppliersMock} />;
}
