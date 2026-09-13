"use client";

import { ArrowLeft, ArrowRight, Building2, Check, FileBadge2, Headset, KeyRound, ListOrdered, Lock, Rocket, Store, Users, Warehouse, Zap } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Field } from "@/components/shared/field";
import { StatusBadge } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import type { Industry } from "@/types/domain";
import { industryOptions, modulesByIndustry } from "../mocks/tenants";

const steps = [
  { id: 1, label: "Empresa y RUC", icon: Building2 },
  { id: 2, label: "Rubro y módulos", icon: Store },
  { id: 3, label: "Certificado SUNAT", icon: FileBadge2 },
  { id: 4, label: "Sedes y almacenes", icon: Warehouse },
  { id: 5, label: "Series de facturación", icon: ListOrdered },
  { id: 6, label: "Usuarios y accesos", icon: Users },
];

export function OnboardingScreen() {
  const router = useRouter();
  const [step, setStep] = React.useState(2);
  const [company, setCompany] = React.useState({ ruc: "20608941235", name: "Clínica Dental Sonrisa S.A.C.", regime: "Régimen MYPE Tributario (RMT)", address: "Av. José Larco 850, Piso 3, Miraflores, Lima", validated: true });
  const [industry, setIndustry] = React.useState<Industry>("odontologia");
  const [modules, setModules] = React.useState(() => Object.fromEntries(modulesByIndustry.odontologia.map((m) => [m.id, m.on])));
  const [env, setEnv] = React.useState<"beta" | "produccion">("beta");
  const [series, setSeries] = React.useState([{ type: "Factura", s: "F001", n: "000001" }, { type: "Boleta", s: "B001", n: "000001" }, { type: "Nota de crédito", s: "FC01", n: "000001" }, { type: "Guía de remisión", s: "T001", n: "000001" }]);
  const [sites, setSites] = React.useState([{ name: "Sede principal", address: company.address, warehouse: "Almacén central" }]);
  const [users, setUsers] = React.useState([{ email: "carlos.mendoza@clinicasonrisa.pe", role: "Super Admin" }]);
  const mods = modulesByIndustry[industry];
  const activeCount = mods.filter((m) => modules[m.id] ?? m.on).length;

  function chooseIndustry(id: Industry) {
    setIndustry(id);
    setModules(Object.fromEntries(modulesByIndustry[id].map((m) => [m.id, m.on])));
  }
  function next() {
    if (step === 1 && !company.validated) { toast.error("Valida el RUC en SUNAT antes de continuar"); return; }
    if (step < 6) setStep(step + 1);
    else { toast.success("Empresa activada · bienvenido a Wanka"); router.push("/seleccionar-empresa"); }
  }
  const progress = Math.round(((step - 1) / 6) * 100);

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 px-4 py-8">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-md bg-primary text-primary-foreground"><Rocket className="size-5" /></span><div><p className="text-xs font-medium text-primary uppercase">Asistente de activación rápida · paso {step} de 6</p><h1 className="text-xl font-semibold tracking-tight">Configura tu espacio de trabajo para emisión electrónica SUNAT</h1></div></div>
        <Button variant="outline" size="sm" onClick={() => toast.info("Soporte especializado: (01) 708-9000 anexo 4")}><Headset data-icon="inline-start" /> Soporte especializado</Button>
      </header>

      <div className="grid gap-6 lg:grid-cols-[14rem_1fr_18rem] [&>*]:min-w-0">
        <ol className="flex gap-2 overflow-x-auto lg:flex-col" aria-label="Pasos">
          {steps.map((s) => {
            const done = s.id < step; const active = s.id === step;
            return (
              <li key={s.id} className="shrink-0">
                <button type="button" onClick={() => s.id <= step && setStep(s.id)} disabled={s.id > step} className={cn("flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm", active && "border-primary bg-accent text-accent-foreground", done && "border-success/40 text-success", !active && !done && "text-muted-foreground")}>
                  <span className={cn("flex size-6 shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold", active && "border-primary bg-primary text-primary-foreground", done && "border-success bg-success text-success-foreground")}>{done ? <Check className="size-3" /> : s.id}</span>
                  <span className="whitespace-nowrap">{s.label}</span>
                </button>
              </li>
            );
          })}
        </ol>

        <section className="rounded-xl border bg-card p-5">
          {step === 1 ? (
            <div className="flex flex-col gap-4">
              <div><h2 className="text-lg font-semibold">Datos de la empresa</h2><p className="text-sm text-muted-foreground">Consultamos el padrón SUNAT para completar tu razón social, régimen y domicilio fiscal.</p></div>
              <Field label="RUC (11 dígitos)"><div className="flex gap-2"><Input value={company.ruc} onChange={(e) => setCompany({ ...company, ruc: e.target.value.replace(/\D/g, "").slice(0, 11), validated: false })} className="font-mono" inputMode="numeric" /><Button type="button" variant="outline" onClick={() => { if (company.ruc.length !== 11) { toast.error("El RUC debe tener 11 dígitos"); return; } setCompany({ ...company, validated: true }); toast.success("SUNAT: contribuyente habido y activo"); }}><Zap data-icon="inline-start" /> Validar en SUNAT</Button></div></Field>
              {company.validated ? <StatusBadge tone="success" icon={Check} label="Validado · habido / activo" className="w-fit" /> : null}
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Razón social" className="sm:col-span-2"><Input value={company.name} onChange={(e) => setCompany({ ...company, name: e.target.value })} /></Field>
                <Field label="Régimen tributario"><Input value={company.regime} readOnly /></Field>
                <Field label="Nombre comercial"><Input placeholder="Clínica Sonrisa" /></Field>
                <Field label="Domicilio fiscal" className="sm:col-span-2"><Input value={company.address} onChange={(e) => setCompany({ ...company, address: e.target.value })} /></Field>
              </div>
            </div>
          ) : step === 2 ? (
            <div className="flex flex-col gap-5">
              <div><p className="text-xs font-medium text-primary uppercase">Personalización económica</p><h2 className="text-lg font-semibold">¿A qué se dedica tu empresa?</h2><p className="text-sm text-muted-foreground">Personalizaremos flujos de caja, terminología, catálogos predeterminados e integraciones según tu actividad principal.</p></div>
              <ul className="grid gap-3 sm:grid-cols-2">
                {industryOptions.map((o) => (
                  <li key={o.id}>
                    <button type="button" onClick={() => chooseIndustry(o.id)} aria-pressed={industry === o.id} className={cn("flex h-full w-full flex-col gap-2 rounded-lg border p-4 text-left hover:border-primary/50", industry === o.id && "border-primary bg-accent/40 ring-2 ring-primary/20")}>
                      <div className="flex items-center justify-between gap-2"><span className="font-semibold">{o.label}</span>{o.recommended ? <StatusBadge tone="success" label="Recomendado" /> : null}{industry === o.id ? <Check className="size-4 text-primary" /> : null}</div>
                      <p className="text-xs text-muted-foreground">{o.description}</p>
                      <span className="mt-auto text-xs font-medium text-primary">{o.tagline} →</span>
                    </button>
                  </li>
                ))}
              </ul>
              <div>
                <div className="flex items-center justify-between"><h3 className="text-sm font-semibold">Módulos sugeridos para {industryOptions.find((o) => o.id === industry)?.label}</h3><span className="text-xs text-muted-foreground">{activeCount} activos / {mods.length - activeCount} pausados</span></div>
                <ul className="mt-2 divide-y rounded-md border">
                  {mods.map((m) => {
                    const on = modules[m.id] ?? m.on;
                    return (
                      <li key={m.id} className="flex items-center gap-3 px-3 py-2.5">
                        <div className="min-w-0 flex-1"><p className="flex flex-wrap items-center gap-2 text-sm font-medium">{m.name}{m.tag ? <StatusBadge tone={on ? "info" : "neutral"} label={m.tag} /> : null}</p><p className="text-xs text-muted-foreground">{m.description}</p></div>
                        <Switch checked={on} onCheckedChange={(v) => setModules({ ...modules, [m.id]: v })} aria-label={m.name} />
                      </li>
                    );
                  })}
                </ul>
              </div>
            </div>
          ) : step === 3 ? (
            <div className="flex flex-col gap-4">
              <div><h2 className="text-lg font-semibold">Certificado digital y Clave SOL</h2><p className="text-sm text-muted-foreground">Necesario para firmar los XML UBL 2.1 y transmitir a SUNAT u OSE.</p></div>
              <label className="flex cursor-pointer flex-col items-center gap-1 rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground hover:bg-muted/40"><FileBadge2 className="size-6" />Arrastra tu certificado <span className="font-mono">.pfx</span> / <span className="font-mono">.p12</span> o <span className="text-primary underline">examina tu equipo</span><input type="file" accept=".pfx,.p12" className="sr-only" onChange={() => toast.success("Certificado cargado · vigente hasta 14/05/2027")} /></label>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Contraseña del certificado"><Input type="password" /></Field>
                <Field label="Usuario secundario Clave SOL" help="Con perfil de facturador electrónico."><Input placeholder="MODDATOS" className="font-mono" /></Field>
                <Field label="Contraseña Clave SOL"><Input type="password" /></Field>
                <Field label="Canal de emisión"><Select defaultValue="ose" items={[{ value: "ose", label: "OSE homologado (Digiflow / Bizlinks)" }, { value: "sunat", label: "SUNAT directo" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ose">OSE homologado (Digiflow / Bizlinks)</SelectItem><SelectItem value="sunat">SUNAT directo</SelectItem></SelectContent></Select></Field>
              </div>
              <Field label="Entorno"><div className="grid grid-cols-2 gap-2">{([["beta", "Beta / pruebas"], ["produccion", "Producción oficial"]] as const).map(([v, l]) => <button key={v} type="button" onClick={() => setEnv(v)} aria-pressed={env === v} className={cn("rounded-md border px-3 py-2 text-sm font-medium", env === v && (v === "produccion" ? "border-success bg-success/10 text-success" : "border-warning bg-warning/10 text-warning"))}>{l}</button>)}</div></Field>
              <Button variant="outline" className="w-fit" onClick={() => toast.success("Conexión exitosa · handshake 238 ms")}><KeyRound data-icon="inline-start" /> Probar conexión</Button>
            </div>
          ) : step === 4 ? (
            <div className="flex flex-col gap-4">
              <div><h2 className="text-lg font-semibold">Sedes y almacenes</h2><p className="text-sm text-muted-foreground">Cada sede se vincula a un establecimiento anexo SUNAT y a uno o más almacenes.</p></div>
              <ul className="flex flex-col gap-3">
                {sites.map((s, i) => (
                  <li key={i} className="grid gap-3 rounded-md border p-3 sm:grid-cols-3">
                    <Field label="Nombre de la sede"><Input value={s.name} onChange={(e) => setSites(sites.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))} /></Field>
                    <Field label="Dirección"><Input value={s.address} onChange={(e) => setSites(sites.map((x, j) => (j === i ? { ...x, address: e.target.value } : x)))} /></Field>
                    <Field label="Almacén principal"><Input value={s.warehouse} onChange={(e) => setSites(sites.map((x, j) => (j === i ? { ...x, warehouse: e.target.value } : x)))} /></Field>
                  </li>
                ))}
              </ul>
              <Button variant="outline" className="w-fit" onClick={() => setSites([...sites, { name: `Sede ${sites.length + 1}`, address: "", warehouse: "Subalmacén" }])}>+ Agregar sede</Button>
            </div>
          ) : step === 5 ? (
            <div className="flex flex-col gap-4">
              <div><h2 className="text-lg font-semibold">Series de comprobantes por sede</h2><p className="text-sm text-muted-foreground">Ajusta el correlativo inicial si migras desde otro software.</p></div>
              <ul className="divide-y rounded-md border">
                {series.map((s, i) => (
                  <li key={s.type} className="grid grid-cols-[1fr_5rem_6rem] items-center gap-2 px-3 py-2 text-sm"><span className="font-medium">{s.type}</span><Input value={s.s} onChange={(e) => setSeries(series.map((x, j) => (j === i ? { ...x, s: e.target.value.toUpperCase().slice(0, 4) } : x)))} className="font-mono uppercase" aria-label={`Serie ${s.type}`} /><Input value={s.n} onChange={(e) => setSeries(series.map((x, j) => (j === i ? { ...x, n: e.target.value.replace(/\D/g, "").slice(0, 8) } : x)))} className="font-mono" aria-label={`Correlativo ${s.type}`} /></li>
                ))}
              </ul>
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              <div><h2 className="text-lg font-semibold">Usuarios y accesos</h2><p className="text-sm text-muted-foreground">Invita a tu equipo; cada usuario recibirá un correo para activar 2FA.</p></div>
              <ul className="divide-y rounded-md border">
                {users.map((u, i) => (
                  <li key={i} className="grid gap-2 px-3 py-2 sm:grid-cols-[1fr_14rem]"><Input value={u.email} onChange={(e) => setUsers(users.map((x, j) => (j === i ? { ...x, email: e.target.value } : x)))} className="h-8" aria-label="Correo" /><Select value={u.role} onValueChange={(v) => setUsers(users.map((x, j) => (j === i ? { ...x, role: String(v) } : x)))} items={["Super Admin", "Administrador de sede", "Cajero POS / Recepción", "Médico odontólogo", "Contador externo / Auditor"].map((r) => ({ value: r, label: r }))}><SelectTrigger className="w-full" aria-label="Rol"><SelectValue /></SelectTrigger><SelectContent>{["Super Admin", "Administrador de sede", "Cajero POS / Recepción", "Médico odontólogo", "Contador externo / Auditor"].map((r) => <SelectItem key={r} value={r}>{r}</SelectItem>)}</SelectContent></Select></li>
                ))}
              </ul>
              <Button variant="outline" className="w-fit" onClick={() => setUsers([...users, { email: "", role: "Cajero POS / Recepción" }])}>+ Invitar colaborador</Button>
            </div>
          )}
          <div className="mt-6 flex items-center justify-between border-t pt-4">
            <Button variant="ghost" onClick={() => setStep(Math.max(1, step - 1))} disabled={step === 1}><ArrowLeft data-icon="inline-start" /> Regresar</Button>
            <Button onClick={next}>{step < 6 ? `Guardar y continuar al paso ${step + 1}` : "Activar empresa"} <ArrowRight data-icon="inline-end" /></Button>
          </div>
        </section>

        <aside className="flex flex-col gap-4">
          <div className="rounded-xl border bg-card p-4 text-sm">
            <div className="flex items-center justify-between"><h3 className="font-semibold">Resumen de empresa</h3>{company.validated ? <StatusBadge tone="success" label="Paso 1 validado" /> : <StatusBadge tone="warning" label="Sin validar" />}</div>
            <dl className="mt-3 flex flex-col gap-2 text-xs">
              <div><dt className="text-muted-foreground">Razón social</dt><dd className="font-medium">{company.name}</dd></div>
              <div><dt className="text-muted-foreground">RUC</dt><dd className="font-mono">{company.ruc}</dd></div>
              <div><dt className="text-muted-foreground">Régimen</dt><dd>{company.regime}</dd></div>
              <div><dt className="text-muted-foreground">Domicilio fiscal</dt><dd>{company.address}</dd></div>
              <div><dt className="text-muted-foreground">Rubro</dt><dd>{industryOptions.find((o) => o.id === industry)?.label} · {activeCount} módulos</dd></div>
            </dl>
            <div className="mt-3"><div className="flex justify-between text-xs"><span className="text-muted-foreground">Avance general</span><span className="font-mono">{progress}% ({step - 1} de 6)</span></div><Progress value={progress} className="mt-1" /></div>
          </div>
          <div className="rounded-xl border bg-muted/30 p-4 text-xs text-muted-foreground">
            <p className="font-medium text-foreground">Vistas previas de siguientes pasos</p>
            <p className="mt-2"><span className="font-medium text-foreground">Paso 3:</span> archivo .pfx/.p12 y contraseña, usuario SOL con permisos CPE, entorno beta/producción.</p>
            <p className="mt-2"><span className="font-medium text-foreground">Paso 5:</span> series propuestas F001 · B001 · FC01 · T001 desde 000001.</p>
            <p className="mt-3 flex items-center gap-1"><Lock className="size-3" /> Claves privadas cifradas AES-256 en reposo.</p>
          </div>
          <p className="text-center text-xs text-muted-foreground">¿Ya tienes cuenta? <Link href="/login" className="text-primary hover:underline">Iniciar sesión</Link></p>
        </aside>
      </div>
    </div>
  );
}
