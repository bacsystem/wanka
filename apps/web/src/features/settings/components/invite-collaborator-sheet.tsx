"use client";

import { Send, ShieldCheck } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Field } from "@/components/shared/field";
import { cn } from "@/lib/utils";
import { permissionActions, permissionMatrix, permissionModules, roles } from "../mocks/settings";

const roleDescription: Record<string, string> = {
  "Super Admin": "Acceso total a todos los módulos, sedes y configuración de seguridad.",
  "Administrador de sede": "Opera ventas, inventario, caja y reportes de las sedes asignadas.",
  "Cajero POS / Recepción": "Emite comprobantes, registra cobros y gestiona la agenda de su sede.",
  "Médico odontólogo": "Historia clínica, odontograma y órdenes de cobro; sin acceso a caja.",
  "Contador externo / Auditor": "Solo lectura y exportación de comprobantes, compras, kardex y reportes SUNAT.",
};
const sites = ["Miraflores", "San Isidro", "Surquillo"];

export function InviteCollaboratorSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [email, setEmail] = React.useState("");
  const [role, setRole] = React.useState(roles[2]);
  const [selSites, setSelSites] = React.useState<string[]>(["Miraflores"]);
  const [twoFa, setTwoFa] = React.useState(true);
  const matrix = permissionMatrix[role] ?? {};
  function send() {
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) { toast.error("Ingresa un correo corporativo válido"); return; }
    if (!selSites.length) { toast.error("Selecciona al menos una sede"); return; }
    toast.success(`Invitación enviada a ${email}`, { description: `${role} · ${selSites.join(", ")} · válida 48 h` });
    onOpenChange(false); setEmail("");
  }
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader><SheetTitle>Invitar colaborador</SheetTitle><SheetDescription>Recibirá un correo válido 48 h para activar su cuenta y configurar 2FA.</SheetDescription></SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Correo corporativo" className="sm:col-span-2"><Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nombre@clinicasonrisa.pe" /></Field>
          <Field label="Nombres y apellidos"><Input placeholder="Rosa María Flores" /></Field>
          <Field label="Documento"><div className="flex gap-2"><Select defaultValue="DNI" items={[{ value: "DNI", label: "DNI" }, { value: "CE", label: "CE" }]}><SelectTrigger className="w-24"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="DNI">DNI</SelectItem><SelectItem value="CE">CE</SelectItem></SelectContent></Select><Input placeholder="72819402" className="font-mono" /></div></Field>
          <Field label="Cargo"><Input placeholder="Recepción / admisión" /></Field>
          <Field label="Rol"><Select value={role} onValueChange={(v) => setRole(String(v))} items={roles.map((r) => ({ value: r, label: r }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{roles.map((r) => <SelectItem key={r} value={r}>{r}</SelectItem>)}</SelectContent></Select></Field>
          <p className="rounded-md bg-muted/40 p-3 text-xs text-muted-foreground sm:col-span-2">{roleDescription[role]}</p>
          <Field label="Sedes autorizadas" className="sm:col-span-2"><div className="flex flex-wrap gap-2">{sites.map((s) => <label key={s} className={cn("flex cursor-pointer items-center gap-2 rounded-md border px-3 py-1.5 text-sm", selSites.includes(s) && "border-primary bg-accent/40")}><Checkbox checked={selSites.includes(s)} onCheckedChange={(v) => setSelSites((x) => (v ? [...x, s] : x.filter((y) => y !== s)))} /> {s}</label>)}</div></Field>
          <Field label="Cajas asignadas"><Select defaultValue="c1" items={[{ value: "c1", label: "CAJA-01 · Recepción principal" }, { value: "c2", label: "CAJA-02 · Box quirúrgico" }, { value: "none", label: "Sin caja" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="c1">CAJA-01 · Recepción principal</SelectItem><SelectItem value="c2">CAJA-02 · Box quirúrgico</SelectItem><SelectItem value="none">Sin caja</SelectItem></SelectContent></Select></Field>
          <Field label="Horario de acceso permitido"><Input defaultValue="L–V 07:00–21:00" /></Field>
          <div className="flex items-center justify-between rounded-md border p-3 sm:col-span-2"><div><p className="flex items-center gap-1.5 text-sm font-medium"><ShieldCheck className="size-4 text-success" /> Obligar autenticación en dos pasos (2FA)</p><p className="text-xs text-muted-foreground">Requerido para roles con acceso a facturación o datos de pacientes.</p></div><Switch checked={twoFa} onCheckedChange={setTwoFa} aria-label="Obligar 2FA" /></div>
          <div className="sm:col-span-2">
            <p className="mb-1.5 text-xs font-medium text-muted-foreground">Vista previa de permisos resultantes <StatusBadge tone="neutral" label="Solo lectura" className="ml-1" /></p>
            <div className="overflow-x-auto rounded-md border"><Table className="min-w-[520px]"><TableHeader><TableRow><TableHead className="px-3 py-1.5 text-left font-medium">Módulo</TableHead>{permissionActions.map((a) => <TableHead key={a} className="text-center">{a}</TableHead>)}</TableRow></TableHeader><TableBody>{permissionModules.map((m) => <TableRow key={m}><TableCell className="px-3 py-1.5">{m}</TableCell>{permissionActions.map((a) => <TableCell key={a} className="px-2 py-1.5 text-center">{role === roles[0] || (matrix[m] ?? []).includes(a) ? <span className="text-success">●</span> : <span className="text-muted-foreground/40">○</span>}</TableCell>)}</TableRow>)}</TableBody></Table></div>
          </div>
        </div>
        <SheetFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button><Button onClick={send}><Send data-icon="inline-start" /> Enviar invitación</Button></SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
