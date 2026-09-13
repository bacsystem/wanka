"use client";

import { Activity, Download, FileBadge2, Route, ShieldAlert, Zap } from "lucide-react";
import * as React from "react";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import { expiringCerts, latency24h, providers, rejectTop } from "../mocks/billing";

const chartConfig = { sunat: { label: "SUNAT Directo", color: "#d97706" }, bizlinks: { label: "Bizlinks OSE", color: "var(--primary)" }, nubefact: { label: "Nubefact", color: "#059669" }, efact: { label: "Efact", color: "#7c3aed" } } satisfies ChartConfig;

export function MonitoringScreen() {
  const [range, setRange] = React.useState("24h");
  const [forced, setForced] = React.useState(false);
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Supervisión global · Monitoreo SUNAT / OSE" title="Monitoreo SUNAT / OSE global" description="Estado de los 4 conectores autorizados para 1,482 empresas, latencia de timbrado, colas, tasa de rechazo y certificados por vencer." status={<StatusBadge tone={forced ? "success" : "warning"} icon={forced ? undefined : ShieldAlert} dot={forced} label={forced ? "Failover activo: Bizlinks 100%" : "SUNAT Directo: degradado"} className="rounded-md" />} actions={<><Button variant="outline" className="font-semibold" onClick={() => toast.success("Reglas de failover actualizadas")}><Route data-icon="inline-start" /> Reglas de failover</Button><Button className="font-semibold" disabled={forced} onClick={() => { setForced(true); toast.success("Tráfico reenrutado: Bizlinks OSE 100%"); }}><Zap data-icon="inline-start" /> Reenrutar failover</Button></>} />

      <div className={cn("flex flex-wrap items-center gap-3 rounded-xl border border-l-4 p-4 text-sm", forced ? "border-emerald-300 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/30" : "border-rose-300 bg-rose-50 dark:border-rose-800 dark:bg-rose-950/30")}>
        <StateIcon state={forced ? "ok" : "error"} className="size-5" />
        <div className="min-w-0 flex-1"><p className="flex flex-wrap items-center gap-2 font-bold">{forced ? "Failover aplicado: SUNAT Directo en bypass total" : "Alerta de infraestructura: latencia excesiva en SUNAT Directo (1,420 ms)"} <span className="rounded bg-card px-1.5 py-0.5 font-mono text-[10px]">INC-8421</span><span className="text-xs font-normal text-muted-foreground">Activa hace 28 min</span></p><p className="text-xs text-muted-foreground">SUNAT Directo rechaza por timeout el 3.8% de comprobantes. El balanceador derivó el {forced ? "100%" : "75%"} del tráfico masivo a Bizlinks OSE y Nubefact. 318 empresas clientes operando bajo bypass seguro.</p></div>
        <div className="flex flex-wrap gap-2"><Button variant="outline" className="bg-card font-semibold" onClick={() => toast.info("Diagnóstico CDR")}>Ver diagnóstico CDR</Button>{!forced ? <Button variant="destructive" className="font-semibold" onClick={() => { setForced(true); toast.success("Bizlinks OSE al 100%"); }}>Forzar Bizlinks 100%</Button> : null}</div>
      </div>

      <SectionCard title="Estado de conectores SUNAT / OSE autorizados" description="4 proveedores integrados para 1,482 empresas" icon={Activity} action={<span className="text-xs text-muted-foreground">Tasa de rechazo admisible global: &lt; 0.5%</span>} contentClassName="grid gap-4 p-4 md:grid-cols-2 xl:grid-cols-4">
        {providers.map((p) => (
          <article key={p.name} className={cn("flex flex-col gap-3 rounded-xl border p-4", p.status === "warning" && !forced ? "border-amber-300 bg-amber-50/50 dark:bg-amber-950/20" : "bg-card")}>
            <div className="flex items-start justify-between gap-2"><div><p className="flex items-center gap-2 text-sm font-bold">{p.name}{p.default ? <Tag tone="primary" label="Predeterminado" /> : null}</p><p className="font-mono text-[11px] text-muted-foreground">{p.host}</p></div><StatusBadge tone={p.status === "ok" ? "success" : "warning"} icon={undefined} label={p.label} className="rounded-md" /></div>
            <div className="grid grid-cols-3 gap-2 text-center text-xs"><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">Latencia</p><p className={cn("font-mono font-bold", p.status === "warning" ? "text-amber-600" : "text-emerald-600")}>{p.latencyMs.toLocaleString("es-PE")} ms</p></div><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">Cola CPE</p><p className="font-mono font-bold">{p.queue.toLocaleString("es-PE")}</p></div><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">Rechazo</p><p className={cn("font-mono font-bold", p.reject > 0.5 && "text-destructive")}>{p.reject}%</p></div></div>
            <div><div className="flex justify-between text-xs text-muted-foreground"><span>Disponibilidad 24h</span><span className="font-semibold text-foreground">{p.availability}%</span></div><div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", p.status === "ok" ? "bg-emerald-500" : "bg-amber-500")} style={{ width: `${p.availability}%` }} /></div></div>
            <p className="flex items-center gap-1.5 text-[11px] text-muted-foreground"><StateIcon state={p.status === "warning" ? (forced ? "offline" : "syncing") : "ok"} className="size-3.5" /> {p.note}</p>
          </article>
        ))}
      </SectionCard>

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Telemetría de latencia en timbrado (últimas 24 horas)" description="Comparativa directa de tiempo de respuesta de autorización CDR en milisegundos" className="xl:col-span-2" action={<div className="flex items-center gap-2"><div className="inline-flex rounded-lg bg-muted p-0.5 text-xs font-medium">{["24h", "7 días", "30 días"].map((r) => <button key={r} type="button" onClick={() => setRange(r)} className={cn("rounded px-2.5 py-1", range === r && "bg-card font-semibold text-primary shadow-xs")}>{r}</button>)}</div><Button variant="outline" size="icon-sm" aria-label="Exportar CSV" onClick={() => toast.success("CSV exportado")}><Download /></Button></div>} contentClassName="p-4">
          <ChartContainer config={chartConfig} className="aspect-auto h-64 w-full">
            <LineChart data={latency24h} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="hour" tickLine={false} axisLine={false} tickMargin={8} minTickGap={28} /><YAxis tickLine={false} axisLine={false} width={48} tickFormatter={(v: number) => `${v}ms`} />
              <ChartTooltip content={<ChartTooltipContent />} /><ChartLegend content={<ChartLegendContent />} />
              {(["sunat", "bizlinks", "nubefact", "efact"] as const).map((k) => <Line key={k} type="monotone" dataKey={k} stroke={`var(--color-${k})`} strokeWidth={k === "sunat" ? 2.5 : 1.75} dot={false} isAnimationActive={false} />)}
            </LineChart>
          </ChartContainer>
        </SectionCard>
        <div className="flex flex-col gap-4">
          <SectionCard title="Certificados por vencer" description="Ventanas de 30 / 60 / 90 días" icon={FileBadge2} iconTone="warning" contentClassName="p-0">
            <ul className="divide-y text-xs">{expiringCerts.map((c) => <li key={c.ruc} className="flex items-center gap-3 px-4 py-2.5"><StateIcon state={c.days === 0 ? "error" : c.days <= 30 ? "warning" : "ok"} /><div className="min-w-0 flex-1"><p className="truncate font-semibold">{c.tenant}</p><p className="font-mono text-muted-foreground">RUC {c.ruc} · {c.ose}</p></div><StatusBadge tone={c.days === 0 ? "danger" : c.days <= 30 ? "warning" : "success"} label={c.days === 0 ? "Vencido" : `${c.days} días`} /></li>)}</ul>
          </SectionCard>
          <SectionCard title="Tenants con más rechazos" description="Últimas 24 horas" icon={ShieldAlert} iconTone="danger" contentClassName="p-0">
            <ul className="divide-y text-xs">{rejectTop.map((r) => <li key={r.tenant} className="flex items-center gap-3 px-4 py-2.5"><div className="min-w-0 flex-1"><p className="truncate font-semibold">{r.tenant}</p><p className="text-muted-foreground">{r.cause}</p></div><span className="font-mono font-bold text-destructive">{r.reject}%</span></li>)}</ul>
          </SectionCard>
        </div>
      </div>
    </div>
  );
}
