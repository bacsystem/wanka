"use client";

import { Download, LogIn, Lock, LockOpen, MoreHorizontal, Plus, X } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { ConnectionIcon } from "@/components/shared/dynamic-icon";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { verticalLabel, type PlatformTenant, type Vertical } from "../mocks/tenants";
import { TenantAvatar, TenantStatusBadge, VerticalBadge } from "./tenant-badges";

const verticalOptions = [{ value: "all", label: "Todos los rubros" }, ...(Object.keys(verticalLabel) as Vertical[]).map((v) => ({ value: v, label: verticalLabel[v] }))];
const planOptions = [{ value: "all", label: "Todos los planes" }, { value: "Básico", label: "Plan Básico (S/ 180)" }, { value: "Clínico", label: "Plan Clínico (S/ 320)" }, { value: "Pro", label: "Plan Pro Multi (S/ 490)" }, { value: "Unlimited", label: "Enterprise (S/ 850+)" }];
const statusOptions = [{ value: "all", label: "Todos los estados" }, { value: "activa", label: "Activa (vigente)" }, { value: "trial", label: "En periodo de prueba" }, { value: "cuota", label: "Cuota límite" }, { value: "suspendida", label: "Suspendida" }];

export function TenantsTable({ tenants, compact }: { tenants: PlatformTenant[]; compact?: boolean }) {
  const [query, setQuery] = React.useState("");
  const [vertical, setVertical] = React.useState("all");
  const [plan, setPlan] = React.useState("all");
  const [status, setStatus] = React.useState("all");
  const [newOpen, setNewOpen] = React.useState(false);
  const [rows, setRows] = React.useState(tenants);
  const filtered = rows.filter((t) => {
    const q = query.trim().toLowerCase();
    return (vertical === "all" || t.vertical === vertical) && (plan === "all" || t.plan.includes(plan)) && (status === "all" || t.status === status) && (!q || t.name.toLowerCase().includes(q) || t.ruc.includes(q) || t.id.toLowerCase().includes(q));
  });
  const toggleSuspend = (id: string) => setRows((rs) => rs.map((t) => (t.id === id ? { ...t, status: t.status === "suspendida" ? "activa" : "suspendida" } : t)));

  const columns: Column<PlatformTenant>[] = [
    { key: "name", header: "Empresa / RUC", cell: (t) => (
      <div className="flex items-center gap-3">
        <TenantAvatar code={t.code} vertical={t.vertical} />
        <div className="min-w-0"><Link href={`/admin/empresas/${t.id}`} className="flex flex-wrap items-center gap-1.5 text-sm font-bold hover:text-primary hover:underline">{t.name}{t.tag ? <span className={cn("rounded px-1.5 py-0.5 text-[9px] font-bold tracking-wide uppercase", t.status === "suspendida" ? "bg-rose-100 text-rose-800" : t.status === "trial" ? "bg-blue-50 text-blue-800" : t.status === "cuota" ? "bg-amber-100 text-amber-800" : "bg-indigo-50 text-indigo-800")}>{t.tag}</span> : <Tag label={t.id} className="font-mono" />}</Link><p className="font-mono text-xs text-muted-foreground">RUC: {t.ruc} • {t.city}</p></div>
      </div>
    ), className: "max-w-80" },
    { key: "vertical", header: "Rubro", cell: (t) => <VerticalBadge vertical={t.vertical} /> },
    { key: "plan", header: "Plan y tarifa", cell: (t) => <TwoLine primary={<span className="font-mono text-sm font-bold">{t.planPrice ? `${formatCurrency(t.planPrice)} /mes` : "S/ 0.00 (prueba)"}</span>} secondary={<span className={cn(t.status === "suspendida" ? "text-destructive" : "text-primary")}>{t.status === "suspendida" ? t.tag : t.plan}</span>} /> },
    { key: "status", header: "Estado", cell: (t) => <TenantStatusBadge status={t.status} /> },
    { key: "sites", header: "Sedes / usr", hidden: compact, cell: (t) => <TwoLine primary={`${t.sites} sede${t.sites === 1 ? "" : "s"} / ${t.users} usuarios`} secondary={t.sitesNote} /> },
    { key: "usage", header: "Uso comprobantes / mes", cell: (t) => { const pct = Math.round((t.cpeUsed / t.cpeLimit) * 100); const tone = pct >= 90 ? "bg-rose-500" : pct >= 70 ? "bg-amber-500" : "bg-primary"; return (
      <div className="min-w-44"><div className="flex items-center justify-between text-xs"><span className={cn("font-mono font-semibold", pct >= 90 && "text-destructive")}>{t.cpeUsed.toLocaleString("es-PE")} / {t.cpeLimit.toLocaleString("es-PE")} CPE</span><span className={cn("font-semibold", pct >= 90 ? "text-destructive" : "text-muted-foreground")}>{pct}%</span></div><div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", tone)} style={{ width: `${pct}%` }} /></div><p className="mt-0.5 text-[11px] text-muted-foreground">{t.cpeNote}</p></div>
    ); } },
    { key: "ose", header: "Conector OSE", cell: (t) => <div className="flex items-start gap-2"><ConnectionIcon online={t.oseLatencyMs !== null} className="mt-0.5" /><TwoLine primary={<span className="font-semibold">{t.ose}</span>} secondary={t.oseLatencyMs !== null ? `${t.oseLatencyMs}ms latencia` : <span className="text-destructive">{t.oseNote}</span>} /></div> },
    { key: "last", header: "Último acceso", hidden: compact, cell: (t) => <span className="font-mono text-xs tabular-nums">{formatDate(t.lastAccess)} {t.lastAccess.slice(11, 16)} hrs</span> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (t) => (
      <div className="flex justify-end gap-1">
        <Button size="sm" variant="outline" className="font-semibold" onClick={() => toast.success(`Impersonando ${t.name}`, { description: "Sesión de soporte con auditoría activa" })}><LogIn data-icon="inline-start" /> Entrar</Button>
        <Button size="icon-sm" variant="outline" aria-label={t.status === "suspendida" ? "Reactivar" : "Suspender"} onClick={() => { toggleSuspend(t.id); toast.info(`${t.name}: ${t.status === "suspendida" ? "reactivada" : "suspendida"}`); }}>{t.status === "suspendida" ? <Lock className="text-destructive" /> : <LockOpen />}</Button>
        <Button size="icon-sm" variant="ghost" aria-label="Más"><MoreHorizontal /></Button>
      </div>
    ) },
  ];

  return (
    <>
      <SectionCard title={compact ? "Directorio de empresas clientes (tenants)" : "Directorio global de tenants"} description={compact ? "Monitoreo segmentado por rubro, cuota mensual y consumo de comprobantes SUNAT" : "Consumo mensual de CPE, conectores OSE, límites de emisión y control de acceso"} action={<><Button variant="outline" className="font-semibold" onClick={() => toast.success("Reporte exportado (XLSX)")}><Download data-icon="inline-start" /> Exportar</Button><Button className="font-semibold" onClick={() => setNewOpen(true)}><Plus data-icon="inline-start" /> Alta de empresa</Button></>} contentClassName="p-0">
        <div className="flex flex-col gap-2 border-b p-3">
          <div className="flex flex-wrap items-center gap-2">
            <SearchInput value={query} onChange={setQuery} placeholder="Buscar por RUC, razón social o ID…" aria-label="Buscar tenant" className="flex-1 sm:max-w-none" />
            <Select value={vertical} onValueChange={(v) => setVertical(String(v))} items={verticalOptions}><SelectTrigger className="w-44 bg-muted/50" aria-label="Rubro"><SelectValue /></SelectTrigger><SelectContent>{verticalOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select>
            <Select value={plan} onValueChange={(v) => setPlan(String(v))} items={planOptions}><SelectTrigger className="w-48 bg-muted/50" aria-label="Plan"><SelectValue /></SelectTrigger><SelectContent>{planOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select>
            <Select value={status} onValueChange={(v) => setStatus(String(v))} items={statusOptions}><SelectTrigger className="w-44 bg-muted/50" aria-label="Estado"><SelectValue /></SelectTrigger><SelectContent>{statusOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select>
            <Button variant="ghost" size="icon" aria-label="Limpiar filtros" onClick={() => { setQuery(""); setVertical("all"); setPlan("all"); setStatus("all"); }}><X /></Button>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">Vistas rápidas:
            {([["odontologia", "Odontología (384)"], ["veterinaria", "Veterinarias (291)"], ["restaurante", "Restaurantes (472)"], ["retail", "Retail (335)"]] as const).map(([v, l]) => <button key={v} type="button" onClick={() => setVertical(v)} className={cn("rounded-full border px-2.5 py-1 font-medium transition-colors", vertical === v ? "border-primary bg-accent text-primary" : "bg-card hover:bg-muted")}>{l}</button>)}
            <button type="button" onClick={() => setStatus("cuota")} className={cn("rounded-full border px-2.5 py-1 font-medium text-destructive transition-colors", status === "cuota" ? "border-rose-300 bg-rose-50" : "bg-card hover:bg-muted")}>Cuota CPE &gt; 90% (14)</button>
          </div>
        </div>
        <DataTable columns={columns} rows={filtered} rowKey={(t) => t.id} minWidth="1200px" controlsSlot={<span>Mostrando: <strong className="text-foreground">{filtered.length}</strong> de 1,482 tenants</span>} emptyMessage="No hay ningún tenant que coincida con los filtros aplicados." mobileCard={(t) => (
          <Link href={`/admin/empresas/${t.id}`} className="flex items-start gap-3"><TenantAvatar code={t.code} vertical={t.vertical} /><div className="min-w-0 flex-1"><p className="truncate text-sm font-bold">{t.name}</p><p className="font-mono text-xs text-muted-foreground">RUC {t.ruc}</p><div className="mt-1 flex flex-wrap gap-1"><VerticalBadge vertical={t.vertical} /><TenantStatusBadge status={t.status} /></div></div><span className="font-mono text-sm font-bold">{formatCurrency(t.planPrice)}</span></Link>
        )} />
        <PaginationBar from={1} to={filtered.length} total={1482} label="empresas registradas" pageSize={compact ? 5 : 25} />
      </SectionCard>

      <Sheet open={newOpen} onOpenChange={setNewOpen}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
          <SheetHeader><SheetTitle>Alta de empresa (tenant)</SheetTitle><SheetDescription>Se consulta el RUC en SUNAT, se crea el espacio de trabajo con su sede inicial y se invita al administrador.</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4 sm:grid-cols-2">
            <Field label="RUC (consulta SUNAT)" className="sm:col-span-2"><div className="flex gap-2"><Input placeholder="20…" className="font-mono" inputMode="numeric" /><Button variant="secondary" className="font-semibold" onClick={() => toast.success("RUC validado: HABIDO · ACTIVO")}>Consultar</Button></div></Field>
            <Field label="Razón social" className="sm:col-span-2"><Input placeholder="Se completa desde SUNAT" /></Field>
            <Field label="Rubro"><Select defaultValue="odontologia" items={verticalOptions.slice(1)}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{verticalOptions.slice(1).map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Plan"><Select defaultValue="Pro" items={planOptions.slice(1)}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{planOptions.slice(1).map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Sede inicial"><Input placeholder="Sede Central" /></Field>
            <Field label="Proveedor OSE"><Select defaultValue="Bizlinks OSE" items={[{ value: "Bizlinks OSE", label: "Bizlinks OSE" }, { value: "Nubefact", label: "Nubefact" }, { value: "Efact OSE", label: "Efact OSE" }, { value: "SUNAT Directo", label: "SUNAT Directo" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="Bizlinks OSE">Bizlinks OSE</SelectItem><SelectItem value="Nubefact">Nubefact</SelectItem><SelectItem value="Efact OSE">Efact OSE</SelectItem><SelectItem value="SUNAT Directo">SUNAT Directo</SelectItem></SelectContent></Select></Field>
            <Field label="Correo del administrador" className="sm:col-span-2"><Input type="email" placeholder="admin@empresa.pe" /></Field>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => setNewOpen(false)}>Cancelar</Button><Button className="font-semibold" onClick={() => { toast.success("Empresa creada · invitación enviada al administrador"); setNewOpen(false); }}>Crear tenant e invitar</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  );
}
