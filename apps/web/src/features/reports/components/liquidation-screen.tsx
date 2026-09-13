"use client";

import { AlertTriangle, Calculator, CalendarClock, Download, FileSpreadsheet, ShieldCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Button } from "@/components/ui/button";
import { Table, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { Progress } from "@/components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Field } from "@/components/shared/field";
import { DataTable } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { kardexConsolidated as KardexType, pdt621 as PdtType } from "../mocks/pdt621";

const chartConfig = { closing: { label: "Valor final", color: "var(--chart-1)" }, opening: { label: "Saldo inicial", color: "var(--chart-3)" } } satisfies ChartConfig;

export function LiquidationScreen({ pdt, kardex }: { pdt: typeof PdtType; kardex: typeof KardexType }) {
  const totals = kardex.map((w) => ({ warehouse: w.warehouse.split(" (")[0], opening: w.rows.reduce((s, r) => s + r.opening, 0), closing: w.rows.reduce((s, r) => s + r.closing, 0) }));
  const grand = totals.reduce((s, t) => s + t.closing, 0);
  const tiles = [["Ventas gravadas", pdt.sales, "Casilla 100"], ["Débito fiscal (18%)", pdt.debit, "Casilla 140"], ["Compras gravadas", pdt.purchases, "RCE"], ["Crédito fiscal", pdt.credit, "Casilla 145"], ["Saldo a favor anterior", pdt.carryOver, "Arrastre"], ["IGV a pagar", pdt.igvToPay, "Casilla 184"], ["Pago a cuenta renta (1.0%)", pdt.rentaAdvance, "Casilla 312"], ["Total a pagar", pdt.total, "PDT 621"]] as const;
  return (
    <>
      <SectionCard title="Parámetros de liquidación oficial SUNAT" action={<StatusBadge tone="neutral" label="R.S. 000193-2020/SUNAT" />} contentClassName="grid gap-3 p-4 sm:grid-cols-3">
        <Field label="Período tributario"><Select defaultValue="2026-09" items={[{ value: "2026-09", label: "Setiembre 2026 (2026-09)" }, { value: "2026-08", label: "Agosto 2026 · declarado" }, { value: "2026-07", label: "Julio 2026 · declarado" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="2026-09">Setiembre 2026 (2026-09)</SelectItem><SelectItem value="2026-08">Agosto 2026 · declarado</SelectItem><SelectItem value="2026-07">Julio 2026 · declarado</SelectItem></SelectContent></Select></Field>
        <Field label="Régimen tributario"><div className="flex h-10 items-center rounded-lg border bg-muted/50 px-3 text-sm">{pdt.regime}</div></Field>
        <Field label="Coeficiente / tasa renta"><div className="flex h-10 items-center rounded-lg border bg-muted/50 px-3 text-sm">{pdt.coefficient} <StatusBadge tone="neutral" label="Art. 6 RMT" className="ml-2" /></div></Field>
      </SectionCard>

      {pdt.alerts.map((a) => <div key={a} className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm"><p className="flex items-center gap-2"><AlertTriangle className="size-4 text-warning" /> <span><span className="font-medium">Alerta de conciliación SIRE:</span> {a}</span></p><Button size="xs" variant="outline" render={<Link href="/compras" />} nativeButton={false}>Conciliar en SIRE →</Button></div>)}

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title={`Resumen de liquidación preliminar (${pdt.period})`} icon={Calculator} className="xl:col-span-2" contentClassName="p-4">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">{tiles.map(([k, v, c], i) => <div key={k} className={cn("rounded-md border p-3", i === 7 && "col-span-2 border-primary/60 bg-accent/30 md:col-span-1")}><p className="text-[11px] text-muted-foreground uppercase">{k}</p><p className={cn("font-mono text-lg font-semibold tabular-nums", i === 5 && "text-warning", i === 7 && "text-xl")}>{formatCurrency(v)}</p><p className="text-[11px] text-muted-foreground">{c}</p></div>)}</div>
          <div className="mt-4 overflow-hidden rounded-lg border"><DataTable controls={false} columns={[{ key: "cell", header: "Casilla", cell: (c) => <span className="font-mono text-xs">{c.cell.replace("b", "")}</span> }, { key: "label", header: "Concepto", cell: (c) => c.label }, { key: "base", header: "Base", align: "right", cell: (c) => <span className="font-mono tabular-nums">{c.base ? formatCurrency(c.base) : "—"}</span> }, { key: "tax", header: "Tributo", align: "right", cell: (c) => <span className="font-mono font-medium tabular-nums">{c.tax ? formatCurrency(c.tax) : "—"}</span> }, { key: "source", header: "Origen", cell: (c) => <StatusBadge tone="neutral" label={c.source} /> }]} rows={pdt.cells} rowKey={(c) => c.cell + c.label} minWidth="640px" /></div>
          <div className="mt-3 flex flex-wrap gap-2"><Button onClick={() => toast.success("TXT para Declara Fácil 621 generado")}><Download data-icon="inline-start" /> Exportar TXT para Declara Fácil</Button><Button variant="outline"><FileSpreadsheet data-icon="inline-start" /> Exportar Excel</Button></div>
        </SectionCard>
        <SectionCard title="Cronograma de obligaciones" icon={CalendarClock} contentClassName="flex flex-col gap-3 p-4 text-sm">
          <p className="text-xs text-muted-foreground">Anexo I SUNAT · último dígito de RUC <span className="font-mono font-semibold text-foreground">{pdt.dueDigit}</span></p>
          <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Fecha límite de vencimiento</p><p className="font-mono text-2xl font-semibold tabular-nums">{formatDate(pdt.dueDate)}</p><StatusBadge className="mt-1" tone="success" dot label="En plazo reglamentario" /></div>
          <div><div className="flex justify-between text-xs"><span className="text-muted-foreground">Faltan</span><span className="font-mono font-medium">{pdt.daysLeft} días</span></div><Progress value={100 - (pdt.daysLeft / 46) * 100} className="mt-1" /></div>
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground"><ShieldCheck className="size-3.5 text-success" /> Clave SOL conectada · último sync SIRE hoy 08:35</p>
        </SectionCard>
      </div>

      <SectionCard title="Kardex valorizado consolidado por almacén y categoría" action={<div className="flex gap-2"><Button size="sm" variant="outline"><FileSpreadsheet data-icon="inline-start" /> Excel</Button><Button size="sm" onClick={() => toast.success("PLE 13.1 consolidado generado")}><ShieldCheck data-icon="inline-start" /> Generar PLE 13.1 consolidado</Button></div>} contentClassName="grid gap-4 p-4 xl:grid-cols-3">
        <div className="overflow-x-auto xl:col-span-2"><Table className="min-w-[720px]"><TableHeader><TableRow><TableHead className="pl-4">Categoría</TableHead><TableHead className="text-right">Saldo inicial</TableHead><TableHead className="text-right">Entradas</TableHead><TableHead className="text-right">Salidas</TableHead><TableHead className="pr-4 text-right">Saldo final</TableHead></TableRow></TableHeader>
          {kardex.map((w) => { const sub = w.rows.reduce((s, r) => ({ o: s.o + r.opening, i: s.i + r.inflow, x: s.x + r.outflow, c: s.c + r.closing }), { o: 0, i: 0, x: 0, c: 0 }); return (
            <tbody key={w.warehouse} className="divide-y border-t">
              <tr className="bg-muted/20"><td colSpan={5} className="px-3 py-1.5 text-xs font-semibold uppercase">{w.warehouse}</td></tr>
              {w.rows.map((r) => <tr key={r.category}><td className="px-3 py-1.5">{r.category}</td><td className="px-2 py-1.5 text-right font-mono tabular-nums">{formatCurrency(r.opening)}</td><td className="px-2 py-1.5 text-right font-mono text-success tabular-nums">{formatCurrency(r.inflow)}</td><td className="px-2 py-1.5 text-right font-mono text-destructive tabular-nums">{formatCurrency(r.outflow)}</td><td className="px-3 py-1.5 text-right font-mono font-medium tabular-nums">{formatCurrency(r.closing)}</td></tr>)}
              <tr className="font-medium"><td className="px-3 py-1.5 text-xs">Subtotal {w.warehouse.split(" (")[0]}</td><td className="px-2 py-1.5 text-right font-mono tabular-nums">{formatCurrency(sub.o)}</td><td className="px-2 py-1.5 text-right font-mono tabular-nums">{formatCurrency(sub.i)}</td><td className="px-2 py-1.5 text-right font-mono tabular-nums">{formatCurrency(sub.x)}</td><td className="px-3 py-1.5 text-right font-mono tabular-nums">{formatCurrency(sub.c)}</td></tr>
            </tbody>
          ); })}
          <tfoot><tr className="border-t bg-muted/40 font-semibold"><td className="px-3 py-2">Total general</td><td colSpan={3} /><td className="px-3 py-2 text-right font-mono tabular-nums" data-testid="kardex-total">{formatCurrency(grand)}</td></tr></tfoot>
        </Table></div>
        <div><p className="mb-2 text-xs font-medium text-muted-foreground">Valor por almacén</p><ChartContainer config={chartConfig} className="aspect-auto h-56 w-full"><BarChart data={totals} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}><CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="warehouse" tickLine={false} axisLine={false} tickMargin={8} /><YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => `${Math.round(v / 1000)}k`} /><ChartTooltip content={<ChartTooltipContent formatter={(v, name) => <span className="flex w-full justify-between gap-4"><span className="text-muted-foreground">{chartConfig[name as keyof typeof chartConfig].label}</span><span className="font-mono">{formatCurrency(Number(v))}</span></span>} />} /><ChartLegend content={<ChartLegendContent />} /><Bar dataKey="opening" stackId="a" fill="var(--color-opening)" isAnimationActive={false} /><Bar dataKey="closing" stackId="b" fill="var(--color-closing)" radius={[3, 3, 0, 0]} isAnimationActive={false} /></BarChart></ChartContainer></div>
      </SectionCard>
    </>
  );
}
