"use client";

import { Check, CreditCard, Download, Plus, Settings2, ShieldAlert, TrendingUp, Users } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { billingSummary as s, plans } from "../mocks/billing";
import { platformTenants } from "../mocks/tenants";
import { TenantStatusBadge } from "./tenant-badges";

const USD = 3.75;

export function PlansScreen() {
  const [currency, setCurrency] = React.useState<"PEN" | "USD">("PEN");
  const money = (v: number) => (currency === "PEN" ? formatCurrency(v) : `$ ${(v / USD).toFixed(2)}`);
  const subs = platformTenants.map((t) => ({ ...t, cycle: "Mensual", nextCharge: "2026-10-15", days: t.status === "suspendida" ? 32 : t.status === "trial" ? 0 : 0 }));
  const columns: Column<(typeof subs)[number]>[] = [
    { key: "tenant", header: "Empresa", cell: (r) => <TwoLine primary={<span className="font-semibold">{r.name}</span>} secondary={`RUC ${r.ruc}`} /> , className: "max-w-72" },
    { key: "plan", header: "Plan", cell: (r) => <span className="font-medium text-primary">{r.plan}</span> },
    { key: "amount", header: "Importe", align: "right", cell: (r) => <span className="font-mono font-bold">{money(r.planPrice)}</span> },
    { key: "cycle", header: "Ciclo", cell: (r) => r.cycle },
    { key: "next", header: "Próximo cobro", cell: (r) => <span className="font-mono">{formatDate(r.nextCharge)}</span> },
    { key: "status", header: "Estado", cell: (r) => <TenantStatusBadge status={r.status} /> },
    { key: "mora", header: "Morosidad", cell: (r) => r.days ? <StatusBadge tone="danger" label={`${r.days} días`} /> : <span className="text-xs text-muted-foreground">Al día</span> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (r) => <Button size="sm" variant="outline" className="font-semibold" onClick={() => toast.info(`Cambiar plan de ${r.name}`)}>Cambiar plan</Button> },
  ];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Facturación y núcleo · Planes y suscripciones" title="Catálogo de planes y gestión de suscripciones" description="Configuración de niveles tarifarios, límites de emisión CPE SUNAT, add-ons modulares y estado de cobranzas de 1,482 empresas clientes." status={<StatusBadge tone="info" label="Catálogo v3.4 (SUNAT 2026)" className="rounded-md" />} actions={<><div className="inline-flex rounded-lg bg-muted p-0.5 text-xs font-semibold">{(["PEN", "USD"] as const).map((c) => <button key={c} type="button" onClick={() => setCurrency(c)} className={cn("rounded px-3 py-2", currency === c && "bg-card text-primary shadow-xs")}>{c === "PEN" ? "S/ (PEN)" : "$ (USD)"}</button>)}</div><Button variant="outline" className="font-semibold" onClick={() => toast.success("Métricas exportadas")}><Download data-icon="inline-start" /> Exportar métricas</Button><Button variant="outline" className="font-semibold"><Settings2 data-icon="inline-start" /> Reglas de corte OSE</Button><Button className="font-semibold" onClick={() => toast.info("Nuevo plan")}><Plus data-icon="inline-start" /> Crear nuevo plan</Button></>} />
      <StatGrid>
        <StatCard label="MRR facturado activo" value={money(s.collected)} icon={TrendingUp} deltaPercent={s.collectedDelta} deltaLabel="vs mes anterior" />
        <StatCard label="Suscripciones activas" value={`${s.activeSubs.toLocaleString("es-PE")}`} suffix="/ 1,482" icon={Users} tone="info" footer={<span className="text-emerald-600">{s.onTime}% al día en pagos</span>} />
        <StatCard label="En morosidad (≥ 5 días)" value={String(s.delinquent)} suffix="empresas" icon={CreditCard} tone="warning" emphasizeValue footer={<span>{money(s.delinquentAmount)} por recaudar</span>} />
        <StatCard label="Timbrado suspendido" value={String(s.suspended)} suffix="empresas" icon={ShieldAlert} tone="danger" emphasizeValue footer="Acceso bloqueado en OSE" />
      </StatGrid>

      <SectionCard title="Catálogo maestro de planes vigentes" description="Estructura de precios, topes de emisión mensual de facturas/boletas SUNAT, usuarios y sedes permitidas." action={<span className="text-xs text-muted-foreground">Tipo de cambio ref: USD 1 = S/ {USD.toFixed(2)}</span>} contentClassName="grid gap-4 p-4 lg:grid-cols-3">
        {plans.map((p) => (
          <article key={p.id} className={cn("relative flex flex-col gap-4 rounded-xl border p-5", p.featured ? "border-primary bg-accent/40 shadow-md ring-1 ring-primary/30 dark:bg-accent/10" : "bg-card")}>
            {p.featured ? <span className="absolute -top-3 left-5 rounded-full bg-primary px-3 py-1 text-[10px] font-bold tracking-wide text-primary-foreground uppercase">Recomendado • Más contratado (58%)</span> : null}
            <div><p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">{p.tagline}</p><h3 className="text-2xl font-bold">{p.name}</h3><p className="mt-1 text-xs text-muted-foreground">{p.audience}</p></div>
            <div><p className="font-mono text-3xl font-bold text-primary">{money(p.price)} <span className="font-sans text-sm font-normal text-muted-foreground">/ mes + IGV</span></p><p className="text-xs text-muted-foreground">Facturado mensualmente o {p.annualNote}</p></div>
            <dl className="grid gap-1.5 rounded-lg bg-muted/50 p-3 text-xs">
              <p className="text-[10px] font-bold tracking-wide text-muted-foreground uppercase">Límites de plataforma</p>
              <div className="flex justify-between"><dt className="text-muted-foreground">Comprobantes CPE:</dt><dd className="font-semibold">{p.cpe}</dd></div>
              <div className="flex justify-between"><dt className="text-muted-foreground">Usuarios simultáneos:</dt><dd className="font-semibold">{p.users}</dd></div>
              <div className="flex justify-between"><dt className="text-muted-foreground">Sedes y locales:</dt><dd className="font-semibold">{p.sites}</dd></div>
            </dl>
            <ul className="flex flex-col gap-1.5 text-xs"><p className="text-[10px] font-bold tracking-wide text-muted-foreground uppercase">Módulos verticales incluidos</p>{p.modules.map((m) => <li key={m} className="flex items-center gap-2"><Check className="size-3.5 text-primary" /> {m}</li>)}<li className="flex items-center gap-2 text-muted-foreground"><Check className="size-3.5" /> {p.support}</li></ul>
            <div className="mt-auto flex flex-col gap-2"><Button variant={p.featured ? "default" : "outline"} className="font-semibold" onClick={() => toast.info(`Configurar ${p.name}`)}>Configurar plan {p.name}</Button><p className="text-center text-xs text-muted-foreground">{p.tenants} empresas activas en este plan</p></div>
          </article>
        ))}
      </SectionCard>

      <SectionCard title="Suscripciones activas" description="Próximos cobros, ciclo y morosidad por empresa" contentClassName="p-0">
        <DataTable columns={columns} rows={subs} rowKey={(r) => r.id} minWidth="1000px" mobileCard={(r) => <div className="flex items-center justify-between gap-3"><div className="min-w-0"><p className="truncate text-sm font-semibold">{r.name}</p><p className="text-xs text-muted-foreground">{r.plan}</p></div><span className="font-mono font-bold">{money(r.planPrice)}</span></div>} />
      </SectionCard>
    </div>
  );
}
