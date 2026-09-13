"use client";

import { ArrowRight, Fingerprint, KeyRound, Layers, Lock, ShieldAlert, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Field } from "@/components/shared/field";
import { StateIcon } from "@/components/shared/dynamic-icon";

export function AdminLoginScreen() {
  const router = useRouter();
  const [step, setStep] = React.useState<"sso" | "2fa">("sso");
  const [code, setCode] = React.useState("");
  return (
    <div className="theme-admin grid min-h-svh bg-background text-foreground lg:grid-cols-2">
      <aside className="relative hidden flex-col justify-between overflow-hidden bg-brand-panel p-10 text-white lg:flex">
        <div className="pointer-events-none absolute -top-24 -right-24 size-96 rounded-full bg-indigo-500/30 blur-3xl" />
        <div>
          <div className="flex items-center gap-3"><span className="flex size-11 items-center justify-center rounded-xl bg-indigo-500"><Layers className="size-5" /></span><div><p className="flex items-center gap-2 text-xl font-bold">Wanka <span className="rounded bg-white/15 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide">ADMIN</span></p><p className="text-xs text-white/70">Consola de administración de la plataforma</p></div></div>
          <h1 className="mt-12 max-w-md text-3xl font-bold tracking-tight">Operación central de 1,482 empresas, 4 conectores OSE y facturación del SaaS.</h1>
          <p className="mt-3 max-w-md text-sm text-white/70">Acceso exclusivo para personal interno con rol asignado. Cada sesión queda registrada con IP, dispositivo y auditoría SIEM.</p>
          <ul className="mt-8 flex flex-col gap-3 text-sm">{[["SSO Google Workspace", "Dominio @perusaas.pe verificado", ShieldCheck], ["2FA obligatorio", "TOTP o llave FIDO2 en cada inicio", KeyRound], ["Impersonación auditada", "Sesiones de soporte con expiración de 30 min", Fingerprint]].map(([t, d, I]) => { const Icon = I as typeof ShieldCheck; return <li key={String(t)} className="flex items-start gap-3 rounded-xl bg-white/10 p-3"><span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-white/15"><Icon className="size-4" /></span><span><span className="block font-semibold">{String(t)}</span><span className="text-xs text-white/70">{String(d)}</span></span></li>; })}</ul>
        </div>
        <p className="text-[11px] text-white/50">Cluster prod-lima-01 · SLA 99.98% · ISO/IEC 27001 · Lima, Perú</p>
      </aside>
      <main className="relative flex flex-col items-center justify-center p-6">
        <div className="absolute top-4 right-4"><ThemeToggle /></div>
        <div className="w-full max-w-md rounded-2xl border bg-card p-8 shadow-sm">
          <div className="mb-6 flex items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200"><ShieldAlert className="size-4 shrink-0" /> Acceso restringido. Los intentos no autorizados se registran con IP <span className="font-mono">190.237.44.12</span> y se reportan al equipo de seguridad.</div>
          {step === "sso" ? (
            <>
              <h2 className="text-2xl font-bold tracking-tight">Ingreso a la consola</h2>
              <p className="mt-1 text-sm text-muted-foreground">Usa tu cuenta corporativa de Google Workspace. No existe registro público.</p>
              <Button size="lg" className="mt-6 w-full font-semibold" onClick={() => { toast.success("SSO verificado · dominio @perusaas.pe"); setStep("2fa"); }}><Lock data-icon="inline-start" /> Continuar con Google Workspace <ArrowRight data-icon="inline-end" /></Button>
              <div className="my-5 flex items-center gap-3 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase"><span className="h-px flex-1 bg-border" /> o con credenciales de emergencia <span className="h-px flex-1 bg-border" /></div>
              <div className="grid gap-4"><Field label="Correo corporativo"><Input type="email" placeholder="usuario@perusaas.pe" /></Field><Field label="Contraseña"><Input type="password" placeholder="••••••••••••" /></Field><Button variant="outline" className="font-semibold" onClick={() => setStep("2fa")}>Ingresar</Button></div>
            </>
          ) : (
            <>
              <h2 className="flex items-center gap-2 text-2xl font-bold tracking-tight"><StateIcon state="syncing" className="size-5" /> Verificación en dos pasos</h2>
              <p className="mt-1 text-sm text-muted-foreground">Ingresa el código de 6 dígitos de tu aplicación autenticadora o toca tu llave FIDO2.</p>
              <div className="mt-6 grid gap-4">
                <Field label="Código TOTP"><Input value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} inputMode="numeric" placeholder="000 000" className="text-center font-mono text-xl tracking-[0.4em]" /></Field>
                <Button size="lg" className="w-full font-semibold" disabled={code.length !== 6} onClick={() => { toast.success("Sesión iniciada · Admin principal"); router.push("/admin"); }}><ShieldCheck data-icon="inline-start" /> Verificar e ingresar</Button>
                <Button variant="ghost" className="font-semibold" onClick={() => toast.info("Esperando llave FIDO2…")}><Fingerprint data-icon="inline-start" /> Usar llave de seguridad</Button>
              </div>
            </>
          )}
          <p className="mt-6 text-center text-[11px] text-muted-foreground">¿Eres una empresa cliente? <Link href="/login" className="font-semibold text-primary hover:underline">Ingresa a la app de empresa</Link></p>
        </div>
      </main>
    </div>
  );
}
