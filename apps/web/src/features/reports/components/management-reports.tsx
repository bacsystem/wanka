"use client";

import { CalendarClock, Download, FileSpreadsheet } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Bar, BarChart, CartesianGrid, Cell, Line, LineChart, Pie, PieChart, XAxis, YAxis } from "recharts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { DataTable, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { formatCurrency, formatDate, formatInteger } from "@/lib/format";
import { cn } from "@/lib/utils";
import { BarChart3, Receipt, ShoppingCart, PieChart as PieIcon } from "lucide-react";
import { bySite, byPayment, dailySales, heatmap, mgmtSummary as s, receivablesAging, topCustomers, topPros, topServices, weeklySales } from "../mocks/management";

const lineCfg = { sales: { label: "Ventas", color: "var(--chart-1)" }, prev: { label: "Período anterior", color: "var(--chart-4)" } } satisfies ChartConfig;
const pct = (a: number, b: number) => ((a - b) / b) * 100;
const ranges = ["Hoy", "Ayer", "Esta semana", "Este mes", "Trimestre", "Personalizado"];
const tabs = [["periodo", "Ventas por período"], ["producto", "Por producto / servicio"], ["vendedor", "Por vendedor / profesional"], ["sede", "Por sede"], ["pago", "Por medio de pago"], ["cxc", "Cuentas por cobrar"], ["clientes", "Clientes"]];

