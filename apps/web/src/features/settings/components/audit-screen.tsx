"use client";

import { Download, ShieldAlert } from "lucide-react";
import * as React from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput, Toolbar } from "@/components/shared/toolbar";
import { formatDate } from "@/lib/format";
import type { AuditEvent } from "@/features/cash/mocks/shifts";

const sev: Record<AuditEvent["severity"], { label: string; tone: BadgeTone }> = { info: { label: "Info", tone: "neutral" }, advertencia: { label: "Advertencia", tone: "warning" }, critico: { label: "Crítico", tone: "danger" } };
const res: Record<AuditEvent["result"], { label: string; tone: BadgeTone }> = { ok: { label: "OK", tone: "success" }, denegado: { label: "Denegado", tone: "danger" }, error: { label: "Error", tone: "danger" } };

export function AuditScreen({ events }: { events: AuditEvent[] }) {
  const [q, setQ] = React.useState(""); const [s, setS] = React.useState("all"); const [open, setOpen] = React.useState<number | null>(1);
  const rows = events.filter((e) => (s === "all" || e.severity === s) && (!q || e.user.toLowerCase().includes(q.toLowerCase()) || e.action.toLowerCase().includes(q.toLowerCase()) || e.entity.toLowerCase().includes(q.toLowerCase()) || e.module.toLowerCase().includes(q.toLowerCase())));
  const cols: Column<AuditEvent>[] = [
    { key: "at", header: "Fecha / hora", cell: (e) => <span className="font-mono text-xs">{formatDate(e.at)} {e.at.slice(11, 19)}</span> },
    { key: "u", header: "Usuario", cell: (e) => <TwoLine primary={<span className="font-medium">{e.user}</span>} secondary={`${e.role} · ${e.site}`} /> },
    { key: "m", header: "Módulo", cell: (e) => <Badge variant="outline">{e.module}</Badge> },
    { key: "a", header: "Acción / entidad", cell: (e) => <TwoLine primary={e.action} secondary={e.entity} mono /> },
    { key: "ip", header: "IP", cell: (e) => <span className="font-mono text-xs">{e.ip}</span> },
    { key: "r", header: "Resultado", cell: (e) => <StatusBadge tone={res[e.result].tone} dot label={res[e.result].label} /> },
    { key: "sv", header: "Severidad", cell: (e) => <StatusBadge tone={sev[e.severity].tone} label={sev[e.severity].label} /> },
    { key: "d", header: <span className="sr-only">Detalle</span>, align: "right", cell: (e) => (e.before || e.after) ? <Button size="xs" variant="outline" onClick={() => setOpen(open === events.indexOf(e) ? null : events.indexOf(e))}>{open === events.indexOf(e) ? "Ocultar" : "Ver cambios"}</Button> : null },
  ];
  const detail = open !== null ? events[open] : null;
  return (
    <>
      <div className="grid gap-3 sm:grid-cols-3">{[["4 intentos fallidos · IP bloqueada", "critico"], ["Cambio de rol: M. Guerrero → Administrador de sede", "critico"], ["Exportación masiva RVIE por contador externo", "advertencia"]].map(([t, v]) => <div key={t} className="flex items-center gap-2 rounded-lg border bg-card p-3 text-sm"><ShieldAlert className={v === "critico" ? "size-4 text-destructive" : "size-4 text-warning"} /><span className="flex-1">{t}</span><StatusBadge tone={sev[v as AuditEvent["severity"]].tone} label={sev[v as AuditEvent["severity"]].label} /></div>)}</div>
      <SectionCard title="Registro de auditoría" action={<div className="flex items-center gap-2 text-xs text-muted-foreground"><span>Retención 5 años</span><Button size="sm" variant="outline"><Download data-icon="inline-start" /> Exportar CSV</Button></div>} contentClassName="p-0">
        <Toolbar><SearchInput placeholder="Buscar usuario, acción, entidad o módulo" value={q} onChange={setQ} /><Select value={s} onValueChange={(v) => setS(String(v))} items={[{ value: "all", label: "Severidad: todas" }, { value: "info", label: "Info" }, { value: "advertencia", label: "Advertencia" }, { value: "critico", label: "Crítico" }]}><SelectTrigger className="w-44" aria-label="Severidad"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Severidad: todas</SelectItem><SelectItem value="info">Info</SelectItem><SelectItem value="advertencia">Advertencia</SelectItem><SelectItem value="critico">Crítico</SelectItem></SelectContent></Select></Toolbar>
        <DataTable columns={cols} rows={rows} rowKey={(e) => e.at + e.action} minWidth="1100px" mobileCard={(e) => <div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="text-sm font-medium">{e.action}</p><p className="text-xs text-muted-foreground">{e.user} · {e.module} · {formatDate(e.at)} {e.at.slice(11, 16)}</p></div><StatusBadge tone={sev[e.severity].tone} label={sev[e.severity].label} /></div>} />
        {detail && (detail.before || detail.after) ? <div className="grid gap-3 border-t p-4 text-xs sm:grid-cols-2"><div><p className="mb-1 font-medium">Antes</p><pre className="overflow-x-auto rounded-md bg-muted p-2 font-mono">{JSON.stringify(detail.before ?? {}, null, 2)}</pre></div><div><p className="mb-1 font-medium">Después</p><pre className="overflow-x-auto rounded-md bg-muted p-2 font-mono">{JSON.stringify(detail.after ?? {}, null, 2)}</pre></div></div> : null}
      </SectionCard>
    </>
  );
}
