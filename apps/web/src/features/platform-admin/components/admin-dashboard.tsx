"use client";

import { AlertTriangle, Building2, FileText, Plus, Settings2, TrendingUp, UserMinus, UserPlus } from "lucide-react";
import Link from "next/link";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { incidents, mrrSeries, oseProviders, platformSummary as s, platformTenants } from "../mocks/tenants";
import { TenantsTable } from "./tenants-table";

const chartConfig = { mrr: { label: "MRR facturado", color: "var(--primary)" }, goal: { label: "Proyección meta", color: "var(--chart-2)" } } satisfies ChartConfig;
const priorityMeta = { alta: { label: "Prioridad alta", cls: "border-amber-200 bg-amber-50/60 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30", icon: "warning" as const }, bloqueante: { label: "Bloqueante", cls: "border-rose-200 bg-rose-50/60 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30", icon: "error" as const }, moderada: { label: "Moderada", cls: "border-blue-200 bg-blue-50/60 text-blue-800 dark:border-blue-900 dark:bg-blue-950/30", icon: "pending" as const } };

export function AdminDashboard() {
  const last = mrrSeries.at(-1)!;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader title="Panel maestro de operaciones" description="Supervisión multitenant, flujo tributario SUNAT y performance del negocio" actions={<span className="text-xs text-muted-foreground">Actualizado hoy: {formatDate("2026-09-12")}</span>} />

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-amber-300 border-l-4 bg-amber-50 p-4 text-sm dark:border-amber-800 dark:bg-amber-950/30">
        <StateIcon state="warning" className="size-5" />
        <div className="min-w-0 flex-1"><p className="font-bold text-amber-900 dark:text-amber-200">Alerta OSE SUNAT: degradación de respuesta en proveedor directo</p><p className="text-xs text-amber-800/80 dark:text-amber-300">SUNAT Directo reporta colas de procesamiento con latencia media de 1,420 ms (+85% respecto a la media). El failover a Bizlinks se encuentra en espera automática.</p></div>
        <Button className="bg-amber-600 font-semibold text-white hover:bg-amber-700" render={<Link href="/admin/soporte" />} nativeButton={false}>Ver incidentes (3)</Button>
      </div>

      <section aria-label="Métricas clave de la plataforma" className="flex flex-col gap-3">
        <div className="flex items-center justify-between"><h2 className="text-xs font-bold tracking-wider text-muted-foreground uppercase">Métricas clave de la plataforma (global)</h2></div>
        <StatGrid columns={5}>
          <StatCard label="MRR plataforma" value={formatCurrency(s.mrr)} icon={TrendingUp} deltaPercent={s.mrrDelta} deltaLabel="vs mes anterior" footer={<span className="flex w-full justify-between"><span>ARR estimado:</span><strong className="font-mono text-foreground">{formatCurrency(s.arr)}</strong></span>} />
          <StatCard label="Empresas activas" value={s.active.toLocaleString("es-PE")} icon={Building2} tone="info" hint={<span className="font-semibold text-emerald-600">↗ +{s.activeDelta} netos este mes</span>} footer={<span className="flex w-full justify-between"><span>Capacidad servidores:</span><strong className="text-emerald-600">{s.capacity}% Opt.</strong></span>} />
          <StatCard label="Altas del mes (set)" value={String(s.newMonth)} icon={UserPlus} tone="success" deltaPercent={s.newDelta} deltaLabel={`meta: ${s.newGoal} altas`} footer={<span className="flex w-full justify-between"><span>Conversión trial:</span><strong className="text-foreground">{s.trialConversion}%</strong></span>} />
          <StatCard label="Tasa de churn" value={`${s.churn}%`} icon={UserMinus} tone="danger" deltaPercent={s.churnDelta} deltaLabel="referencia: < 2%" footer={<span className="flex w-full justify-between"><span>Bajas del mes:</span><strong className="text-foreground">{s.churnCount} tenants</strong></span>} />
          <StatCard label="Comprobantes hoy" value={s.cpeToday.toLocaleString("es-PE")} icon={FileText} tone="info" emphasizeValue hint={<span className="font-semibold text-emerald-600">⚡ {s.cdrRate}% CDR aceptados</span>} footer={<span className="flex w-full justify-between"><span>Pico x segundo:</span><strong className="font-mono text-foreground">{s.peakPerSecond} cpe/s</strong></span>} />
        </StatGrid>
      </section>

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Evolución de ingresos recurrentes (MRR 12 meses)" description="Trayectoria histórica Oct 2025 – Set 2026 (crecimiento compuesto: 6.8% mensual)" className="xl:col-span-2" action={<span className="flex items-center gap-3 text-xs"><span className="rounded bg-accent px-2 py-0.5 font-semibold text-accent-foreground">S/ Soles</span><span className="flex items-center gap-1"><span className="size-2 rounded-full bg-primary" /> MRR facturado</span><span className="flex items-center gap-1"><span className="size-2 rounded-full bg-chart-2" /> Proyección meta</span></span>} contentClassName="p-4">
          <ChartContainer config={chartConfig} className="aspect-auto h-64 w-full">
            <AreaChart data={mrrSeries} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <defs><linearGradient id="mrrFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="var(--primary)" stopOpacity={0.25} /><stop offset="100%" stopColor="var(--primary)" stopOpacity={0.02} /></linearGradient></defs>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis dataKey="month" tickLine={false} axisLine={false} tickMargin={8} />
              <YAxis tickLine={false} axisLine={false} width={64} tickFormatter={(v: number) => `S/${Math.round(v / 1000)}k`} />
              <ChartTooltip content={<ChartTooltipContent formatter={(v, name) => <span className="flex w-full justify-between gap-4"><span className="text-muted-foreground">{chartConfig[name as keyof typeof chartConfig].label}</span><span className="font-mono font-medium">{formatCurrency(Number(v))}</span></span>} />} />
              <Area type="monotone" dataKey="goal" stroke="var(--chart-2)" strokeDasharray="4 4" fill="transparent" isAnimationActive={false} />
              <Area type="monotone" dataKey="mrr" stroke="var(--primary)" strokeWidth={2.5} fill="url(#mrrFill)" dot={{ r: 3, fill: "var(--primary)" }} isAnimationActive={false} />
            </AreaChart>
          </ChartContainer>
          <div className="mt-3 grid grid-cols-3 gap-3 border-t pt-3 text-center text-xs">
            <div><p className="text-muted-foreground">MRR inicio (Oct 2025)</p><p className="font-mono text-sm font-bold">{formatCurrency(mrrSeries[0].mrr)}</p></div>
            <div><p className="text-muted-foreground">Crecimiento neto anual</p><p className="font-mono text-sm font-bold text-emerald-600">+{formatCurrency(last.mrr - mrrSeries[0].mrr)} (+{Math.round(((last.mrr - mrrSeries[0].mrr) / mrrSeries[0].mrr) * 1000) / 10}%)</p></div>
            <div><p className="text-muted-foreground">ARPU promedio x empresa</p><p className="font-mono text-sm font-bold text-primary">{formatCurrency(s.mrr / s.active)} /mes</p></div>
          </div>
        </SectionCard>

        <SectionCard title="Monitoreo OSE / SUNAT" description="Latencia en tiempo real de timbrado" action={<span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300">Avg: 385ms</span>} contentClassName="flex flex-col gap-4 p-4">
          {oseProviders.map((p) => (
            <div key={p.name} className="flex flex-col gap-1.5 text-xs">
              <div className="flex items-center justify-between gap-2"><span className="flex items-center gap-2 text-sm font-semibold"><StateIcon state={p.status} className="size-4" /> {p.name} <StatusBadge tone={p.status === "ok" ? "success" : "warning"} label={p.label} className="rounded-md" /></span><span className={cn("font-mono font-semibold", p.status === "warning" ? "text-amber-600" : "text-emerald-600")}>{p.latencyMs.toLocaleString("es-PE")} ms</span></div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", p.status === "ok" ? "bg-emerald-500" : "bg-amber-500")} style={{ width: `${p.availability}%` }} /></div>
              <div className="flex justify-between text-muted-foreground"><span>Disponibilidad 24h: {p.availability}%</span><span>{p.load}</span></div>
            </div>
          ))}
          <Button variant="outline" className="font-semibold" render={<Link href="/admin/monitoreo" />} nativeButton={false}><Settings2 data-icon="inline-start" /> Configurar reglas de failover automático</Button>
        </SectionCard>
      </div>

      <SectionCard title="Incidentes y tickets críticos de plataforma" description="Eventos que impactan facturación o sincronización de comprobantes electrónicos" icon={AlertTriangle} iconTone="warning" action={<><StatusBadge tone="warning" label={`${incidents.length} incidentes abiertos`} className="rounded-md" /><Button variant="outline" className="font-semibold" onClick={() => toast.info("Nuevo incidente")}><Plus data-icon="inline-start" /> Crear incidente</Button></>} contentClassName="grid gap-3 p-4 lg:grid-cols-3">
        {incidents.map((i) => { const m = priorityMeta[i.priority]; return (
          <article key={i.id} className={cn("flex flex-col gap-2 rounded-lg border p-4 text-xs", m.cls)}>
            <div className="flex items-center justify-between"><span className="flex items-center gap-1.5 text-[10px] font-bold tracking-wide uppercase"><StateIcon state={m.icon} className="size-3.5" /> {m.label}</span><span className="font-mono text-muted-foreground">{i.id}</span></div>
            <p className="text-sm font-bold text-foreground">{i.title}</p>
            <p className="text-muted-foreground">{i.text}</p>
            <div className="mt-auto flex items-center justify-between border-t border-current/10 pt-2 text-muted-foreground"><span>Afecta: <strong className="text-foreground">{i.affects}</strong></span><span>Abierto: {formatDate(i.openedAt)} {i.openedAt.slice(11, 16)}</span></div>
          </article>
        ); })}
      </SectionCard>

      <TenantsTable tenants={platformTenants.slice(0, 5)} compact />
    </div>
  );
}
