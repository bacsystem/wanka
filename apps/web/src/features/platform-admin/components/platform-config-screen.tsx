"use client";

import { CreditCard, Database, Globe, Mail, MessageCircle, Save, SlidersHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { ConnectionIcon } from "@/components/shared/dynamic-icon";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";

export function PlatformConfigScreen() {
  const save = () => toast.success("Configuración de plataforma guardada", { description: "Cambio registrado en auditoría con doble aprobación" });
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Plataforma · Configuración núcleo" title="Configuración de plataforma" description="Dominios, correo transaccional, WhatsApp Business API, pasarelas de pago, backups y límites por defecto." status={<StatusBadge tone="success" dot label="Entorno: producción" className="rounded-md" />} actions={<Button className="font-semibold" onClick={save}><Save data-icon="inline-start" /> Guardar cambios</Button>} />
      <div className="grid gap-4 xl:grid-cols-2">
        <SectionCard title="Dominios y subdominios por tenant" description="Wildcard *.perusaas.pe con TLS automático" icon={Globe} contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="Dominio raíz de la app"><Input defaultValue="app.perusaas.pe" /></Field>
          <Field label="Dominio de la consola"><Input defaultValue="admin.perusaas.pe" /></Field>
          <Field label="Patrón de subdominio por tenant" className="sm:col-span-2"><Input defaultValue="{slug}.perusaas.pe" className="font-mono" /></Field>
          <div className="flex items-center justify-between rounded-lg border p-3 text-sm sm:col-span-2"><span>Permitir dominios personalizados (CNAME) en plan Enterprise</span><Switch defaultChecked aria-label="Dominios personalizados" /></div>
        </SectionCard>
        <SectionCard title="Correo transaccional" description="Facturas, recordatorios y alertas" icon={Mail} action={<span className="flex items-center gap-1 text-xs"><ConnectionIcon online /> SES us-east-1</span>} contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="Proveedor"><Select defaultValue="ses" items={[{ value: "ses", label: "Amazon SES" }, { value: "sendgrid", label: "SendGrid" }, { value: "postmark", label: "Postmark" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ses">Amazon SES</SelectItem><SelectItem value="sendgrid">SendGrid</SelectItem><SelectItem value="postmark">Postmark</SelectItem></SelectContent></Select></Field>
          <Field label="Remitente por defecto"><Input defaultValue="no-reply@perusaas.pe" /></Field>
          <Field label="Dominio firmado (DKIM)"><Input defaultValue="mail.perusaas.pe" /></Field>
          <Field label="Límite diario por tenant"><Input defaultValue="5000" type="number" className="font-mono" /></Field>
        </SectionCard>
        <SectionCard title="WhatsApp Business API" description="Meta Cloud API · plantillas aprobadas" icon={MessageCircle} action={<StatusBadge tone="success" dot label="Calidad alta" />} contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="Business Account ID"><Input defaultValue="1043928118824" className="font-mono" /></Field>
          <Field label="Pool de remitentes"><Input defaultValue="6 números" /></Field>
          <Field label="Límite de mensajes / hora por tenant"><Input defaultValue="1000" type="number" className="font-mono" /></Field>
          <div className="flex items-center justify-between rounded-lg border p-3 text-sm"><span>Failover a SMS (Twilio)</span><Switch aria-label="Failover SMS" /></div>
        </SectionCard>
        <SectionCard title="Pasarelas de pago" description="Cobro de suscripciones a tenants" icon={CreditCard} contentClassName="flex flex-col gap-2 p-4">
          {[["Niubiz", "Débito automático · comercio 4531892", true], ["Izipay", "Checkout · llave pk_live_…", true], ["Mercado Pago", "Suscripciones · pendiente homologación", false], ["Culqi", "Tarjetas · sandbox", false]].map(([n, d, on]) => <div key={String(n)} className="flex items-center justify-between rounded-lg border p-3 text-sm"><span className="flex items-center gap-3"><ConnectionIcon online={Boolean(on)} /><span><span className="block font-semibold">{n}</span><span className="text-xs text-muted-foreground">{d}</span></span></span><Switch defaultChecked={Boolean(on)} aria-label={`Pasarela ${n}`} /></div>)}
        </SectionCard>
        <SectionCard title="Backups y retención" description="RPO 15 min · RTO 1 h" icon={Database} contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="Frecuencia de snapshot"><Select defaultValue="15" items={[{ value: "15", label: "Cada 15 minutos" }, { value: "60", label: "Cada hora" }, { value: "1440", label: "Diario" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="15">Cada 15 minutos</SelectItem><SelectItem value="60">Cada hora</SelectItem><SelectItem value="1440">Diario</SelectItem></SelectContent></Select></Field>
          <Field label="Retención (días)"><Input defaultValue="2555" type="number" className="font-mono" /></Field>
          <Field label="Región secundaria"><Input defaultValue="sa-east-1 (São Paulo)" /></Field>
          <div className="flex items-center justify-between rounded-lg border p-3 text-sm"><span>Cifrado en reposo (KMS)</span><Switch defaultChecked aria-label="Cifrado" /></div>
        </SectionCard>
        <SectionCard title="Límites por defecto" description="Se aplican a tenants nuevos hasta asignar plan" icon={SlidersHorizontal} contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="CPE / mes"><Input defaultValue="500" type="number" className="font-mono" /></Field>
          <Field label="Usuarios"><Input defaultValue="3" type="number" className="font-mono" /></Field>
          <Field label="Sedes"><Input defaultValue="1" type="number" className="font-mono" /></Field>
          <Field label="Duración del trial (días)"><Input defaultValue="14" type="number" className="font-mono" /></Field>
        </SectionCard>
      </div>
    </div>
  );
}
