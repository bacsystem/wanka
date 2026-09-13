"use client";

import { Building2, History, KeyRound, LockOpen, MoreHorizontal, RotateCcw, Save, ShieldCheck, Smartphone, Store, UserPlus, Users } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { PaginationBar } from "@/components/shared/pagination";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput, Toolbar } from "@/components/shared/toolbar";
import { initials } from "@/lib/tenant";
import { cn } from "@/lib/utils";
import type { Collaborator } from "../mocks/settings";
import { permissionActions, permissionModules } from "../mocks/settings";
import { InviteCollaboratorSheet } from "./invite-collaborator-sheet";

const twoFaMeta: Record<Collaborator["twoFa"], { label: string; tone: BadgeTone }> = {
  authenticator: { label: "Authenticator OK", tone: "success" },
  sms: { label: "SMS verificado", tone: "info" },
  pendiente: { label: "2FA pendiente", tone: "warning" },
};

interface Props {
  collaborators: Collaborator[];
  roles: string[];
  matrix: Record<string, Record<string, ReadonlyArray<string>>>;
}

export function SecurityScreen({ collaborators, roles, matrix, kpis }: Props & { kpis?: React.ReactNode }) {
  const [query, setQuery] = React.useState("");
  const [role, setRole] = React.useState("all");
  const [inviteOpen, setInviteOpen] = React.useState(false);
  const roleOptions = [{ value: "all", label: "Todos los roles" }, ...roles.map((r) => ({ value: r, label: r }))];
  const rows = collaborators.filter((c) => {
    const s = query.trim().toLowerCase();
    return (role === "all" || c.role === role) && (!s || c.name.toLowerCase().includes(s) || c.email.toLowerCase().includes(s));
  });

  const columns: Column<Collaborator>[] = [
    {
      key: "name", header: "Colaborador / identidad",
      cell: (c) => (
        <div className="flex items-center gap-2.5">
          <InitialsAvatar initials={initials(c.name)} />
          <TwoLine primary={<span className="font-medium">{c.name}</span>} secondary={`${c.email} · ${c.identity}`} />
        </div>
      ),
      className: "max-w-80",
    },
    { key: "role", header: "Rol en clínica", cell: (c) => <span className={cn("inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold", c.role === "Super Admin" ? "bg-teal-200 text-teal-900 dark:bg-teal-900 dark:text-teal-100" : /Cajero/.test(c.role) ? "bg-teal-100 text-teal-900 dark:bg-teal-900/50 dark:text-teal-100" : /Administrador/.test(c.role) ? "bg-blue-100 text-blue-900 dark:bg-blue-950 dark:text-blue-200" : "bg-muted text-foreground")}><ShieldCheck className="size-3.5" /> {c.role}</span> },
    { key: "sites", header: "Sedes autorizadas", cell: (c) => <span className="inline-flex items-center gap-1"><Store className="size-3.5 text-muted-foreground" /> {c.sites}</span> },
    { key: "2fa", header: "Seguridad 2FA", cell: (c) => <StatusBadge tone={twoFaMeta[c.twoFa].tone} icon={c.twoFa === "sms" ? Smartphone : ShieldCheck} label={twoFaMeta[c.twoFa].label} /> },
    { key: "last", header: "Último acceso / IP", cell: (c) => <TwoLine primary={c.lastAccess} secondary={c.ip} mono /> },
    { key: "status", header: "Estado", cell: (c) => <StatusBadge tone={c.status === "activo" ? "success" : "danger"} dot label={c.status === "activo" ? "Activo" : "Suspendido"} /> },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (c) => (
        <div className="flex justify-end gap-0.5">
          <Button variant="ghost" size="icon-sm" aria-label="Restablecer contraseña" onClick={() => toast.success(`Enlace de restablecimiento enviado a ${c.email}`)}><KeyRound /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Forzar 2FA" onClick={() => toast.info("OTP de configuración enviado al móvil")}><Smartphone /></Button>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Más" />}><MoreHorizontal /></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem>Editar rol y sedes</DropdownMenuItem>
              <DropdownMenuItem><History /> Ver auditoría</DropdownMenuItem>
              <DropdownMenuItem><LockOpen /> Cerrar sesiones activas</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive">{c.status === "activo" ? "Suspender acceso" : "Reactivar acceso"}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
    },
  ];

  return (
    <Tabs defaultValue="users" className="min-w-0 max-w-full gap-4">
      <PageHeader
        eyebrow="Configuración y seguridad · Usuarios, roles y permisos multisede"
        title="Gobierno de identidad y control de accesos"
        status={<StatusBadge tone="info" dot label="SUNAT OSE tokenized • Multi-tenant RUC 20608941235" />}
        actions={
          <>
            <Button variant="secondary" className="bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted"><ShieldCheck data-icon="inline-start" /> Políticas de clave</Button>
            <Button className="font-semibold" onClick={() => setInviteOpen(true)}><UserPlus data-icon="inline-start" /> Invitar nuevo colaborador</Button>
          </>
        }
      />
      {kpis}
      <TabsList variant="card">
        <TabsTrigger value="users"><Users /> Colaboradores y usuarios ({collaborators.length})</TabsTrigger>
        <TabsTrigger value="roles"><ShieldCheck /> Roles y matriz de permisos ({roles.length} roles)</TabsTrigger>
        <TabsTrigger value="sites"><Building2 /> Sedes y cajas asignadas (3 sedes)</TabsTrigger>
        <TabsTrigger value="audit"><History /> Registro de auditoría (audit log)</TabsTrigger>
      </TabsList>

      <TabsContent value="users">
        <SectionCard title={<span className="sr-only">Colaboradores y usuarios</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
          <Toolbar>
            <SearchInput placeholder="Buscar por nombre o correo" value={query} onChange={setQuery} />
            <Select value={role} onValueChange={(v) => setRole(String(v))} items={roleOptions}>
              <SelectTrigger className="w-56" aria-label="Rol"><SelectValue /></SelectTrigger>
              <SelectContent>{roleOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
            </Select>
          </Toolbar>
          <DataTable
            columns={columns} rows={rows} rowKey={(c) => c.id} minWidth="1100px"
            mobileCard={(c) => (
              <div className="flex items-center gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-mono text-xs font-semibold text-accent-foreground">{initials(c.name)}</span>
                <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{c.name}</p><p className="truncate text-xs text-muted-foreground">{c.role} · {c.sites}</p></div>
                <StatusBadge tone={twoFaMeta[c.twoFa].tone} label={twoFaMeta[c.twoFa].label} />
              </div>
            )}
          />
          <PaginationBar from={1} to={rows.length} total={collaborators.length} label="colaboradores" />
        </SectionCard>
      </TabsContent>

      <TabsContent value="roles">
        <PermissionMatrix roles={roles} matrix={matrix} />
      </TabsContent>

      <InviteCollaboratorSheet open={inviteOpen} onOpenChange={setInviteOpen} />
      <TabsContent value="audit">
        <SectionCard title="Registro de auditoría (últimas 24 h)" contentClassName="p-0">
          <ul className="divide-y text-sm">
            {[
              ["10:42", "Dra. Valeria Mendoza", "Inicio de sesión con Authenticator", "190.237.44.12"],
              ["10:15", "Lucía Espinoza", "Anuló boleta B001-004287 (motivo: error de digitación)", "181.67.112.4"],
              ["09:58", "Sistema", "Certificado digital: recordatorio de renovación (245 días)", "—"],
              ["09:30", "Dr. Miguel Flores", "Intento de acceso a Caja y arqueo denegado por rol", "190.237.44.12"],
              ["08:15", "Lucía Espinoza", "Apertura de turno Caja 01 · S/ 300.00", "181.67.112.4"],
              ["03:12", "Desconocido", "4 intentos fallidos de contraseña · IP bloqueada 15 min", "45.231.9.120"],
            ].map(([t, who, what, ip]) => (
              <li key={t + what} className="flex items-start gap-3 px-4 py-2.5">
                <span className="w-12 shrink-0 font-mono text-xs text-muted-foreground tabular-nums">{t}</span>
                <div className="min-w-0 flex-1"><p><span className="font-medium">{who}</span> · {what}</p></div>
                <span className="shrink-0 font-mono text-xs text-muted-foreground">{ip}</span>
              </li>
            ))}
          </ul>
        </SectionCard>
      </TabsContent>
    </Tabs>
  );
}

function PermissionMatrix({ roles, matrix }: { roles: string[]; matrix: Props["matrix"] }) {
  const [role, setRole] = React.useState(roles[1]);
  const [state, setState] = React.useState(() => matrix);
  const allowed = state[role] ?? {};
  const isSuper = role === roles[0];

  function toggle(mod: string, action: string, on: boolean) {
    setState((s) => {
      const current = new Set(s[role]?.[mod] ?? []);
      if (on) current.add(action as never);
      else current.delete(action as never);
      return { ...s, [role]: { ...s[role], [mod]: permissionActions.filter((a) => current.has(a)) } };
    });
  }

  return (
    <SectionCard
      title="Matriz de permisos granulares"
      action={
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => { setState(matrix); toast.info("Permisos restablecidos a los predefinidos"); }}><RotateCcw data-icon="inline-start" /> Restablecer</Button>
          <Button size="sm" onClick={() => toast.success(`Permisos de "${role}" guardados`)}><Save data-icon="inline-start" /> Guardar matriz</Button>
        </div>
      }
      contentClassName="p-0"
    >
      <div className="flex flex-wrap gap-1.5 border-b p-3">
        {roles.map((r) => (
          <Button key={r} variant={role === r ? "default" : "outline"} onClick={() => setRole(r)}>{r}</Button>
        ))}
      </div>
      <div className="overflow-x-auto">
        <Table className="min-w-[720px]">
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">Módulo</TableHead>
              {permissionActions.map((a) => <TableHead key={a} className="text-center">{a}</TableHead>)}
            </TableRow>
          </TableHeader>
          <TableBody>
            {permissionModules.map((m) => (
              <TableRow key={m} className="hover:bg-muted/30">
                <TableCell className="px-4 py-2 font-medium">{m}</TableCell>
                {permissionActions.map((a) => {
                  const on = isSuper || (allowed[m] ?? []).includes(a);
                  return (
                    <TableCell key={a} className="px-2 py-2 text-center">
                      <Checkbox aria-label={`${m}: ${a}`} checked={on} disabled={isSuper} onCheckedChange={(v) => toggle(m, a, Boolean(v))} className={cn(isSuper && "opacity-60")} />
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {isSuper ? <p className="border-t px-4 py-2 text-xs text-muted-foreground">El rol Super Admin tiene acceso total y no es editable.</p> : null}
    </SectionCard>
  );
}
