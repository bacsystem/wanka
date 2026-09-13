"use client";

import { Download, KeyRound, LogIn, Settings2, ShieldAlert, ShieldCheck, Upload } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { auditEvents, type AuditEvent } from "../mocks/ops";

const typeMeta: Record<AuditEvent["type"], { label: string; icon: typeof LogIn }> = { impersonacion: { label: "Impersonación", icon: LogIn }, exportacion: { label: "Exportación masiva", icon: Upload }, plan: { label: "Cambio de plan", icon: Settings2 }, acceso_fallido: { label: "Acceso fallido", icon: ShieldAlert }, config: { label: "Configuración", icon: Settings2 }, certificado: { label: "Certificado", icon: KeyRound } };
const riskTone = { alto: "danger", medio: "warning", bajo: "success" } as const;

export function AuditGlobalScreen() {
  const [query, setQuery] = React.useState("");
  const [type, setType] = React.useState("all");
  const [risk, setRisk] = React.useState("all");
  const rows = auditEvents.filter((e) => (type === "all" || e.type === type) && (risk === "all" || e.risk === risk) && (!query || `${e.actor} ${e.tenant ?? ""} ${e.detail} ${e.ip}`.toLowerCase().includes(query.toLowerCase())));
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Plataforma · Auditoría global y seguridad" title="Auditoría global y seguridad (SIEM)" description="Línea de tiempo de eventos críticos multi-tenant: impersonaciones, exportaciones, cambios de plan, accesos fallidos e IPs." status={<StatusBadge tone="success" icon={ShieldCheck} label="Retención 7 años · WORM" className="rounded-md" />} actions={<Button variant="outline" className="font-semibold" onClick={() => toast.success("Exportación auditada generada")}><Download data-icon="inline-start" /> Exportar</Button>} />
      <StatGrid>
        <StatCard label="Eventos (24 h)" value="1,284" icon={ShieldCheck} footer="Todos los tenants y staff" />
        <StatCard label="Riesgo alto" value="2" icon={ShieldAlert} tone="danger" emphasizeValue footer="1 certificado expirado · 1 fuerza bruta" />
        <StatCard label="Impersonaciones activas" value="1" icon={LogIn} tone="warning" footer="Lucía Paredes → OdontoSalud (TK-10432)" />
        <StatCard label="IPs bloqueadas" value="3" icon={ShieldAlert} tone="info" footer="Bloqueo automático 15 min" />
      </StatGrid>
      <SectionCard title={<span className="sr-only">Eventos</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <div className="flex flex-wrap items-center gap-2 border-b p-3">
          <SearchInput placeholder="Buscar por tenant, usuario, detalle o IP…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
          <Select value={type} onValueChange={(v) => setType(String(v))} items={[{ value: "all", label: "Tipo: todos" }, ...Object.entries(typeMeta).map(([v, m]) => ({ value: v, label: m.label }))]}><SelectTrigger className="w-52 bg-muted/50" aria-label="Tipo"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Tipo: todos</SelectItem>{Object.entries(typeMeta).map(([v, m]) => <SelectItem key={v} value={v}>{m.label}</SelectItem>)}</SelectContent></Select>
          <Select value={risk} onValueChange={(v) => setRisk(String(v))} items={[{ value: "all", label: "Riesgo: todos" }, { value: "alto", label: "Alto" }, { value: "medio", label: "Medio" }, { value: "bajo", label: "Bajo" }]}><SelectTrigger className="w-40 bg-muted/50" aria-label="Riesgo"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Riesgo: todos</SelectItem><SelectItem value="alto">Alto</SelectItem><SelectItem value="medio">Medio</SelectItem><SelectItem value="bajo">Bajo</SelectItem></SelectContent></Select>
        </div>
        <ol className="divide-y">
          {rows.map((e) => { const m = typeMeta[e.type]; return (
            <li key={e.id} className="flex gap-4 px-4 py-3">
              <div className="w-28 shrink-0 font-mono text-xs text-muted-foreground">{formatDate(e.at)}<br />{e.at.slice(11, 19)}</div>
              <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg", e.risk === "alto" ? "bg-rose-50 text-rose-600 dark:bg-rose-950/60" : e.risk === "medio" ? "bg-amber-50 text-amber-600 dark:bg-amber-950/60" : "bg-accent text-accent-foreground")}><m.icon className="size-4" /></span>
              <div className="min-w-0 flex-1 text-sm"><p className="flex flex-wrap items-center gap-2 font-semibold">{m.label}<StatusBadge tone={riskTone[e.risk]} label={`Riesgo ${e.risk}`} />{e.tenant ? <span className="rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium">{e.tenant}</span> : null}</p><p className="text-xs text-muted-foreground">{e.detail}</p><p className="mt-0.5 flex items-center gap-2 text-[11px] text-muted-foreground"><StateIcon state={e.risk === "alto" ? "error" : "ok"} className="size-3" /> {e.actor} · IP <span className="font-mono">{e.ip}</span></p></div>
            </li>
          ); })}
        </ol>
      </SectionCard>
    </div>
  );
}
