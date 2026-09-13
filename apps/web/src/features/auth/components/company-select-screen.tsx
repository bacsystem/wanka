"use client";

import { ArrowRight, Building2, Check, Eye, KeyRound, LockOpen, LogOut, MapPin, PlusCircle, Wifi } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { SITE_COOKIE, TENANT_COOKIE, setClientCookie } from "@/features/auth/lib/tenant-cookie";
import { initials } from "@/lib/tenant";
import { cn } from "@/lib/utils";
import type { TenantOption } from "../mocks/tenants";
import { tenantSites } from "../mocks/tenants";

export function CompanySelectScreen({ tenants, currentId }: { tenants: TenantOption[]; currentId?: string }) {
  const router = useRouter();
  const [tenantId, setTenantId] = React.useState<string>(currentId ?? tenants[0].id);
  const sites = tenantSites[tenantId] ?? [];
  const [siteId, setSiteId] = React.useState(sites[0]?.id);
  const [cashId, setCashId] = React.useState<string>(sites[0]?.cashboxes[0]?.id ?? "auditor");
  const site = sites.find((s) => s.id === siteId) ?? sites[0];
  const step2 = React.useRef<HTMLDivElement>(null);

  function pick(id: string) {
    setTenantId(id);
    const first = tenantSites[id]?.[0];
    setSiteId(first?.id);
    setCashId(first?.cashboxes[0]?.id ?? "auditor");
    setTimeout(() => step2.current?.scrollIntoView({ behavior: "smooth", block: "start" }), 50);
  }

  function go() {
    setClientCookie(TENANT_COOKIE, tenantId);
    if (site) setClientCookie(SITE_COOKIE, site.id);
    const t = tenants.find((x) => x.id === tenantId)!;
    toast.success(`Sesión iniciada en ${t.name}`, { description: `${site?.name ?? ""} · ${cashId === "auditor" ? "modo consulta" : "caja asignada"}` });
    router.push("/");
    router.refresh();
  }

  return (
    <main className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 px-4 py-6 md:px-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2"><span className="flex size-9 items-center justify-center rounded-md bg-primary font-mono text-lg font-bold text-primary-foreground">S</span><div><p className="font-semibold leading-tight">Wanka</p><p className="text-xs text-muted-foreground">Acceso seguro SSL · autenticación 2FA activa</p></div></div>
        <div className="flex items-center gap-2 text-sm"><span className="flex size-8 items-center justify-center rounded-full bg-accent font-mono text-xs font-semibold text-accent-foreground">CM</span><div className="hidden sm:block"><p className="font-medium leading-tight">Carlos Mendoza</p><p className="text-xs text-muted-foreground">carlos.mendoza@clinicasonrisa.pe</p></div><Button variant="ghost" size="icon-sm" aria-label="Cerrar sesión" render={<Link href="/login" />} nativeButton={false}><LogOut /></Button></div>
      </header>

      <section aria-labelledby="step1">
        <p className="text-xs font-medium text-primary uppercase">Paso 1 de 2 · Entidad jurídica</p>
        <div className="flex flex-wrap items-end justify-between gap-2">
          <div><h1 id="step1" className="text-2xl font-semibold tracking-tight">Hola, Carlos · elige con qué empresa trabajar</h1><p className="text-sm text-muted-foreground">Tienes acceso a {tenants.length} organizaciones vinculadas a tu DNI y correo corporativo.</p></div>
          <Button variant="outline" render={<Link href="/onboarding" />} nativeButton={false}><PlusCircle data-icon="inline-start" /> Registrar nueva empresa (RUC)</Button>
        </div>
        <ul className="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {tenants.map((t) => {
            const active = t.id === tenantId;
            return (
              <li key={t.id}>
                <button type="button" onClick={() => pick(t.id)} aria-pressed={active} className={cn("flex h-full w-full flex-col gap-3 rounded-xl border bg-card p-4 text-left transition-colors hover:border-primary/50", active && "border-primary ring-2 ring-primary/20")}>
                  <div className="flex items-start justify-between gap-2">
                    <span className="flex size-12 items-center justify-center rounded-xl bg-primary text-sm font-bold text-primary-foreground">{initials(t.name)}</span>
                    {active ? <StatusBadge tone="success" icon={Check} label="Seleccionada" /> : <StatusBadge tone="success" dot label="Activa" />}
                  </div>
                  <div><p className="font-semibold leading-tight">{t.name}</p><p className="font-mono text-xs text-muted-foreground">RUC {t.ruc} · Habido</p></div>
                  <div className="flex flex-wrap gap-1.5"><StatusBadge tone="info" label={t.industryLabel} /><Badge variant="outline"><MapPin className="size-3" /> {t.sites} sede{t.sites === 1 ? "" : "s"}</Badge><Badge variant="outline">{t.role}</Badge></div>
                  <dl className="mt-auto grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                    <dt className="text-muted-foreground">Sincronización SUNAT</dt><dd className="text-success">{t.sunat.label}</dd>
                    <dt className="text-muted-foreground">Certificado digital</dt><dd className={cn(t.certificateDays < 30 ? "font-medium text-warning" : "")}>Vence en {t.certificateDays} días</dd>
                    <dt className="text-muted-foreground">Último acceso</dt><dd>{t.lastAccess}</dd>
                  </dl>
                </button>
              </li>
            );
          })}
        </ul>
      </section>

      <section ref={step2} aria-labelledby="step2" className="rounded-xl border bg-card p-5">
        <p className="text-xs font-medium text-primary uppercase">Paso 2 de 2 · Terminal de acceso</p>
        <div className="flex flex-wrap items-center justify-between gap-2"><h2 id="step2" className="text-lg font-semibold">Configura tu terminal en {tenants.find((t) => t.id === tenantId)?.name}</h2><span className="inline-flex items-center gap-1 text-xs text-muted-foreground"><Wifi className="size-3.5" /> Red privada · IP 190.237.45.12</span></div>
        <div className="mt-4 grid gap-5 lg:grid-cols-2">
          <div>
            <p className="mb-2 text-sm font-medium">Sede operativa</p>
            <ul className="flex flex-col gap-2">
              {sites.map((s) => (
                <li key={s.id}>
                  <button type="button" onClick={() => { setSiteId(s.id); setCashId(s.cashboxes[0]?.id ?? "auditor"); }} aria-pressed={siteId === s.id} className={cn("flex w-full items-start gap-3 rounded-md border p-3 text-left hover:bg-muted/40", siteId === s.id && "border-primary bg-accent/40")}>
                    <span className={cn("mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border", siteId === s.id && "border-primary bg-primary text-primary-foreground")}>{siteId === s.id ? <Check className="size-3" /> : null}</span>
                    <span className="min-w-0"><span className="flex items-center gap-2 text-sm font-medium">{s.name} <Badge variant="outline" className="font-mono">{s.code}</Badge></span><span className="block text-xs text-muted-foreground">{s.address}</span><span className="mt-1 flex flex-wrap gap-1">{s.facts.map((f) => <Badge key={f} variant="outline">{f}</Badge>)}</span></span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <p className="mb-2 text-sm font-medium">Caja registradora / turno operativo</p>
            <ul className="flex flex-col gap-2">
              {site?.cashboxes.map((c) => (
                <li key={c.id}>
                  <button type="button" onClick={() => setCashId(c.id)} aria-pressed={cashId === c.id} className={cn("flex w-full items-start gap-3 rounded-md border p-3 text-left hover:bg-muted/40", cashId === c.id && "border-primary bg-accent/40")}>
                    <span className={cn("mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md", c.status === "abierta" ? "bg-success/10 text-success" : "bg-muted text-muted-foreground")}>{c.status === "abierta" ? <LockOpen className="size-4" /> : <KeyRound className="size-4" />}</span>
                    <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2 text-sm font-medium">{c.name}<StatusBadge tone={c.status === "abierta" ? "success" : "neutral"} dot label={c.status === "abierta" ? "Abierta" : "Cerrada"} /></span><span className="block text-xs text-muted-foreground">{c.detail}</span></span>
                    <span className="shrink-0 text-xs font-medium text-primary">{c.status === "abierta" ? "Unirse al turno" : "Aperturar"}</span>
                  </button>
                </li>
              ))}
              <li>
                <button type="button" onClick={() => setCashId("auditor")} aria-pressed={cashId === "auditor"} className={cn("flex w-full items-start gap-3 rounded-md border border-dashed p-3 text-left hover:bg-muted/40", cashId === "auditor" && "border-primary bg-accent/40")}>
                  <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground"><Eye className="size-4" /></span>
                  <span className="min-w-0 flex-1"><span className="flex items-center gap-2 text-sm font-medium">Modo consulta gerencial / auditor <StatusBadge tone="neutral" label="Solo lectura" /></span><span className="block text-xs text-muted-foreground">Consulta métricas, agenda, inventario y reportes sin afectar dinero en caja.</span></span>
                </button>
              </li>
            </ul>
          </div>
        </div>
        <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t pt-4">
          <p className="text-xs text-muted-foreground"><Building2 className="mr-1 inline size-3.5" />Series asignadas: <span className="font-mono">F001</span> facturas · <span className="font-mono">B001</span> boletas · <span className="font-mono">FC01</span> notas de crédito · OSE Digiflow OK</p>
          <div className="flex flex-wrap gap-2"><Button variant="ghost" render={<Link href="/login" />} nativeButton={false}>Cambiar de cuenta</Button><Button size="lg" onClick={go}>Continuar al panel principal <ArrowRight data-icon="inline-end" /></Button></div>
        </div>
      </section>
      <p className="text-center text-[11px] text-muted-foreground">SUNAT producción v4.2 · IP autorizada 190.237.45.12 · SHA-256 · Soporte técnico Lima (01) 700-3400</p>
    </main>
  );
}
