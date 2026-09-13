"use client";

import { ArrowLeft, ArrowRight, Eye, EyeOff, KeyRound, Lock, Mail, MailCheck, ShieldCheck, Smartphone } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/shared/field";
import { StatusBadge } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import { BrandPanel } from "./brand-panel";

type Step = "login" | "2fa" | "recover";

export function LoginScreen() {
  const router = useRouter();
  const [step, setStep] = React.useState<Step>("login");
  const [email, setEmail] = React.useState("carlos.mendoza@clinicasonrisa.pe");
  const [password, setPassword] = React.useState("");
  const [show, setShow] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  function submitLogin(e: React.FormEvent) {
    e.preventDefault();
    if (!email.includes("@") || password.length < 6) {
      setError("Ingresa un correo válido y una contraseña de al menos 6 caracteres.");
      return;
    }
    setError(null);
    setStep("2fa");
  }

  return (
    <div className="grid min-h-svh lg:grid-cols-[1.1fr_1fr]">
      <BrandPanel />
      <main className="flex flex-col items-center justify-center px-4 py-10">
        <div className="mb-6 flex items-center gap-2 lg:hidden">
          <span className="flex size-9 items-center justify-center rounded-md bg-primary font-mono text-lg font-bold text-primary-foreground">S</span>
          <div><p className="font-semibold leading-tight">Wanka</p><p className="text-xs text-muted-foreground">Gestión empresarial y facturación SUNAT</p></div>
        </div>
        <ol className="mb-4 flex gap-2 text-xs" aria-label="Pasos de acceso">
          {(["login", "2fa", "recover"] as Step[]).map((s, i) => (
            <li key={s} className={cn("rounded-full border px-2.5 py-0.5", step === s ? "border-primary bg-accent text-accent-foreground" : "text-muted-foreground")}>{i + 1}. {s === "login" ? "Ingreso" : s === "2fa" ? "2FA" : "Recuperar"}</li>
          ))}
        </ol>
        <div className="w-full max-w-md rounded-xl border bg-card p-6 shadow-sm">
          {step === "login" ? (
            <form onSubmit={submitLogin} className="flex flex-col gap-4" noValidate>
              <div className="rounded-md border bg-muted/40 px-3 py-2 text-xs"><span className="text-muted-foreground">Espacio de trabajo asignado</span><p className="font-medium">Clínica Dental Sonrisa – Sede Central <StatusBadge className="ml-1" tone="success" dot label="Activo" /></p></div>
              <div><h1 className="text-xl font-semibold">Bienvenido de nuevo</h1><p className="text-sm text-muted-foreground">Ingresa tus credenciales corporativas para abrir la terminal POS y el módulo clínico.</p></div>
              <Field label="Correo electrónico o usuario" htmlFor="email"><div className="relative"><Mail className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" /><Input id="email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} className="pl-8" /></div></Field>
              <Field label="Contraseña" htmlFor="password">
                <div className="relative"><Lock className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" /><Input id="password" type={show ? "text" : "password"} autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} className="pr-10 pl-8" /><button type="button" onClick={() => setShow((v) => !v)} aria-label={show ? "Ocultar contraseña" : "Mostrar contraseña"} className="absolute top-1/2 right-2 -translate-y-1/2 rounded p-1 text-muted-foreground hover:text-foreground">{show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}</button></div>
                <div className="flex justify-end"><button type="button" onClick={() => setStep("recover")} className="text-xs text-primary hover:underline">¿Olvidaste tu contraseña?</button></div>
              </Field>
              {error ? <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{error}</p> : null}
              <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> Recordar mi sesión en este equipo (30 días)</label>
              <Button type="submit" size="lg" className="w-full text-base">Ingresar a Wanka <ArrowRight data-icon="inline-end" /></Button>
              <div className="flex items-center gap-3 text-xs text-muted-foreground"><span className="h-px flex-1 bg-border" />o continuar con<span className="h-px flex-1 bg-border" /></div>
              <Button type="button" variant="outline" className="w-full" onClick={() => toast.info("Redirigiendo a Google Workspace…")}>Continuar con Google Workspace</Button>
              <p className="text-center text-xs text-muted-foreground">¿Tu empresa aún no está registrada? <Link href="/onboarding" className="text-primary hover:underline">Solicitar demo de 14 días</Link></p>
            </form>
          ) : step === "2fa" ? (
            <TwoFactor onBack={() => setStep("login")} onDone={() => router.push("/seleccionar-empresa")} />
          ) : (
            <Recover onBack={() => setStep("login")} email={email} />
          )}
        </div>
        <p className="mt-4 flex items-center gap-1.5 text-[11px] text-muted-foreground"><ShieldCheck className="size-3.5 text-success" /> SUNAT OSE homologado · datos cifrados SSL 256-bit · servidores en Perú</p>
        <p className="mt-6 text-[11px] text-muted-foreground">SUNAT servidor principal: <span className="text-success">operativo (99.98%)</span> · Moneda S/ · v4.18.2-prod</p>
      </main>
    </div>
  );
}

