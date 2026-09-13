"use client";

import { ArrowLeft, Building2, CreditCard, FileBadge2, LogIn, Lock, MapPin, RefreshCw, Send, ShieldCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EntityHeader } from "@/components/shared/entity-header";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PlatformTenant } from "../mocks/tenants";
import { TenantAvatar, TenantStatusBadge, VerticalBadge } from "./tenant-badges";

const chartConfig = { facturas: { label: "Facturas", color: "var(--primary)" }, boletas: { label: "Boletas", color: "var(--chart-2)" }, notas: { label: "Notas", color: "var(--chart-3)" } } satisfies ChartConfig;
const daily = Array.from({ length: 14 }, (_, i) => { const d = new Date("2026-08-30T00:00:00"); d.setDate(d.getDate() + i); return { day: `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`, facturas: 180 + (i * 37) % 120, boletas: 60 + (i * 23) % 70, notas: 4 + (i % 5) }; });
const tabs = ["Resumen", "Suscripción y facturación", "Sedes y cajas", "Usuarios", "Certificado y OSE", "Consumo y cuotas", "Auditoría y logs", "Soporte y tickets"];

export function TenantDetail({ tenant: t }: { tenant: PlatformTenant }) {
  const [tab, setTab] = React.useState(tabs[0]);
  const pct = Math.round((t.cpeUsed / t.cpeLimit) * 1000) / 10;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <nav className="flex items-center gap-1.5 text-xs text-muted-foreground" aria-label="Ruta"><Link href="/admin/empresas" className="flex items-center gap-1 hover:text-primary"><ArrowLeft className="size-3.5" /> Empresas (tenants)</Link><span>/</span><span className="font-semibold text-foreground">{t.name}</span></nav>

      <EntityHeader
        avatar={<TenantAvatar code={t.code} vertical={t.vertical} className="size-16 text-xl" />}
        title={t.name}
        chips={<><Tag label={t.id} className="font-mono" /><TenantStatusBadge status={t.status} /><StatusBadge tone={t.oseLatencyMs !== null ? "success" : "danger"} icon={ShieldCheck} label={`${t.ose} (${t.oseLatencyMs !== null ? "certificado activo" : "token vencido"})`} /></>}
        meta={<p className="flex flex-wrap items-center gap-x-3 gap-y-1"><span className="font-mono">RUC: {t.ruc}</span><span className="flex items-center gap-1"><MapPin className="size-3.5" /> {t.city}</span><VerticalBadge vertical={t.vertical} /><span className="flex items-center gap-1 font-semibold text-foreground"><CreditCard className="size-3.5 text-primary" /> {t.plan} ({formatCurrency(t.planPrice)} /mes)</span></p>}
        actions={<>
          <Button onClick={() => toast.success(`Impersonando ${t.name}`, { description: "Sesión de soporte auditada · 30 min" })}><LogIn data-icon="inline-start" /> Impersonar consola</Button>
          <Button variant="outline" render={<Link href="/admin/planes" />} nativeButton={false}><CreditCard data-icon="inline-start" /> Cambiar plan</Button>
          <Button variant="outline" className="text-destructive hover:text-destructive" onClick={() => toast.warning(`${t.name} suspendida temporalmente`)}><Lock data-icon="inline-start" /> Suspender</Button>
        </>}
      />

      <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="min-w-0 max-w-full">
        <TabsList variant="card">
          {tabs.map((name) => <TabsTrigger key={name} value={name}>{name}{name === "Sedes y cajas" ? <span className="rounded bg-muted px-1.5 font-mono text-[10px]">{t.sites} / {t.sites * 4}</span> : name === "Usuarios" ? <span className="rounded bg-muted px-1.5 font-mono text-[10px]">{t.users}</span> : name === "Soporte y tickets" ? <span className="rounded bg-emerald-100 px-1.5 font-mono text-[10px] text-emerald-800">1 resuelto</span> : null}</TabsTrigger>)}
        </TabsList>
      </Tabs>

      {tab === "Resumen" ? (
        <>
          <div className="flex flex-wrap items-center gap-3 rounded-xl border border-amber-300 border-l-4 bg-amber-50 p-4 text-sm dark:border-amber-800 dark:bg-amber-950/30">
            <StateIcon state="warning" className="size-5" />
            <div className="min-w-0 flex-1"><p className="flex flex-wrap items-center gap-2 font-bold text-amber-900 dark:text-amber-200">Certificado digital tributario SUNAT próximo a vencer en 18 días <StatusBadge tone="warning" label="Urgencia media" className="rounded-md" /></p><p className="text-xs text-amber-800/80 dark:text-amber-300">Fecha de expiración: 30/09/2026. Notificación automática enviada al contacto legal. Requiere subir un nuevo archivo .pfx antes de la fecha límite para evitar rechazos en {t.ose}.</p></div>
            <div className="flex flex-wrap gap-2"><Button variant="outline" className="bg-card font-semibold" onClick={() => setTab("Certificado y OSE")}><FileBadge2 data-icon="inline-start" /> Gestionar certificado</Button><Button className="bg-amber-600 font-semibold text-white hover:bg-amber-700" onClick={() => toast.success("Recordatorio reenviado al contacto legal")}><Send data-icon="inline-start" /> Reenviar recordatorio</Button></div>
          </div>
          <StatGrid>
            <StatCard label="Consumo CPE del mes" value={`${t.cpeUsed.toLocaleString("es-PE")}`} suffix={`/ ${t.cpeLimit.toLocaleString("es-PE")} CPE`} icon={FileBadge2} emphasizeValue={pct >= 90} tone={pct >= 90 ? "danger" : "default"} footer={<span className="flex w-full flex-col gap-1.5"><span className="flex justify-between"><span>{pct}% consumido</span><span className="font-mono text-foreground">3,410 F | 1,280 B | 130 NC</span></span><span className="h-1.5 w-full overflow-hidden rounded-full bg-muted"><span className={cn("block h-full rounded-full", pct >= 90 ? "bg-rose-500" : "bg-primary")} style={{ width: `${pct}%` }} /></span></span>} />
            <StatCard label="Facturación recurrente (MRR)" value={`${formatCurrency(t.planPrice)}`} suffix="/ mes" icon={CreditCard} tone="info" footer={<span className="flex w-full justify-between"><span>Próximo cobro: 15/10/2026</span><span className="text-foreground">Débito aut. Niubiz</span></span>} />
            <StatCard label="Infraestructura y sedes" value={String(t.sites)} suffix={`sedes · ${t.sites * 4} cajas POS`} icon={Building2} footer={<span className="flex w-full flex-col"><span>{t.sitesNote}</span><span className="text-emerald-600">0 incidentes en las últimas 24h</span></span>} />
            <StatCard label="Salud conector OSE" value="99.85%" suffix="éxito timbrado" icon={ShieldCheck} tone="success" emphasizeValue footer={<span className="flex w-full justify-between"><span>Operador: {t.ose}</span><span className="font-mono text-foreground">{t.oseLatencyMs ?? "—"} ms</span></span>} />
          </StatGrid>
          <div className="grid gap-4 xl:grid-cols-3">
            <SectionCard title="Emisión diaria de comprobantes" description="Comportamiento en los últimos 14 días (30/08/2026 – 12/09/2026)" className="xl:col-span-2" action={<div className="inline-flex rounded-lg bg-muted p-0.5 text-xs font-medium">{["Últimos 14 días", "Este mes", "Mes anterior"].map((r, i) => <span key={r} className={cn("rounded px-2.5 py-1", i === 0 && "bg-card font-semibold text-primary shadow-xs")}>{r}</span>)}</div>} contentClassName="p-4">
              <ChartContainer config={chartConfig} className="aspect-auto h-60 w-full">
                <BarChart data={daily} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="day" tickLine={false} axisLine={false} tickMargin={8} /><YAxis tickLine={false} axisLine={false} width={36} />
                  <ChartTooltip content={<ChartTooltipContent />} /><ChartLegend content={<ChartLegendContent />} />
                  <Bar dataKey="facturas" stackId="a" fill="var(--color-facturas)" isAnimationActive={false} /><Bar dataKey="boletas" stackId="a" fill="var(--color-boletas)" isAnimationActive={false} /><Bar dataKey="notas" stackId="a" fill="var(--color-notas)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
                </BarChart>
              </ChartContainer>
            </SectionCard>
            <SectionCard title="Actividad reciente" description="Eventos del tenant en las últimas 48 h" contentClassName="p-0">
              <ul className="divide-y text-xs">
                {[["ok", "Cobro mensual conciliado (Niubiz)", "12/09 08:31"], ["syncing", "Sincronización de catálogo con OSE", "12/09 07:10"], ["warning", "Certificado por vencer notificado", "11/09 09:00"], ["ok", "Nuevo usuario: recepcion.surco@…", "10/09 16:42"], ["pending", "Solicitud de ampliación de cuota CPE", "10/09 11:20"]].map(([st, txt, at]) => <li key={txt} className="flex items-center gap-3 px-4 py-3"><StateIcon state={st as "ok"} /><span className="min-w-0 flex-1 truncate font-medium">{txt}</span><span className="font-mono text-muted-foreground">{at}</span></li>)}
              </ul>
              <div className="border-t p-3"><Button variant="ghost" size="sm" className="text-xs font-semibold text-primary" onClick={() => setTab("Auditoría y logs")}>Ver auditoría completa →</Button></div>
            </SectionCard>
          </div>
        </>
      ) : (
        <SectionCard title={tab} description="Vista del módulo dentro del expediente del tenant" contentClassName="p-6">
          <div className="flex flex-col items-center gap-3 py-10 text-center text-sm text-muted-foreground"><RefreshCw className="size-6 text-primary" /><p>Contenido de <strong className="text-foreground">{tab}</strong> para {t.name}.</p><p className="text-xs">Último acceso: {formatDate(t.lastAccess)} {t.lastAccess.slice(11, 16)} · {t.users} usuarios · {t.sites} sedes.</p></div>
        </SectionCard>
      )}
    </div>
  );
}
