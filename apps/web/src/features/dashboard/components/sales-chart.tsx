"use client";

import { Download } from "lucide-react";
import * as React from "react";
import { Bar, BarChart, Cell, XAxis, YAxis } from "recharts";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { cn } from "@/lib/utils";
import { formatCurrency, formatDate } from "@/lib/format";
import type { DailySales } from "@/types/domain";
import { SectionCard } from "@/components/shared/section-card";

const chartConfig = { total: { label: "Ventas", color: "var(--chart-1)" } } satisfies ChartConfig;
const ranges = [{ id: "7", label: "7 días", days: 7 }, { id: "30", label: "30 días", days: 30 }, { id: "month", label: "Este mes", days: 12 }] as const;

/** Stitch card: title + SUNAT badge + net total, range segmented control, bars with the peak highlighted, week labels. */
export function SalesChartCard({ data }: { data: DailySales[] }) {
  const [range, setRange] = React.useState<(typeof ranges)[number]["id"]>("30");
  const days = ranges.find((r) => r.id === range)!.days;
  const rows = data.slice(-days);
  const total = rows.reduce((s, r) => s + r.total, 0);
  const peak = rows.reduce((m, r) => (r.total > m.total ? r : m), rows[0]);
  const last = rows.at(-1);
  const weeks = Math.min(4, Math.ceil(rows.length / 7));
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
          <Button variant="outline" size="icon-sm" aria-label="Exportar reporte" onClick={() => toast.success("Reporte de ventas exportado (XLSX)")}><Download /></Button>
      </>}
      contentClassName="p-4 pt-4"
    >
      <div>
        <ChartContainer config={chartConfig} className="aspect-auto h-44 w-full sm:h-48">
          <BarChart data={rows} margin={{ top: 8, right: 0, left: 0, bottom: 0 }} barCategoryGap={6}>
            <XAxis dataKey="date" hide />
            <YAxis hide domain={[0, "dataMax"]} />
            <ChartTooltip
              cursor={{ fill: "var(--muted)" }}
              content={<ChartTooltipContent labelFormatter={(d) => formatDate(String(d))} formatter={(value) => <span className="font-mono font-medium tabular-nums">{formatCurrency(Number(value))}</span>} />}
            />
            <Bar dataKey="total" radius={[3, 3, 0, 0]} isAnimationActive={false}>
              {rows.map((r) => <Cell key={r.date} fill={r.date === peak?.date || r.date === last?.date ? "var(--primary)" : "var(--chart-5)"} />)}
            </Bar>
          </BarChart>
        </ChartContainer>
        <div className="flex justify-between pt-2 text-[11px] font-medium text-muted-foreground">
          {Array.from({ length: weeks }, (_, i) => {
            const d = rows[Math.min(rows.length - 1, i * 7)];
            const isLast = i === weeks - 1;
            return <span key={i} className={cn(isLast && "font-bold text-primary")}>{isLast ? `Semana ${i + 1} (actual)` : `Semana ${i + 1} (${formatDate(d.date).slice(0, 5)})`}</span>;
          })}
        </div>
      </div>
    </SectionCard>
  );
}

/** Bare chart (used by the industry dashboards). */
export function SalesChart({ data }: { data: DailySales[] }) {
  return (
    <ChartContainer config={chartConfig} className="aspect-auto h-56 w-full sm:h-64">
      <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }} barCategoryGap={3}>
        <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} minTickGap={24} tickFormatter={(d: string) => formatDate(d).slice(0, 5)} />
        <YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => (v >= 1000 ? `${Math.round(v / 1000)}k` : String(v))} />
        <ChartTooltip cursor={{ fill: "var(--muted)" }} content={<ChartTooltipContent labelFormatter={(d) => formatDate(String(d))} formatter={(value) => <span className="font-mono font-medium tabular-nums">{formatCurrency(Number(value))}</span>} />} />
        <Bar dataKey="total" fill="var(--color-total)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
      </BarChart>
    </ChartContainer>
  );
}