function TwoFactor({ onBack, onDone }: { onBack: () => void; onDone: () => void }) {
  const [digits, setDigits] = React.useState<string[]>(Array(6).fill(""));
  const [seconds, setSeconds] = React.useState(45);
  const refs = React.useRef<(HTMLInputElement | null)[]>([]);
  React.useEffect(() => {
    if (seconds <= 0) return;
    const id = setTimeout(() => setSeconds((s) => s - 1), 1000);
    return () => clearTimeout(id);
  }, [seconds]);
  const code = digits.join("");
  function setAt(i: number, v: string) {
    const d = v.replace(/\D/g, "").slice(-1);
    setDigits((arr) => arr.map((x, j) => (j === i ? d : x)));
    if (d && i < 5) refs.current[i + 1]?.focus();
  }
  function verify() {
    if (code.length < 6) { toast.error("Ingresa los 6 dígitos del código"); return; }
    toast.success("Identidad verificada");
    onDone();
  }
  return (
    <div className="flex flex-col gap-4">
      <button type="button" onClick={onBack} className="flex w-fit items-center gap-1 text-xs text-muted-foreground hover:text-foreground"><ArrowLeft className="size-3.5" /> Volver a ingresar credenciales</button>
      <div className="flex items-start gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground"><Smartphone className="size-5" strokeWidth={1.5} /></span><div><h1 className="text-xl font-semibold">Verificación en dos pasos (2FA)</h1><p className="text-sm text-muted-foreground">Protección obligatoria de facturación y acceso a la base de datos de pacientes.</p></div></div>
      <p className="text-sm">Ingresa el código de 6 dígitos de <span className="font-medium">Google Authenticator</span> o enviado por SMS al <span className="font-mono">+51 987 *** 321</span>.</p>
      <div className="flex justify-between gap-2" role="group" aria-label="Código de autenticación">
        {digits.map((d, i) => (
          <Input key={i} ref={(el) => { refs.current[i] = el; }} value={d} onChange={(e) => setAt(i, e.target.value)} onKeyDown={(e) => { if (e.key === "Backspace" && !d && i > 0) refs.current[i - 1]?.focus(); }} inputMode="numeric" maxLength={1} aria-label={`Dígito ${i + 1}`} className="h-12 w-11 text-center font-mono text-xl" />
        ))}
      </div>
      <div className="flex items-center justify-between text-xs text-muted-foreground"><span>{code.length === 6 ? "Código completo" : "Esperando entrada"} · expira en <span className="font-mono tabular-nums">00:{String(Math.max(0, seconds)).padStart(2, "0")}</span></span><Button variant="link" size="xs" className="px-0" disabled={seconds > 0} onClick={() => { setSeconds(45); toast.success("SMS reenviado"); }}>Reenviar SMS</Button></div>
      <Button size="lg" className="w-full" onClick={verify} disabled={code.length < 6}>Verificar y acceder</Button>
      <p className="text-xs text-muted-foreground"><KeyRound className="mr-1 inline size-3" />¿No tienes acceso a tu teléfono? <button type="button" className="text-primary hover:underline" onClick={() => toast.info("Usa tu llave de seguridad o un código de recuperación")}>Usar llave o código de recuperación</button>. El administrador master puede emitir un token temporal de 1 hora.</p>
    </div>
  );
}

function Recover({ onBack, email }: { onBack: () => void; email: string }) {
  const [sent, setSent] = React.useState(false);
  const [value, setValue] = React.useState(email);
  const masked = value.replace(/^(.)(.*)(.@.*)$/, (_, a, b, c) => a + "*".repeat(Math.min(4, b.length)) + c);
  return (
    <div className="flex flex-col gap-4">
      <button type="button" onClick={onBack} className="flex w-fit items-center gap-1 text-xs text-muted-foreground hover:text-foreground"><ArrowLeft className="size-3.5" /> Volver a inicio de sesión</button>
      <div><h1 className="text-xl font-semibold">Recuperar acceso</h1><p className="text-sm text-muted-foreground">Ingresa tu correo corporativo registrado y te enviaremos las instrucciones de restablecimiento.</p></div>
      {sent ? (
        <div className="flex items-start gap-3 rounded-md border border-success/40 bg-success/10 p-3 text-sm"><MailCheck className="mt-0.5 size-4 shrink-0 text-success" /><div><p className="font-medium">Enlace despachado con éxito</p><p className="text-muted-foreground">Te enviamos un enlace de un solo uso válido por 15 minutos a <span className="font-mono">{masked}</span>. Revisa tu bandeja de entrada o spam.</p></div></div>
      ) : null}
      <Field label="Correo electrónico registrado" htmlFor="recover-email" help="Debe coincidir con la cuenta asignada a tu usuario."><Input id="recover-email" type="email" value={value} onChange={(e) => setValue(e.target.value)} className="h-10" /></Field>
      <Button size="lg" className="w-full" onClick={() => { setSent(true); toast.success("Correo de restablecimiento enviado"); }}>{sent ? "Reenviar correo de restablecimiento" : "Enviar enlace de restablecimiento"}</Button>
      <p className="text-xs text-muted-foreground">Si no recibes el mensaje en 5 minutos, contacta a la mesa de ayuda: (01) 708-9000 anexo 4.</p>
    </div>
  );
}