export function ManagementReports() {
  const [range, setRange] = React.useState("Este mes");
  const [compare, setCompare] = React.useState(true);
  const [tab, setTab] = React.useState("periodo");
  const totals = dailySales.reduce((a, d) => ({ sales: a.sales + d.sales, docs: a.docs + d.docs, igv: a.igv + d.igv, disc: a.disc + d.discounts, ret: a.ret + d.returns }), { sales: 0, docs: 0, igv: 0, disc: 0, ret: 0 });
  const dailyColumns: Column<(typeof dailySales)[number]>[] = [
    { key: "date", header: "Fecha", cell: (d) => <span className="font-mono">{formatDate(d.date)}</span> },
    { key: "docs", header: "CPE", align: "right", cell: (d) => <span className="font-mono">{d.docs}</span> },
    { key: "sales", header: "Ventas", align: "right", cell: (d) => <span className="font-mono">{formatCurrency(d.sales)}</span> },
    { key: "igv", header: "IGV", align: "right", cell: (d) => <span className="font-mono text-muted-foreground">{formatCurrency(d.igv)}</span> },
    { key: "discounts", header: "Descuentos", align: "right", cell: (d) => <span className="font-mono text-muted-foreground">{formatCurrency(d.discounts)}</span> },
    { key: "returns", header: "Devoluciones", align: "right", cell: (d) => <span className={cn("font-mono", d.returns && "text-destructive")}>{d.returns ? `− ${formatCurrency(d.returns)}` : "—"}</span> },
    { key: "net", header: "Neto", align: "right", cell: (d) => <span className="font-mono font-medium">{formatCurrency(d.sales - d.discounts - d.returns)}</span> },
  ];
  return (
    <>
      <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card p-3">
        <ToggleGroup value={[range]} onValueChange={(v) => v[0] && setRange(v[0])} variant="outline" size="sm" className="flex-wrap">{ranges.map((r) => <ToggleGroupItem key={r} value={r}>{r}</ToggleGroupItem>)}</ToggleGroup>
        <Badge variant="outline" className="font-mono">01/09/2026 – 18/09/2026</Badge>
        <Select defaultValue="all" items={[{ value: "all", label: "Sede: todas" }, { value: "mira", label: "Sede Miraflores" }, { value: "si", label: "Sede San Isidro" }]}><SelectTrigger className="w-44" aria-label="Sede"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Sede: todas</SelectItem><SelectItem value="mira">Sede Miraflores</SelectItem><SelectItem value="si">Sede San Isidro</SelectItem></SelectContent></Select>
        <Select defaultValue="all" items={[{ value: "all", label: "Profesional: todos (8)" }, ...topPros.map((p) => ({ value: p.name, label: p.name }))]}><SelectTrigger className="w-52" aria-label="Profesional"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Profesional: todos (8)</SelectItem>{topPros.map((p) => <SelectItem key={p.name} value={p.name}>{p.name}</SelectItem>)}</SelectContent></Select>
        <Select defaultValue="all" items={[{ value: "all", label: "Tipo CPE: todos" }, { value: "01", label: "Facturas" }, { value: "03", label: "Boletas" }, { value: "07", label: "Notas de crédito" }]}><SelectTrigger className="w-40" aria-label="Tipo de comprobante"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Tipo CPE: todos</SelectItem><SelectItem value="01">Facturas</SelectItem><SelectItem value="03">Boletas</SelectItem><SelectItem value="07">Notas de crédito</SelectItem></SelectContent></Select>
        <label className="ml-auto flex items-center gap-2 text-xs"><Switch checked={compare} onCheckedChange={setCompare} aria-label="Comparar con período anterior" /> Comparar con período anterior</label>
        <div className="flex gap-1"><Button variant="outline"><FileSpreadsheet data-icon="inline-start" /> Excel</Button><Button variant="outline"><Download data-icon="inline-start" /> PDF</Button><Button variant="outline" onClick={() => toast.success("Envío programado: lunes 08:00 a gerencia@clinicasonrisa.pe")}><CalendarClock data-icon="inline-start" /> Programar envío</Button></div>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList>{tabs.map(([v, l]) => <TabsTrigger key={v} value={v}>{l}</TabsTrigger>)}</TabsList></Tabs>

      {tab === "periodo" ? (
        <>
          <StatGrid>
            <StatCard label="Ventas netas" value={formatCurrency(s.sales)} icon={BarChart3} deltaPercent={compare ? pct(s.sales, s.salesPrev) : null} hint={compare ? `vs ${formatCurrency(s.salesPrev)} período anterior` : "Período seleccionado"} />
            <StatCard label="Comprobantes emitidos" value={formatInteger(s.docs)} icon={Receipt} deltaPercent={compare ? pct(s.docs, s.docsPrev) : null} hint={compare ? `vs ${s.docsPrev} documentos` : "CPE válidos"} />
            <StatCard label="Ticket promedio" value={formatCurrency(s.ticket)} icon={ShoppingCart} deltaPercent={compare ? pct(s.ticket, s.ticketPrev) : null} hint={compare ? `vs ${formatCurrency(s.ticketPrev)}` : ""} tone="success" />
            <StatCard label="Margen bruto estimado" value={`${s.margin}%`} icon={PieIcon} deltaPercent={compare ? s.margin - s.marginPrev : null} hint={`Costo de insumos ${formatCurrency(s.cost)}`} />
          </StatGrid>
          <div className="grid gap-4 xl:grid-cols-3">
            <SectionCard title="Curva diaria de ventas" className="xl:col-span-2" contentClassName="p-4">
              <ChartContainer config={lineCfg} className="aspect-auto h-64 w-full"><LineChart data={dailySales} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}><CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} minTickGap={24} tickFormatter={(d: string) => formatDate(d).slice(0, 5)} /><YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => `${Math.round(v / 1000)}k`} /><ChartTooltip content={<ChartTooltipContent labelFormatter={(d) => formatDate(String(d))} formatter={(v, name) => <span className="flex w-full justify-between gap-4"><span className="text-muted-foreground">{lineCfg[name as keyof typeof lineCfg].label}</span><span className="font-mono">{formatCurrency(Number(v))}</span></span>} />} /><ChartLegend content={<ChartLegendContent />} /><Line type="monotone" dataKey="sales" stroke="var(--color-sales)" strokeWidth={2} dot={false} isAnimationActive={false} />{compare ? <Line type="monotone" dataKey="prev" stroke="var(--color-prev)" strokeWidth={1.5} strokeDasharray="4 4" dot={false} isAnimationActive={false} /> : null}</LineChart></ChartContainer>
            </SectionCard>
            <SectionCard title="Ventas por semana" contentClassName="p-4">
              <ChartContainer config={lineCfg} className="aspect-auto h-64 w-full"><BarChart data={weeklySales} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}><CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="week" tickLine={false} axisLine={false} /><YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => `${Math.round(v / 1000)}k`} /><ChartTooltip content={<ChartTooltipContent formatter={(v) => <span className="font-mono">{formatCurrency(Number(v))}</span>} />} /><Bar dataKey="sales" fill="var(--color-sales)" radius={[3, 3, 0, 0]} isAnimationActive={false} />{compare ? <Bar dataKey="prev" fill="var(--color-prev)" radius={[3, 3, 0, 0]} isAnimationActive={false} /> : null}</BarChart></ChartContainer>
            </SectionCard>
          </div>
          <div className="grid gap-4 xl:grid-cols-3">
            <SectionCard title="Detalle por día" className="xl:col-span-2" contentClassName="p-0">
              <DataTable columns={dailyColumns} rows={dailySales} rowKey={(d) => d.date} minWidth="720px" footer={{ date: "Total", docs: <span className="font-mono">{totals.docs}</span>, sales: <span className="font-mono">{formatCurrency(totals.sales)}</span>, igv: <span className="font-mono">{formatCurrency(totals.igv)}</span>, discounts: <span className="font-mono">{formatCurrency(totals.disc)}</span>, returns: <span className="font-mono">{formatCurrency(totals.ret)}</span>, net: <span className="font-mono">{formatCurrency(totals.sales - totals.disc - totals.ret)}</span> }} />
            </SectionCard>
            <div className="flex min-w-0 flex-col gap-4">
              <SectionCard title="Top 5 servicios" contentClassName="p-0"><ul className="divide-y text-sm">{topServices.map((t, i) => <li key={t.name} className="flex items-center gap-3 px-4 py-2"><span className="w-4 font-mono text-xs text-muted-foreground">{i + 1}</span><div className="min-w-0 flex-1"><p className="truncate">{t.name}</p><div className="mt-1 h-1.5 rounded bg-muted"><div className="h-1.5 rounded bg-primary" style={{ width: `${t.share * 5}%` }} /></div></div><span className="font-mono text-xs">{formatCurrency(t.amount)}</span></li>)}</ul></SectionCard>
              <SectionCard title="Horas pico (semana)" contentClassName="p-3"><div className="grid grid-cols-[1.5rem_1fr] gap-1 text-[10px]">{heatmap.map((r) => <React.Fragment key={r.day}><span className="self-center font-mono text-muted-foreground">{r.day}</span><div className="grid grid-cols-12 gap-0.5">{r.hours.map((c) => <span key={c.h} title={`${r.day} ${c.h}:00`} className="aspect-square rounded-[2px]" style={{ background: `color-mix(in oklch, var(--primary) ${c.v * 10}%, var(--muted))` }} />)}</div></React.Fragment>)}<span /><div className="grid grid-cols-12 gap-0.5 font-mono text-muted-foreground">{[8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19].map((h) => <span key={h} className="text-center">{h}</span>)}</div></div></SectionCard>
            </div>
          </div>
        </>
      ) : null}
      {tab === "producto" ? <SectionCard title="Ventas por producto / servicio" contentClassName="p-0"><DataTable columns={[{ key: "name", header: "Servicio", cell: (t) => t.name }, { key: "amount", header: "Ventas", align: "right", cell: (t) => <span className="font-mono">{formatCurrency(t.amount)}</span> }, { key: "share", header: "Participación", align: "right", cell: (t) => <span className="font-mono">{t.share}%</span> }]} rows={topServices} rowKey={(t) => t.name} minWidth="480px" /></SectionCard> : null}
      {tab === "vendedor" ? <SectionCard title="Ventas por profesional" contentClassName="p-0"><DataTable columns={[{ key: "name", header: "Profesional", cell: (t) => t.name }, { key: "docs", header: "CPE", align: "right", cell: (t) => <span className="font-mono">{t.docs}</span> }, { key: "amount", header: "Ventas", align: "right", cell: (t) => <span className="font-mono">{formatCurrency(t.amount)}</span> }, { key: "ticket", header: "Ticket", align: "right", cell: (t) => <span className="font-mono">{formatCurrency(t.amount / t.docs)}</span> }]} rows={topPros} rowKey={(t) => t.name} minWidth="560px" /></SectionCard> : null}
      {tab === "sede" ? <StatGrid columns={3}>{bySite.map((b) => <StatCard key={b.site} label={`Sede ${b.site}`} value={formatCurrency(b.sales)} icon={BarChart3} hint={`${b.docs} CPE · ticket ${formatCurrency(b.ticket)}`} />)}</StatGrid> : null}
      {tab === "pago" ? <SectionCard title="Distribución por medio de pago" contentClassName="grid gap-4 p-4 md:grid-cols-2"><ChartContainer config={{}} className="aspect-square h-56 w-full"><PieChart><Pie data={byPayment} dataKey="value" nameKey="name" innerRadius={50} outerRadius={80} isAnimationActive={false}>{byPayment.map((e) => <Cell key={e.name} fill={e.fill} />)}</Pie><ChartTooltip content={<ChartTooltipContent formatter={(v) => `${v}%`} />} /></PieChart></ChartContainer><ul className="self-center divide-y text-sm">{byPayment.map((b) => <li key={b.name} className="flex items-center gap-2 py-1.5"><span className="size-3 rounded-sm" style={{ background: b.fill }} /><span className="flex-1">{b.name}</span><span className="font-mono">{b.value}%</span><span className="w-24 text-right font-mono text-xs text-muted-foreground">{formatCurrency((s.sales * b.value) / 100)}</span></li>)}</ul></SectionCard> : null}
      {tab === "cxc" ? <SectionCard title="Antigüedad de cuentas por cobrar" contentClassName="p-0"><DataTable columns={[{ key: "bucket", header: "Tramo", cell: (r) => <span className={cn(receivablesAging.indexOf(r) >= 3 && "text-destructive")}>{r.bucket}</span> }, { key: "count", header: "Comprobantes", align: "right", cell: (r) => <span className="font-mono">{r.count}</span> }, { key: "amount", header: "Saldo", align: "right", cell: (r) => <span className="font-mono font-medium">{formatCurrency(r.amount)}</span> }]} rows={receivablesAging} rowKey={(r) => r.bucket} minWidth="480px" footer={{ bucket: "Total cartera", count: <span className="font-mono">{receivablesAging.reduce((a, r) => a + r.count, 0)}</span>, amount: <span className="font-mono">{formatCurrency(receivablesAging.reduce((a, r) => a + r.amount, 0))}</span> }} /></SectionCard> : null}
      {tab === "clientes" ? <SectionCard title="Top clientes del período" contentClassName="p-0"><DataTable columns={[{ key: "name", header: "Cliente", cell: (c) => c.name }, { key: "docs", header: "CPE", align: "right", cell: (c) => <span className="font-mono">{c.docs}</span> }, { key: "amount", header: "Facturado", align: "right", cell: (c) => <span className="font-mono font-medium">{formatCurrency(c.amount)}</span> }]} rows={topCustomers} rowKey={(c) => c.name} minWidth="480px" /></SectionCard> : null}
    </>
  );
}
