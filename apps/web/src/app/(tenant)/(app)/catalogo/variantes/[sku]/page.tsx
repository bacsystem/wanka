import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { ProductVariants } from "@/features/retail/components/product-variants";
import { productMock, variantsMock } from "@/features/retail/mocks/variants";

export const metadata: Metadata = { title: "Variantes del producto" };

export default async function VariantsPage({ params }: PageProps<"/catalogo/variantes/[sku]">) {
  const { sku } = await params;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/catalogo/productos" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Productos</Button>
      <PageHeader eyebrow={`Catálogo · ${productMock.category}`} title={`${productMock.name.split(" ").slice(0, 4).join(" ")} (SKU ${sku})`} description="Matriz talla × color con códigos EAN-13, stock por almacén, listas de precios y costo promedio." status={<StatusBadge tone="success" dot label="12 variantes activas" />} />
      <ProductVariants variants={variantsMock} />
    </div>
  );
}
