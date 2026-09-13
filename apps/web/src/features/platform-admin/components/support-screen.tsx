"use client";

import { Clock, Download, MessageCircle, Plus, Send, Smile, Star, Timer, Wand2 } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { supportSummary as s, tickets, type Ticket } from "../mocks/ops";

const statusMeta: Record<Ticket["status"], { label: string; tone: BadgeTone }> = { abierto: { label: "Abierto", tone: "danger" }, en_progreso: { label: "En progreso", tone: "info" }, esperando: { label: "Esperando cliente", tone: "warning" }, resuelto: { label: "Resuelto", tone: "success" } };
const prioCls = { P1: "bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300", P2: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300", P3: "bg-muted text-muted-foreground" };
const macros = ["Bypass a Bizlinks OSE", "Reintento de cobro Niubiz", "Guía renovación .pfx", "Ampliación de cuota CPE"];

export function SupportScreen() {
  const [rows, setRows] = React.useState(tickets);
  const [selectedId, setSelectedId] = React.useState(tickets[0].id);
  const [reply, setReply] = React.useState("");
  const sel = rows.find((t) => t.id === selectedId) ?? rows[0];
  const sla = (m: number) => (m === 0 ? "—" : m < 60 ? `${m} min` : `${Math.floor(m / 60)}h ${m % 60}m`);
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Soporte técnico · Mesa de tickets y SLA multi-tenant" title="Bandeja de tickets y monitor de SLA B2B SaaS" description="Gestión de incidentes tributarios SUNAT, integraciones OSE, pasarelas de pago y soporte operativo para 1,482 empresas clientes en Perú." status={<StatusBadge tone="success" label="SLA cumplido en 98.4% de 342 tickets este ciclo" className="rounded-md" />} actions={<><Button variant="outline" className="font-semibold" onClick={() => toast.success("Métricas exportadas")}><Download data-icon="inline-start" /> Exportar métricas CSV</Button><Button variant="outline" className="font-semibold">Políticas de SLA OSE</Button><Button className="font-semibold" onClick={() => toast.info("Nuevo ticket")}><Plus data-icon="inline-start" /> Crear incidente / ticket</Button></>} />
      <StatGrid>
        <StatCard label="Tiempo 1ª respuesta" value={`${s.firstResponse}`} suffix="min" icon={Timer} tone="success" hint={<span className="font-semibold text-emerald-600">{s.firstResponseDelta}m vs semana anterior</span>} footer={<span className="flex w-full justify-between"><span>Objetivo SLA: &lt; 20 min</span><strong className="text-emerald-600">{s.slaMet}% cumplido</strong></span>} />
        <StatCard label="Resolución promedio (MTTR)" value={s.mttr} icon={Clock} tone="info" hint={<span className="font-semibold text-emerald-600">{s.mttrDelta} optimizado</span>} footer={<span className="flex w-full justify-between"><span>Impacto en contingencia CDR</span><span>Récord {s.record}</span></span>} />
        <StatCard label="Tickets en riesgo SLA" value={String(s.atRisk)} icon={Timer} tone="danger" emphasizeValue hint="< 30 min para breach" footer={<span className="flex w-full justify-between"><span>1 en SUNAT OSE · 1 en Niubiz</span><span className="text-emerald-600">{s.overdue} vencidos hoy</span></span>} />
        <StatCard label="CSAT empresas clientes" value={String(s.csat)} suffix="/5" icon={Smile} tone="success" hint={<span className="flex items-center gap-1 font-semibold text-emerald-600"><Star className="size-3 fill-current" /> {s.positive}% positivos</span>} footer={`Encuestas respondidas: ${s.surveys}`} />
      </StatGrid>

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-amber-300 border-l-4 bg-amber-50 p-4 text-sm dark:border-amber-800 dark:bg-amber-950/30">
        <StateIcon state="warning" className="size-5" />
        <div className="min-w-0 flex-1"><p className="font-bold text-amber-900 dark:text-amber-200">Alerta de mesa SRE: incidencia SUNAT Web Services directo (error 1033) <Tag label="INC-8421" className="ml-1 font-mono" /></p><p className="text-xs text-amber-800/80 dark:text-amber-300">Afecta a 4 empresas con certificado directo en Huancayo y Trujillo. Las respuestas automáticas sugieren activar bypass a Bizlinks OSE para evitar rechazos en facturas de contingencia de cierre diario.</p></div>
        <Button className="bg-amber-600 font-semibold text-white hover:bg-amber-700" onClick={() => toast.success("Macro 'Bypass a Bizlinks OSE' enviada a 4 tenants")}><Wand2 data-icon="inline-start" /> Activar macro</Button>
      </div>

      <div className="grid gap-4 xl:grid-cols-5">
        <SectionCard title="Bandeja por prioridad y SLA" description={`${rows.filter((t) => t.status !== "resuelto").length} tickets activos`} className="xl:col-span-3" contentClassName="p-0">
          <ul className="divide-y">
            {rows.map((t) => { const m = statusMeta[t.status]; const risk = t.slaMin > 0 && t.slaMin <= 30; return (
              <li key={t.id}><button type="button" onClick={() => setSelectedId(t.id)} aria-pressed={sel.id === t.id} className={cn("flex w-full flex-col gap-2 px-4 py-3 text-left hover:bg-muted/40", sel.id === t.id && "border-l-4 border-l-primary bg-accent/40 dark:bg-accent/10")}>
                <div className="flex flex-wrap items-center gap-2 text-xs"><span className={cn("rounded px-1.5 py-0.5 font-bold", prioCls[t.priority])}>{t.priority}</span><span className="font-mono font-semibold">{t.id}</span><StatusBadge tone={m.tone} dot label={m.label} /><span className="ml-auto flex items-center gap-1 text-muted-foreground"><MessageCircle className="size-3" /> {t.channel}</span></div>
                <p className="text-sm font-semibold">{t.subject}</p>
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"><span>{t.tenant} · Agente: {t.agent}</span><span className={cn("flex items-center gap-1 font-mono font-semibold", risk ? "text-destructive" : t.status === "resuelto" ? "text-emerald-600" : "text-foreground")}><Timer className="size-3" /> {t.status === "resuelto" ? "SLA cumplido" : `SLA restante: ${sla(t.slaMin)}`}</span></div>
              </button></li>
            ); })}
          </ul>
        </SectionCard>
        <SectionCard title={`Conversación · ${sel.id}`} description={`${sel.tenant} · abierto ${formatDate(sel.createdAt)} ${sel.createdAt.slice(11, 16)}`} className="xl:col-span-2" action={<Button size="sm" variant="outline" className="font-semibold" onClick={() => { setRows((rs) => rs.map((t) => (t.id === sel.id ? { ...t, status: "resuelto", slaMin: 0 } : t))); toast.success(`${sel.id} resuelto`); }} disabled={sel.status === "resuelto"}>Marcar resuelto</Button>} contentClassName="flex flex-col gap-3 p-4 text-sm">
          <div className="grid grid-cols-3 gap-2 text-xs"><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">Plan</p><p className="font-semibold">Pro Multi-Sede</p></div><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">OSE</p><p className="font-semibold">Bizlinks</p></div><div className="rounded-md bg-muted/50 p-2"><p className="text-muted-foreground">Contacto</p><p className="font-semibold">+51 984 123 456</p></div></div>
          <div className="flex flex-col gap-2 rounded-lg bg-muted/40 p-3 text-xs"><div className="max-w-[85%] rounded-lg bg-card p-2.5 shadow-xs"><p className="font-semibold">Cliente</p><p>{sel.subject}. Necesitamos emitir el cierre del día.</p></div><div className="ml-auto max-w-[85%] rounded-lg bg-primary p-2.5 text-primary-foreground"><p className="font-semibold">{sel.agent}</p><p>Estamos aplicando bypass al OSE alterno; en 5 minutos reintentamos los comprobantes en cola.</p></div></div>
          <div className="flex flex-wrap gap-1.5">{macros.map((m) => <button key={m} type="button" onClick={() => setReply(`[Macro] ${m}: `)} className="rounded-full border bg-card px-2.5 py-1 text-xs font-medium hover:bg-muted">{m}</button>)}</div>
          <Textarea rows={3} value={reply} onChange={(e) => setReply(e.target.value)} placeholder="Escribe una respuesta o usa una macro…" />
          <div className="flex justify-end"><Button className="font-semibold" disabled={!reply.trim()} onClick={() => { toast.success(`Respuesta enviada por ${sel.channel}`); setReply(""); }}><Send data-icon="inline-start" /> Enviar respuesta</Button></div>
        </SectionCard>
      </div>
    </div>
  );
}
