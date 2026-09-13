"use client";

import { CircleDot, Link2, Plus, ScanLine } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { CatalogItem } from "@/types/domain";

interface CatalogGridProps {
  items: CatalogItem[];
  onAdd: (item: CatalogItem) => void;
}

/** Category chip colors (Stitch: services teal, supplies slate, drugs blue). */
const categoryChip: Record<string, string> = {
  Tratamientos: "bg-teal-50 text-teal-800 dark:bg-teal-950/60 dark:text-teal-300",
  Consultas: "bg-teal-50 text-teal-800 dark:bg-teal-950/60 dark:text-teal-300",
  "Insumos clínicos": "bg-muted text-foreground/80",
  Medicamentos: "bg-blue-50 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300",
};
const categoryShort: Record<string, string> = { Tratamientos: "Servicio dental", Consultas: "Consulta", "Insumos clínicos": "Insumo", Medicamentos: "Medicamento" };

export function CatalogGrid({ items, onAdd }: CatalogGridProps) {
  if (items.length === 0) {
    return <p className="py-16 text-center text-sm text-muted-foreground">Ningún producto o servicio coincide con la búsqueda.</p>;
  }
  return (
    <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-3">
      {items.map((item) => {
        const low = item.stock !== undefined && item.stock <= 5;
        const service = item.kind === "servicio";
        return (
          <li key={item.id}>
            <div className="flex h-full flex-col gap-2 rounded-xl border bg-card p-4 shadow-xs transition-colors hover:border-primary/40">
              <div className="flex items-start justify-between gap-2">
                <span className={cn("rounded px-2 py-0.5 text-[11px] font-medium", categoryChip[item.category] ?? "bg-muted text-foreground/80")}>{categoryShort[item.category] ?? item.category}</span>
                <span className="font-mono text-xs text-muted-foreground">{item.sku}</span>
              </div>
              <p className="text-[15px] leading-5 font-semibold">{item.name}</p>
              <p className={cn("flex items-center gap-1.5 text-xs", low ? "font-medium text-destructive" : service ? "text-primary" : "text-primary")}>
                {service ? <Link2 className="size-3.5" /> : low ? <ScanLine className="size-3.5" /> : <CircleDot className="size-3 fill-current" />}
                {item.stock !== undefined ? `Stock: ${item.stock} ${item.unit}${item.stock > 20 ? " en almacén" : ""}` : (item.note ?? "Disponibilidad clínica inmediata")}
              </p>
              <div className="mt-auto flex items-end justify-between gap-2 pt-2">
                <div>
                  <p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Precio unit.</p>
                  <p className="font-mono text-xl font-bold text-primary tabular-nums">{formatCurrency(item.price)}</p>
                  {!item.taxable ? <span className="text-[10px] text-muted-foreground">Inafecto</span> : null}
                </div>
                <Button size="sm" className="font-semibold" onClick={() => onAdd(item)} aria-label={`Agregar ${item.name}`}><Plus data-icon="inline-start" /> Agregar</Button>
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

export function CategoryChips({ categories, counts, value, onChange }: { categories: readonly string[]; counts: Record<string, number>; value: string; onChange: (c: string) => void }) {
  return (
    <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 md:mx-0 md:flex-wrap md:px-0">
      {categories.map((c) => (
        <button key={c} type="button" onClick={() => onChange(c)} aria-pressed={value === c} className={cn("flex shrink-0 items-center gap-2 rounded-full px-4 py-1.5 text-sm font-medium transition-colors", value === c ? "bg-primary text-primary-foreground shadow-sm" : "bg-accent text-foreground hover:bg-accent/70 dark:bg-muted")}>
          {c}
          <span className={cn("rounded-full px-1.5 font-mono text-[11px] tabular-nums", value === c ? "bg-primary-foreground/20" : "bg-card text-muted-foreground")}>{counts[c] ?? 0}</span>
        </button>
      ))}
    </div>
  );
}
