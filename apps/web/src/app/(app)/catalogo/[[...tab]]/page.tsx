import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Briefcase, Landmark, Package, Pill } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { CatalogScreen, type CatalogTab } from "@/features/catalog/components/catalog-screen";
import { catalogCategoriesMock, catalogEntries, catalogSummary as s } from "@/features/catalog/mocks/catalog-items";

const tabs: Record<string, CatalogTab> = { productos: "productos", servicios: "servicios", categorias: "categorias" };

export const metadata: Metadata = { title: "Catálogo" };

export default async function CatalogPage({ params }: PageProps<"/catalogo/[[...tab]]">) {
  const { tab: segments } = await params;
  const tab: CatalogTab = !segments?.length ? "todos" : (tabs[segments[0]] ?? notFound());
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <CatalogScreen tab={tab} entries={catalogEntries} categories={catalogCategoriesMock} total={s.items} kpis={
        <StatGrid>
          <StatCard label="Ítems registrados" value={String(s.items)} icon={Package} footer={<span className="flex w-full justify-between"><span>{s.services} tratamientos • {s.products} fármacos</span><span className="font-semibold text-primary">100% sincronizado</span></span>} />
          <StatCard label="Servicios y aranceles" value={String(s.services)} icon={Briefcase} emphasizeValue footer={<span className="flex w-full justify-between"><span>Honorarios médicos odontológicos</span><span className="rounded bg-muted px-1.5 py-0.5 text-foreground">Tarifas válidas</span></span>} />
          <StatCard label="Productos con kardex" value={String(s.products)} icon={Pill} tone="info" emphasizeValue footer={<span className="flex w-full justify-between"><span>{s.lowStock} alertas por stock mínimo</span><span className="font-semibold text-destructive">Reposición</span></span>} />
          <StatCard label="Régimen tributario SUNAT" value={`${s.taxedShare}%`} suffix="Gravado 18%" icon={Landmark} emphasizeValue footer={<span className="flex w-full flex-col gap-1.5"><span className="flex h-1.5 w-full overflow-hidden rounded-full bg-teal-100"><span className="h-full rounded-full bg-primary" style={{ width: `${s.taxedShare}%` }} /></span><span className="flex justify-between"><span>{s.taxedShare}% Op. Gravada (10)</span><span>{s.exemptShare}% Exonerado (20)</span></span></span>} />
        </StatGrid>
      } />
    </div>
  );
}
