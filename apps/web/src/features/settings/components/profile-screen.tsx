"use client";

import { Check, KeyRound, Laptop, LogOut, Monitor, Moon, ShieldCheck, Smartphone, Sun } from "lucide-react";
import * as React from "react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";

const sessions = [
  { device: "MacBook Pro 14”", browser: "Chrome 131 · macOS", ip: "190.237.44.12", site: "Sede Miraflores", last: "Ahora", current: true, icon: Laptop },
  { device: "iPhone 15", browser: "Safari · iOS 18", ip: "181.67.112.4", site: "Móvil", last: "Hoy 07:58", current: false, icon: Smartphone },
  { device: "Terminal POS-01", browser: "Edge 130 · Windows 11", ip: "190.237.44.12", site: "Sede Miraflores · Caja 01", last: "Hoy 08:30", current: false, icon: Monitor },
];
const activity = [["Hoy 10:42", "Inicio de sesión con Authenticator", "190.237.44.12"], ["Hoy 10:20", "Aprobó cotización COT-2026-00128", "190.237.44.12"], ["Hoy 09:15", "Emitió factura F001-00000842", "190.237.44.12"], ["Ayer 18:40", "Cambió rol de colaborador (M. Guerrero)", "190.237.44.12"], ["Ayer 12:05", "Exportó RVIE 2026-08", "190.237.44.12"], ["10/09 16:20", "Actualizó certificado digital", "190.237.44.12"]];

function strength(pw: string) { let s = 0; if (pw.length >= 10) s += 30; if (/[A-Z]/.test(pw) && /\d/.test(pw)) s += 25; if (/[^A-Za-z0-9]/.test(pw)) s += 25; if (pw.length >= 14) s += 20; return Math.min(100, s); }

