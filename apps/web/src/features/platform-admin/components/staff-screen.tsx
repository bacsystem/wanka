"use client";

import { Check, Download, Globe, KeyRound, MonitorSmartphone, ShieldAlert, ShieldCheck, UserPlus, Users, X } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { roleMatrix, staff, staffSummary as s, type StaffUser } from "../mocks/ops";

const areas = ["Todas las áreas", "Soporte técnico", "Ventas y expansión", "Finanzas y cobranzas", "Ingeniería y DevOps", "SuperAdmin global"];

export function StaffScreen() {
  const [tab, setTab] = React.useState("users");
  const [query, setQuery] = React.useState("");
  const [area, setArea] = React.useState(areas[0]);
  const [twoFa, setTwoFa] = React.useState("all");
  const [inviteOpen, setInviteOpen] = React.useState(false);
  const rows = staff.filter((u) => (area === areas[0] || u.area === area) && (twoFa === "all" || u.twoFa === twoFa) && (!query || u.name.toLowerCase().includes(query.toLowerCase()) || u.email.includes(query.toLowerCase())));
  const columns: Column<StaffUser>[] = [
    { key: "user", header: "Usuario y correo", cell: (u) => <div className="flex items-center gap-3"><span className="relative flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-accent-foreground">{u.name.split(" ").map((w) => w[0]).slice(0, 2).join("")}<span className={cn("absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-card", u.online ? "bg-emerald-500" : "bg-slate-300")} /></span><TwoLine primary={<span className="font-semibold">{u.name}</span>} secondary={u.email} /></div>, className: "max-w-72" },
    { key: "area", header: "Área y rol", cell: (u) => <TwoLine primary={<span className={cn("rounded-md px-2 py-0.5 text-xs font-semibold", u.area === "SuperAdmin global" ? "bg-primary text-primary-foreground" : "bg-accent text-accent-foreground")}>{u.role}</span>} secondary={u.area} /> },
    { key: "2fa", header: "Seguridad 2FA", cell: (u) => <span className="flex items-center gap-1.5 text-xs"><StateIcon state={u.twoFa === "activo" ? "ok" : u.twoFa === "pendiente" ? "warning" : "error"} />{u.twoFa === "activo" ? "TOTP / FIDO2 activo" : u.twoFa === "pendiente" ? "Pendiente (gracia vence hoy)" : "No configurado"}</span> },
    { key: "sessions", header: "Sesiones activas", align: "center", cell: (u) => <span className="font-mono font-semibold">{u.sessions}</span> },
    { key: "last", header: "Último acceso / IP", cell: (u) => <TwoLine primary={<span className="font-mono text-xs">{formatDate(u.lastAccess)} {u.lastAccess.slice(11, 16)}</span>} secondary={<span className="font-mono">{u.ip}</span>} /> },
    { key: "status", header: "Estado cuenta", cell: (u) => <StatusBadge tone={u.status === "activa" ? "success" : "danger"} dot label={u.status === "activa" ? "Activa" : "Suspendida"} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (u) => <div className="flex justify-end gap-1"><Button size="icon-sm" variant="outline" aria-label="Restablecer credenciales" onClick={() => toast.success(`Enlace de restablecimiento enviado a ${u.email}`)}><KeyRound /></Button><Button size="icon-sm" variant="outline" aria-label="Cerrar sesiones" onClick={() => toast.info(`${u.sessions} sesiones cerradas`)}><MonitorSmartphone /></Button></div> },
  ];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Gobernanza · Usuarios internos y roles" title="Personal interno y control de acceso (RBAC)" description="Administración de operadores de soporte, ejecutivos comerciales, auditores de finanzas e ingenieros con acceso a la infraestructura Wanka." status={<StatusBadge tone="success" icon={ShieldCheck} label="Políticas 2FA obligatorias activas" className="rounded-md" />} actions={<Button className="font-semibold" onClick={() => setInviteOpen(true)}><UserPlus data-icon="inline-start" /> Invitar usuario</Button>} />
      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList variant="card"><TabsTrigger value="users"><Users /> Usuarios staff ({s.total})</TabsTrigger><TabsTrigger value="roles"><ShieldCheck /> Matriz de permisos por rol</TabsTrigger></TabsList></Tabs>
      <StatGrid>
        <StatCard label="Total staff activo" value={String(s.total)} icon={Users} hint={<span className="font-semibold text-emerald-600">+{s.newMonth} este mes</span>} footer={`Distribuidos en ${s.areas} áreas operativas`} />
        <StatCard label="Cobertura 2FA (MFA)" value={`${s.mfa}%`} suffix={s.mfaCount} icon={ShieldCheck} tone="success" emphasizeValue footer={<span className="text-amber-700">1 usuario con gracia (vence hoy)</span>} />
        <StatCard label="Sesiones en vivo" value={String(s.sessions)} suffix={`${s.ips} IPs únicas`} icon={Globe} tone="info" footer="Monitoreo de geolocalización Lima / Cusco" />
        <StatCard label="Intentos fallidos 24h" value={String(s.failed24h)} suffix="0 bloqueados" icon={ShieldAlert} tone="warning" footer="Tasa de rechazo admisible: < 1%" />
      </StatGrid>
      {tab === "users" ? (
        <SectionCard title={<span className="sr-only">Usuarios</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
          <div className="flex flex-wrap items-center gap-2 border-b p-3">
            <SearchInput placeholder="Buscar por nombre o correo…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
            <Select value={area} onValueChange={(v) => setArea(String(v))} items={areas.map((a) => ({ value: a, label: a }))}><SelectTrigger className="w-52 bg-muted/50" aria-label="Área"><SelectValue /></SelectTrigger><SelectContent>{areas.map((a) => <SelectItem key={a} value={a}>{a}</SelectItem>)}</SelectContent></Select>
            <Select value={twoFa} onValueChange={(v) => setTwoFa(String(v))} items={[{ value: "all", label: "Estado 2FA: todos" }, { value: "activo", label: "2FA activo (TOTP/FIDO2)" }, { value: "pendiente", label: "2FA pendiente" }, { value: "no", label: "2FA no configurado" }]}><SelectTrigger className="w-52 bg-muted/50" aria-label="Estado 2FA"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Estado 2FA: todos</SelectItem><SelectItem value="activo">2FA activo (TOTP/FIDO2)</SelectItem><SelectItem value="pendiente">2FA pendiente</SelectItem><SelectItem value="no">2FA no configurado</SelectItem></SelectContent></Select>
            <Button variant="outline" className="font-semibold" onClick={() => toast.success("CSV exportado")}><Download data-icon="inline-start" /> CSV</Button>
          </div>
          <DataTable columns={columns} rows={rows} rowKey={(u) => u.id} minWidth="1000px" mobileCard={(u) => <div className="flex items-center justify-between gap-3"><div className="min-w-0"><p className="truncate text-sm font-semibold">{u.name}</p><p className="truncate text-xs text-muted-foreground">{u.role} · {u.area}</p></div><StatusBadge tone={u.status === "activa" ? "success" : "danger"} dot label={u.status === "activa" ? "Activa" : "Suspendida"} /></div>} />
        </SectionCard>
      ) : (
        <SectionCard title="Matriz de permisos por rol" description="Capacidades por rol interno; los cambios se auditan y requieren doble aprobación." contentClassName="p-0">
          <div className="overflow-x-auto"><Table className="min-w-[720px]"><TableHeader><TableRow><TableHead className="text-left">Capacidad</TableHead>{roleMatrix.roles.map((r) => <TableHead key={r} className="text-center">{r}</TableHead>)}</TableRow></TableHeader><TableBody>{roleMatrix.modules.map(([m, perms]) => <TableRow key={m}><TableCell className="px-4 py-2.5 font-medium">{m}</TableCell>{perms.map((p, i) => <TableCell key={i} className="px-3 py-2.5 text-center">{p ? <span className="inline-flex size-6 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Check className="size-3.5" /></span> : <span className="inline-flex size-6 items-center justify-center rounded-full bg-muted text-muted-foreground"><X className="size-3.5" /></span>}</TableCell>)}</TableRow>)}</TableBody></Table></div>
        </SectionCard>
      )}
      <Sheet open={inviteOpen} onOpenChange={setInviteOpen}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-lg">
          <SheetHeader><SheetTitle>Invitar usuario interno</SheetTitle><SheetDescription>Recibirá un correo con enlace válido 48 h; el 2FA es obligatorio en el primer acceso.</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4">
            <Field label="Correo corporativo"><Input type="email" placeholder="nombre@perusaas.pe" /></Field>
            <Field label="Nombres y apellidos"><Input placeholder="Nombre completo" /></Field>
            <Field label="Área"><Select defaultValue="Soporte técnico" items={areas.slice(1).map((a) => ({ value: a, label: a }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{areas.slice(1).map((a) => <SelectItem key={a} value={a}>{a}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Rol"><Select defaultValue="Soporte nivel 1" items={[{ value: "Soporte nivel 1", label: "Soporte nivel 1" }, { value: "Soporte nivel 2", label: "Soporte nivel 2" }, { value: "Analista de cobranzas", label: "Analista de cobranzas" }, { value: "Ejecutivo comercial", label: "Ejecutivo comercial" }, { value: "Ingeniero de plataforma", label: "Ingeniero de plataforma" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{["Soporte nivel 1", "Soporte nivel 2", "Analista de cobranzas", "Ejecutivo comercial", "Ingeniero de plataforma"].map((r) => <SelectItem key={r} value={r}>{r}</SelectItem>)}</SelectContent></Select></Field>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => setInviteOpen(false)}>Cancelar</Button><Button className="font-semibold" onClick={() => { toast.success("Invitación enviada"); setInviteOpen(false); }}>Enviar invitación</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  );
}
