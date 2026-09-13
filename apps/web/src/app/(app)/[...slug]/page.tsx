import { Construction } from "lucide-react";
import { notFound } from "next/navigation";
import { getNavigation, type NavItem } from "@/config/navigation";
import type { Industry } from "@/types/domain";

const industries: Industry[] = ["odontologia", "veterinaria", "restaurante", "retail"];

function findLabel(items: NavItem[], href: string): string | undefined {
  for (const item of items) {
    if (item.href === href) return item.label;
    const child = item.children && findLabel(item.children, href);
    if (child) return child;
  }
}

/** Placeholder only for routes that exist in some navigation but have no screen yet; anything else is a 404. */
export default async function ModulePlaceholder({ params }: PageProps<"/[...slug]">) {
  const { slug } = await params;
  const href = `/${slug.join("/")}`;
  const label = industries.map((i) => findLabel(getNavigation(i), href)).find(Boolean);
  if (!label) notFound();

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
      <div className="flex size-12 items-center justify-center rounded-lg bg-accent text-accent-foreground">
        <Construction className="size-6" />
      </div>
      <h1 className="text-lg font-semibold">{label}</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        Este módulo aún no está implementado. Forma parte de las siguientes fases del sistema.
      </p>
    </div>
  );
}
