"use client";

import { Armchair, CalendarDays, CalendarPlus, CheckCircle2, ChevronLeft, ChevronRight, Clock, CreditCard, List, MessageCircle, Phone, Stethoscope, Store, CalendarClock, LogIn, Smile, Users } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { useMediaQuery } from "@/hooks/use-media-query";
import { useIsMobile } from "@/hooks/use-mobile";
import { customersMock } from "@/features/customers/mocks/customers";
import { initials } from "@/lib/tenant";
import { cn } from "@/lib/utils";
import type { Appointment, AppointmentStatus, Chair } from "../mocks/appointments";
import { NewAppointmentSheet, RescheduleDialog, waitlist } from "./new-appointment-sheet";

const statusMeta: Record<AppointmentStatus, { label: string; tone: BadgeTone; card: string; text: string }> = {
  atendido: { label: "Atendido", tone: "success", card: "border-transparent bg-accent dark:bg-accent/30", text: "text-primary" },
  en_silla: { label: "En silla", tone: "info", card: "border-transparent bg-teal-100 dark:bg-teal-900/40", text: "text-teal-900 dark:text-teal-100" },
  en_espera: { label: "En espera", tone: "info", card: "border-transparent bg-blue-50 dark:bg-blue-950/40", text: "text-blue-700 dark:text-blue-300" },
  confirmado: { label: "Confirmado", tone: "neutral", card: "border-transparent bg-indigo-50 dark:bg-indigo-950/40", text: "text-indigo-700 dark:text-indigo-300" },
  agendado: { label: "Agendado", tone: "neutral", card: "border-transparent bg-muted", text: "text-muted-foreground" },
  cancelado: { label: "Bloqueado / cancelado", tone: "danger", card: "border-dashed bg-muted/40 opacity-70", text: "text-muted-foreground" },
};

const START_MIN = 8 * 60;
const PX_PER_MIN = 2.4;
const toMin = (t: string) => Number(t.slice(0, 2)) * 60 + Number(t.slice(3, 5));

