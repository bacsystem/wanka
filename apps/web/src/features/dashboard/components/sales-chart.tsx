"use client";

import { Download } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "khipu-design-system/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { GraficoBarras } from "khipu-design-system/datos/grafico";
import { cn } from "@/lib/utils";
import { formatCurrency, formatDate } from "@/lib/format";
import type { DailySales } from "@/types/domain";
import { SectionCard } from "@/components/shared/section-card";

const ranges = [{ id: "7", label: "7 días", days: 7 }, { id: "30", label: "30 días", days: 30 }, { id: "month", label: "Este mes", days: 12 }] as const;

/** Stitch card: title + SUNAT badge + net total, range segmented control, bars with the peak highlighted, week labels. */
export function SalesChartCard({ data }: { data: DailySales[] }) {
  const [range, setRange] = React.useState<(typeof ranges)[number]["id"]>("30");
  const days = ranges.find((r) => r.id === range)!.days;
  const rows = data.slice(-days);
  const total = rows.reduce((s, r) => s + r.total, 0);
  const peak = rows.reduce((m, r) => (r.total > m.total ? r : m), rows[0]);
  return (
    <SectionCard
      title={<span className="flex flex-wrap items-center gap-2">Ventas últimos {days === 12 ? "días del mes" : `${days} días`} <Tag tone="success" label="SUNAT Facturado" /></span>}
      description={<>Facturación neta acumulada del periodo: <strong className="font-mono font-bold text-foreground">{formatCurrency(total)}</strong></>}
      action={<>
          <div className="inline-flex rounded-lg bg-muted p-0.5 text-xs font-medium" role="tablist" aria-label="Rango">
            {ranges.map((r) => (
              <button key={r.id} type="button" role="tab" aria-selected={range === r.id} onClick={() => setRange(r.id)} className={cn("rounded px-2.5 py-1 text-muted-foreground transition-colors hover:text-foreground", range === r.id && "bg-card font-semibold text-primary shadow-xs")}>{r.label}</button>
            ))}
          </div>
          <Button variant="outline" size="icon" aria-label="Exportar reporte" onClick={() => toast.success("Reporte de ventas exportado (XLSX)")}><Download /></Button>
      </>}
      contentClassName="p-4 pt-4"
    >
      <GraficoBarras
        plano
        alto={176}
        etiqueta="Barras de ventas netas por día"
        categorias={rows.map((r) => formatDate(r.date).slice(0, 5))}
        series={[{ nombre: "Ventas", valores: rows.map((r) => r.total) }]}
        formato={formatCurrency}
        destacado={rows.findIndex((r) => r.date === peak?.date)}
        rotuloVisible={(i) => i % 7 === 0}
        tituloDeCategoria={(i) => formatDate(rows[i].date)}
        descripcion={`Facturación neta de ${rows.length} días: ${formatCurrency(total)}; el pico fue el ${formatDate(peak.date)} con ${formatCurrency(peak.total)}.`}
      />
    </SectionCard>
  );
}