export function ProfileScreen() {
  const { theme = "system", setTheme } = useTheme();
  const [tab, setTab] = React.useState("datos");
  const [pw, setPw] = React.useState("");
  const [notif, setNotif] = React.useState({ sunat: { mail: true, wa: true, push: true }, stock: { mail: false, wa: true, push: true }, cobranzas: { mail: true, wa: false, push: true } });
  const sc = strength(pw);
  return (
    <>
      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList><TabsTrigger value="datos">Datos y seguridad</TabsTrigger><TabsTrigger value="sesiones">Sesiones activas <span className="ml-1 font-mono text-[11px] opacity-70">{sessions.length}</span></TabsTrigger><TabsTrigger value="prefs">Preferencias y notificaciones</TabsTrigger><TabsTrigger value="actividad">Actividad reciente</TabsTrigger></TabsList></Tabs>
      {tab === "datos" ? (
        <div className="grid gap-4 xl:grid-cols-2">
          <SectionCard title="Información de contacto profesional" contentClassName="grid gap-4 p-4 sm:grid-cols-2">
            <Field label="Nombres completos" className="sm:col-span-2"><Input defaultValue="Carlos Alberto Mendoza Ruiz" /></Field>
            <Field label="Correo corporativo"><Input defaultValue="carlos.mendoza@clinicasonrisa.pe" /></Field>
            <Field label="Celular / WhatsApp para alertas"><Input defaultValue="+51 987 654 321" /></Field>
            <Field label="Documento (RENIEC verificado)"><Input defaultValue="DNI 10721098" readOnly className="font-mono" /></Field>
            <Field label="Colegiatura (COP)"><Input defaultValue="34291" className="font-mono" /></Field>
            <Field label="Idioma"><Input readOnly value="Español (Perú)" /></Field><Field label="Zona horaria"><Input readOnly value="America/Lima (UTC−5)" /></Field>
            <div className="flex justify-end sm:col-span-2"><Button onClick={() => toast.success("Perfil actualizado")}>Guardar cambios</Button></div>
          </SectionCard>
          <div className="flex min-w-0 flex-col gap-4">
            <SectionCard title="Seguridad de la contraseña" icon={KeyRound} action={<span className="text-xs text-muted-foreground">Último cambio hace 42 días</span>} contentClassName="grid gap-3 p-4">
              <Field label="Contraseña actual"><Input type="password" /></Field>
              <Field label="Nueva contraseña"><Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} /></Field>
              <div><div className="flex justify-between text-xs"><span className="text-muted-foreground">Fortaleza</span><span className={cn("font-medium", sc >= 75 ? "text-success" : sc >= 50 ? "text-warning" : "text-destructive")}>{sc >= 75 ? "Muy fuerte" : sc >= 50 ? "Aceptable" : "Débil"} ({sc}/100)</span></div><Progress value={sc} className="mt-1" /></div>
              <ul className="grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">{[["Mínimo 10 caracteres", pw.length >= 10], ["Números y mayúsculas", /[A-Z]/.test(pw) && /\d/.test(pw)], ["Carácter especial", /[^A-Za-z0-9]/.test(pw)], ["Sin palabras comunes ni RUC", !/20608941235|clinica|sonrisa/i.test(pw)]].map(([l, ok]) => <li key={String(l)} className={cn("flex items-center gap-1", ok && "text-success")}><Check className="size-3" /> {l}</li>)}</ul>
              <Field label="Confirmar nueva contraseña"><Input type="password" /></Field>
              <div className="flex items-center justify-between"><label className="flex items-center gap-2 text-sm"><Switch defaultChecked aria-label="Exigir cambio cada 90 días" /> Exigir cambio cada 90 días</label><Button disabled={sc < 50} onClick={() => { toast.success("Contraseña actualizada"); setPw(""); }}>Actualizar contraseña</Button></div>
            </SectionCard>
            <SectionCard title="Autenticación en dos pasos" icon={ShieldCheck} action={<StatusBadge tone="success" dot label="2FA activo" />} contentClassName="grid gap-2 p-4 text-sm">
              <div className="flex items-center justify-between rounded-md border p-3"><div><p className="font-medium">Google Authenticator</p><p className="text-xs text-muted-foreground">Configurado el 14/03/2026 · códigos de respaldo: 8 disponibles</p></div><Button size="sm" variant="outline" onClick={() => toast.info("Códigos de respaldo descargados")}>Ver códigos</Button></div>
              <div className="flex items-center justify-between rounded-md border p-3"><div><p className="font-medium">Llave de seguridad (FIDO2)</p><p className="text-xs text-muted-foreground">Sin llaves registradas</p></div><Button size="sm" variant="outline">Registrar llave</Button></div>
            </SectionCard>
          </div>
        </div>
      ) : null}
      {tab === "sesiones" ? (
        <SectionCard title="Sesiones activas" action={<Button size="sm" variant="outline" className="text-destructive" onClick={() => toast.success("Se cerraron las otras sesiones")}><LogOut data-icon="inline-start" /> Cerrar las demás</Button>} contentClassName="p-0">
          <ul className="divide-y">{sessions.map((s) => <li key={s.device} className="flex items-center gap-3 px-4 py-3 text-sm"><span className="flex size-9 items-center justify-center rounded-md bg-muted"><s.icon className="size-4" strokeWidth={1.5} /></span><div className="min-w-0 flex-1"><p className="font-medium">{s.device} {s.current ? <StatusBadge tone="success" dot label="Sesión actual" className="ml-1" /> : null}</p><p className="text-xs text-muted-foreground">{s.browser} · IP {s.ip} · {s.site} · {s.last}</p></div>{!s.current ? <Button size="xs" variant="outline" onClick={() => toast.success(`Sesión en ${s.device} cerrada`)}>Cerrar</Button> : null}</li>)}</ul>
        </SectionCard>
      ) : null}
      {tab === "prefs" ? (
        <div className="grid gap-4 xl:grid-cols-2">
          <SectionCard title="Apariencia" contentClassName="grid gap-3 p-4">
            <Field label="Tema"><div className="grid grid-cols-3 gap-2">{([["light", "Claro", Sun], ["dark", "Oscuro", Moon], ["system", "Sistema", Monitor]] as const).map(([v, l, I]) => <button key={v} type="button" onClick={() => setTheme(v)} aria-pressed={theme === v} className={cn("flex items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm", theme === v && "border-primary bg-accent text-accent-foreground")}><I className="size-4" /> {l}</button>)}</div></Field>
            <Field label="Densidad de tablas"><div className="grid grid-cols-2 gap-2"><button type="button" className="rounded-md border border-primary bg-accent px-3 py-2 text-sm" aria-pressed>Compacta</button><button type="button" className="rounded-md border px-3 py-2 text-sm">Cómoda</button></div></Field>
          </SectionCard>
          <SectionCard title="Notificaciones por canal" contentClassName="p-0">
            <Table><TableHeader><TableRow><TableHead className="pl-4">Tipo</TableHead><TableHead className="text-center">Correo</TableHead><TableHead className="text-center">WhatsApp</TableHead><TableHead className="text-center">Push</TableHead></TableRow></TableHeader><TableBody>{(Object.keys(notif) as (keyof typeof notif)[]).map((k) => <TableRow key={k}><TableCell className="px-4 py-2 font-medium capitalize">{k === "sunat" ? "SUNAT / OSE" : k}</TableCell>{(["mail", "wa", "push"] as const).map((c) => <TableCell key={c} className="px-2 py-2 text-center"><Switch checked={notif[k][c]} onCheckedChange={(v) => setNotif({ ...notif, [k]: { ...notif[k], [c]: v } })} aria-label={`${k} ${c}`} /></TableCell>)}</TableRow>)}</TableBody></Table>
          </SectionCard>
        </div>
      ) : null}
      {tab === "actividad" ? (
        <SectionCard title="Actividad reciente" contentClassName="p-0"><ul className="divide-y text-sm">{activity.map(([t, w, ip]) => <li key={t + w} className="flex items-center gap-3 px-4 py-2.5"><span className="w-24 shrink-0 font-mono text-xs text-muted-foreground">{t}</span><span className="flex-1">{w}</span><span className="font-mono text-xs text-muted-foreground">{ip}</span></li>)}</ul></SectionCard>
      ) : null}
    </>
  );
}