export function Agenda({ chairs, appointments, hours }: { chairs: Chair[]; appointments: Appointment[]; hours: string[] }) {
  const [selectedId, setSelectedId] = React.useState<string | null>(appointments[0]?.id ?? null);
  const isMobile = useIsMobile();
  const belowXl = useMediaQuery("(max-width: 1279px)");
  const [viewChoice, setViewChoice] = React.useState<string | null>(null);
  const view = viewChoice ?? (isMobile ? "list" : "chair");
  const setView = setViewChoice;
  const selected = appointments.find((a) => a.id === selectedId) ?? null;
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const [newOpen, setNewOpen] = React.useState(false);
  const [rescheduling, setRescheduling] = React.useState<Appointment | null>(null);
  const gridHeight = (hours.length) * 60 * PX_PER_MIN;

  function select(a: Appointment) {
    setSelectedId(a.id);
    if (belowXl) setMobileOpen(true); // desktop shows the detail in the side panel
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[1fr_22rem]">
      <div className="flex flex-wrap items-center gap-2 xl:col-span-2">
        <div className="flex items-center gap-1 rounded-xl border bg-card p-1.5 shadow-xs">
          <Button variant="ghost" size="icon-xs" aria-label="Día anterior"><ChevronLeft /></Button>
          <Button variant="ghost" size="xs" className="font-semibold">Hoy</Button>
          <Button variant="ghost" size="icon-xs" aria-label="Día siguiente"><ChevronRight /></Button>
          <span className="ml-1 flex items-center gap-2 rounded-lg bg-accent px-3 py-1.5 text-sm font-semibold text-primary dark:bg-muted"><CalendarDays className="size-4" /> Viernes, 12 de setiembre 2026</span>
        </div>
        <Select defaultValue="miraflores" items={[{ value: "miraflores", label: "Sede Central – Miraflores" }, { value: "sanisidro", label: "Sede San Isidro" }]}>
          <SelectTrigger className="h-auto w-60 rounded-xl border bg-card py-1.5 shadow-xs [&>span]:flex [&>span]:flex-col [&>span]:items-start" aria-label="Sede de atención"><Store className="size-4 text-muted-foreground" /><span className="flex flex-col text-left leading-tight"><span className="text-[10px] text-muted-foreground">Sede de atención</span><SelectValue /></span></SelectTrigger>
          <SelectContent><SelectItem value="miraflores">Sede Central – Miraflores</SelectItem><SelectItem value="sanisidro">Sede San Isidro</SelectItem></SelectContent>
        </Select>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          <ToggleGroup value={[view]} onValueChange={(v) => v[0] && setView(v[0])} className="rounded-xl bg-accent p-1 dark:bg-muted">
            <ToggleGroupItem value="chair" aria-label="Por sillón" className="h-8 rounded-lg border-transparent px-3 text-xs font-semibold text-foreground/80 hover:bg-transparent aria-pressed:border-transparent aria-pressed:bg-card aria-pressed:text-primary aria-pressed:shadow-xs data-pressed:border-transparent data-pressed:bg-card data-pressed:text-primary"><Armchair /> <span className="hidden sm:inline">Por sillón (1, 2, 3)</span></ToggleGroupItem>
            <ToggleGroupItem value="pro" aria-label="Por especialista" className="h-8 rounded-lg border-transparent px-3 text-xs font-semibold text-foreground/80 hover:bg-transparent aria-pressed:border-transparent aria-pressed:bg-card aria-pressed:text-primary aria-pressed:shadow-xs data-pressed:border-transparent data-pressed:bg-card data-pressed:text-primary"><Stethoscope /> <span className="hidden sm:inline">Por especialista</span></ToggleGroupItem>
            <ToggleGroupItem value="list" aria-label="Lista" className="h-8 rounded-lg border-transparent px-3 text-xs font-semibold text-foreground/80 hover:bg-transparent aria-pressed:border-transparent aria-pressed:bg-card aria-pressed:text-primary aria-pressed:shadow-xs data-pressed:border-transparent data-pressed:bg-card data-pressed:text-primary"><List /> <span className="hidden sm:inline">Lista</span></ToggleGroupItem>
          </ToggleGroup>
          <Button size="lg" className="font-semibold" onClick={() => setNewOpen(true)}><CalendarPlus data-icon="inline-start" /> Agendar nueva cita</Button>
        </div>
      </div>

      <SectionCard
        title={<span className="sr-only">Agenda del día</span>}
        className="[&>div:first-child]:hidden"
        contentClassName="p-0"
      >
        {view === "list" ? (
          <ul className="divide-y">
            {[...appointments].sort((a, b) => toMin(a.start) - toMin(b.start)).map((a) => (
              <li key={a.id}>
                <button type="button" onClick={() => select(a)} className={cn("flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/50", a.id === selectedId && "bg-accent/40")}>
                  <span className="w-24 shrink-0 font-mono text-xs tabular-nums">{a.start}–{a.end}</span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{a.patient}</p>
                    <p className="truncate text-xs text-muted-foreground">{a.procedure} · {a.professional}</p>
                  </div>
                  <StatusBadge tone={statusMeta[a.status].tone} label={statusMeta[a.status].label} />
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <div className="overflow-x-auto">
            <div className="min-w-[720px]">
              <div className="grid border-b bg-muted/40 text-[11px] font-semibold tracking-wide uppercase" style={{ gridTemplateColumns: `4rem repeat(${chairs.length}, minmax(0, 1fr))` }}>
                <div className="flex items-center gap-1 px-2 py-2.5 text-muted-foreground"><Clock className="size-3.5" /> Hora</div>
                {chairs.map((c) => (
                  <div key={c.id} className="flex items-center gap-2 px-3 py-2.5">
                    <span className="size-2 rounded-full bg-primary" /><span>{c.label}</span><span className="font-normal text-muted-foreground normal-case">• {c.professional}</span>
                  </div>
                ))}
              </div>
              <div className="relative grid" style={{ gridTemplateColumns: `4rem repeat(${chairs.length}, minmax(0, 1fr))`, height: gridHeight }}>
                {/* hour lines */}
                <div className="relative">
                  {hours.map((h, i) => (
                    <div key={h} className="absolute left-0 w-full border-t px-2 font-mono text-[11px] text-muted-foreground" style={{ top: i * 60 * PX_PER_MIN }}>
                      {h}
                    </div>
                  ))}
                </div>
                {chairs.map((c) => (
                  <div key={c.id} className="relative border-l">
                    {hours.map((h, i) => (
                      <div key={h} className="absolute left-0 w-full border-t border-dashed" style={{ top: i * 60 * PX_PER_MIN }} />
                    ))}
                    {appointments.filter((a) => a.chairId === c.id).map((a) => {
                      const top = (toMin(a.start) - START_MIN) * PX_PER_MIN;
                      const height = Math.max(44, (toMin(a.end) - toMin(a.start)) * PX_PER_MIN - 4);
                      const compact = height < 78;
                      const m = statusMeta[a.status];
                      return (
                        <button
                          key={a.id}
                          type="button"
                          onClick={() => select(a)}
                          aria-label={`${a.start} ${a.patient}`}
                          className={cn(
                            "absolute inset-x-1.5 flex flex-col overflow-hidden rounded-lg border p-2.5 text-left text-xs transition-shadow hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                            m.card,
                            a.id === selectedId && "ring-2 ring-primary",
                          )}
                          style={{ top: top + 2, height }}
                        >
                          <span className="flex items-center justify-between gap-2 leading-tight">
                            <span className="truncate text-[13px] font-bold">{a.patient}</span>
                            <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold", a.status === "atendido" ? "bg-foreground text-background" : "bg-card/80 " + m.text)}>{a.status === "atendido" ? "F10 POS" : m.label}</span>
                          </span>
                          <span className={cn("mt-0.5 truncate text-[11px]", a.status === "atendido" ? "font-medium text-primary" : "text-muted-foreground")}>{a.procedure}</span>
                          {!compact ? (
                            <span className="mt-auto flex items-center justify-between gap-2 rounded-md bg-card/80 px-2 py-1 text-[11px]">
                              <span className={cn("flex items-center gap-1 font-medium", m.text)}><span className="size-1.5 rounded-full bg-current" />{m.label}</span>
                              {a.price ? <span className="font-mono font-bold text-primary tabular-nums">{formatCurrency(a.price)}</span> : <span className="font-mono text-muted-foreground">{a.start}–{a.end}</span>}
                            </span>
                          ) : null}
                        </button>
                      );
                    })}
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </SectionCard>

      <aside aria-label="Detalle de cita" className="hidden flex-col gap-4 xl:flex">
        {selected ? <AppointmentDetail a={selected} onReschedule={() => setRescheduling(selected)} /> : null}
        <SectionCard title="Próximos en sala de espera" icon={Users} action={<StatusBadge tone="info" label={`${waitlist.length} pacientes`} />} contentClassName="p-0">
          <ul className="divide-y">{waitlist.map((w) => <li key={w.name} className="flex items-center gap-3 px-4 py-2.5 text-sm"><div className="min-w-0 flex-1"><p className="truncate font-semibold">{w.name}</p><p className="text-xs text-muted-foreground">{w.reason} • {w.wants}</p></div><Button size="xs" variant="secondary" className="font-semibold" onClick={() => setNewOpen(true)}>Asignar</Button></li>)}</ul>
        </SectionCard>
      </aside>
      <NewAppointmentSheet open={newOpen} onOpenChange={setNewOpen} chairs={chairs} />
      <RescheduleDialog appointment={rescheduling} onClose={() => setRescheduling(null)} />
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="bottom" className="max-h-[90svh] overflow-y-auto rounded-t-xl xl:hidden">
          <SheetHeader className="sr-only"><SheetTitle>Detalle de cita</SheetTitle></SheetHeader>
          {selected ? <AppointmentDetail a={selected} onReschedule={() => { setMobileOpen(false); setRescheduling(selected); }} /> : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}

function posHref(a: Appointment) {
  const doc = a.document.replace(/^\w+\s/, "");
  const customer = customersMock.find((c) => c.documentNumber === doc);
  const item = /resina|curaci/i.test(a.procedure) ? "c1" : /profilaxis/i.test(a.procedure) ? "c2" : /radiograf/i.test(a.procedure) ? "c6" : /extracci|exodoncia/i.test(a.procedure) ? "c10" : "c9";
  return `/ventas/nueva?${customer ? `cliente=${customer.id}&` : ""}items=${item}`;
}

function AppointmentDetail({ a, onReschedule }: { a: Appointment; onReschedule: () => void }) {
  const m = statusMeta[a.status];
  return (
    <div className="flex flex-col gap-3 rounded-xl border bg-card p-4 shadow-xs">
      <div className="flex items-center justify-between gap-2">
        <span className={cn("flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-bold tracking-wide uppercase", a.status === "atendido" ? "bg-primary text-primary-foreground" : "bg-accent text-primary dark:bg-muted")}>{a.status === "atendido" ? <><CheckCircle2 className="size-3.5" /> Atendido · facturar</> : m.label}</span>
        <span className="text-xs text-muted-foreground">Cita #{a.code}</span>
      </div>
      <div className="flex items-center gap-3">
        <span className="flex size-12 items-center justify-center rounded-full bg-accent text-sm font-bold text-primary ring-2 ring-primary/30">{initials(a.patient)}</span>
        <div className="min-w-0">
          <p className="truncate text-base font-bold">{a.patient}</p>
          <p className="text-xs text-muted-foreground">{a.document.replace(/^(\w+)\s(.+)$/, "$1: ")}<strong className="font-semibold text-foreground">{a.document.split(" ")[1]}</strong>{a.age ? ` • ${a.age} años` : ""}</p>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Button variant="secondary" className="bg-teal-100 font-semibold text-teal-900 hover:bg-teal-200 dark:bg-teal-900/50 dark:text-teal-100" onClick={() => toast.info("Abriendo WhatsApp…")}><MessageCircle data-icon="inline-start" /> WhatsApp (+51)</Button>
        <Button variant="secondary" className="bg-accent font-semibold hover:bg-accent/70 dark:bg-muted"><Phone data-icon="inline-start" /> 987 654 321</Button>
      </div>
      <div className="rounded-lg bg-muted/50 p-3 text-sm">
        <div className="flex items-center justify-between text-[10px] font-semibold tracking-wide uppercase"><span className="text-muted-foreground">{a.status === "atendido" ? "Procedimiento ejecutado" : "Procedimiento programado"}</span><span className="text-primary normal-case">{a.professional}</span></div>
        <p className="mt-1 text-base font-bold">{a.procedure}</p>
        {a.note ? <p className="mt-1 text-xs text-muted-foreground">{a.note}</p> : null}
        <div className="mt-2 flex flex-wrap gap-1.5 text-[11px]"><span className="rounded bg-card px-2 py-0.5 shadow-xs">{a.status === "atendido" ? "Término" : "Inicio"}: {a.status === "atendido" ? a.end : a.start}</span><span className="rounded bg-card px-2 py-0.5 shadow-xs">SUNAT: Catálogo 01 / Gravado</span></div>
      </div>
      {a.price ? (
        <dl className="rounded-lg bg-muted/50 p-3 text-xs">
          <div className="flex justify-between py-0.5 text-muted-foreground"><dt>Honorarios y material odontológico</dt><dd className="font-mono">{formatCurrency(a.price / 1.18)}</dd></div>
          <div className="flex justify-between py-0.5 text-muted-foreground"><dt>I.G.V. (18%)</dt><dd className="font-mono">{formatCurrency(a.price - a.price / 1.18)}</dd></div>
          <div className="mt-1 flex items-baseline justify-between border-t pt-2"><dt className="text-base font-bold">Total a liquidar:</dt><dd className="font-mono text-xl font-bold text-primary">{formatCurrency(a.price)}</dd></div>
        </dl>
      ) : null}
      <div className="flex flex-col gap-2">
        {a.status === "atendido" ? (
          <Button size="lg" className="font-semibold" render={<Link href={posHref(a)} />} nativeButton={false}><CreditCard data-icon="inline-start" /> Cobrar y emitir comprobante en POS (F10)</Button>
        ) : a.status === "en_espera" || a.status === "confirmado" ? (
          <Button size="lg" className="font-semibold" onClick={() => toast.success(`${a.patient} pasó a sillón`)}><LogIn data-icon="inline-start" /> Hacer pasar</Button>
        ) : null}
        <div className="grid grid-cols-2 gap-2">
          <Button variant="secondary" className="bg-accent font-semibold hover:bg-accent/70 dark:bg-muted"><Smile data-icon="inline-start" /> Ver odontograma</Button>
          <Button variant="secondary" className="bg-accent font-semibold hover:bg-accent/70 dark:bg-muted" onClick={onReschedule}><CalendarClock data-icon="inline-start" /> Reprogramar</Button>
        </div>
      </div>
    </div>
  );
}
