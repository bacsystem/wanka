"use client";

import { AlertTriangle, CalendarClock, Check, MessageCircle, UserPlus } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { customersMock } from "@/features/customers/mocks/customers";
import { formatCurrency, formatDate } from "@/lib/format";
import { initials } from "@/lib/tenant";
import { cn } from "@/lib/utils";
import type { Appointment, Chair } from "../mocks/appointments";

const services = [
  { id: "consulta", label: "Consulta odontológica", minutes: 30, price: 60 },
  { id: "profilaxis", label: "Profilaxis dental", minutes: 45, price: 70 },
  { id: "curacion", label: "Curación con resina", minutes: 45, price: 85 },
  { id: "endodoncia", label: "Endodoncia", minutes: 90, price: 450 },
  { id: "control", label: "Control de ortodoncia", minutes: 30, price: 120 },
];
const slots = ["08:30", "09:00", "09:30", "10:00", "10:30", "11:00", "11:30", "12:00", "14:00", "14:30", "15:00", "15:30", "16:00", "16:30"];
const busy = new Set(["08:30", "10:00", "11:30", "15:00"]);
const professionals = ["Dra. Mendoza", "Dr. Flores", "Higienista R. Cano"];
const pad = (n: number) => String(n).padStart(2, "0");
const addMinutes = (t: string, m: number) => { const [h, mi] = t.split(":").map(Number); const total = h * 60 + mi + m; return `${pad(Math.floor(total / 60))}:${pad(total % 60)}`; };

export function NewAppointmentSheet({ open, onOpenChange, chairs }: { open: boolean; onOpenChange: (o: boolean) => void; chairs: Chair[] }) {
  const [query, setQuery] = React.useState("");
  const [patientId, setPatientId] = React.useState<string | null>("u2");
  const [chair, setChair] = React.useState(chairs[0].id);
  const [pro, setPro] = React.useState(professionals[0]);
  const [service, setService] = React.useState(services[2].id);
  const [date, setDate] = React.useState("2026-09-18");
  const [slot, setSlot] = React.useState("09:30");
  const [whatsapp, setWhatsapp] = React.useState(true);
  const [reminder, setReminder] = React.useState(true);
  const patient = customersMock.find((c) => c.id === patientId) ?? null;
  const svc = services.find((s) => s.id === service)!;
  const results = query.trim() ? customersMock.filter((c) => c.name.toLowerCase().includes(query.toLowerCase()) || c.documentNumber.includes(query)).slice(0, 4) : [];
  const chairLabel = chairs.find((c) => c.id === chair)?.label;

  function submit() {
    if (!patient) { toast.error("Selecciona un paciente"); return; }
    toast.success(`Cita agendada · ${formatDate(date)} ${slot} · ${chairLabel} · ${pro}`, { description: whatsapp ? `Confirmación enviada por WhatsApp a ${patient.name}` : patient.name });
    onOpenChange(false);
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader><SheetTitle>Nueva cita</SheetTitle><SheetDescription>Agenda por sillón · confirmación automática por WhatsApp.</SheetDescription></SheetHeader>
        <div className="flex flex-col gap-4 px-4">
          <section>
            <p className="mb-1.5 text-xs font-medium text-muted-foreground uppercase">1 · Paciente</p>
            {patient ? (
              <div className="flex items-center gap-3 rounded-md border p-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-mono text-xs font-semibold text-accent-foreground">{initials(patient.name)}</span>
                <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{patient.name}</p><p className="font-mono text-xs text-muted-foreground">{patient.documentType} {patient.documentNumber}{patient.clinicalRecord ? ` · ${patient.clinicalRecord}` : ""}</p>{patient.id === "u2" ? <StatusBadge tone="danger" icon={AlertTriangle} label="Alergia: penicilina" className="mt-1" /> : null}</div>
                <Button variant="ghost" size="sm" onClick={() => setPatientId(null)}>Cambiar</Button>
              </div>
            ) : (
              <div className="relative"><Input autoFocus value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Buscar por DNI o nombre…" aria-label="Buscar paciente" />{results.length ? <ul className="absolute z-10 mt-1 w-full rounded-md border bg-popover shadow-md">{results.map((c) => <li key={c.id}><button type="button" onClick={() => { setPatientId(c.id); setQuery(""); }} className="flex w-full justify-between px-3 py-2 text-left text-sm hover:bg-accent"><span>{c.name}</span><span className="font-mono text-xs text-muted-foreground">{c.documentType} {c.documentNumber}</span></button></li>)}</ul> : null}<Button variant="link" size="sm" className="px-0"><UserPlus data-icon="inline-start" /> Nuevo paciente</Button></div>
            )}
          </section>
          <section className="grid gap-3 sm:grid-cols-2">
            <p className="text-xs font-medium text-muted-foreground uppercase sm:col-span-2">2 · Atención</p>
            <Field label="Sede"><Select defaultValue="miraflores" items={[{ value: "miraflores", label: "Sede Miraflores" }, { value: "sanisidro", label: "Sede San Isidro" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="miraflores">Sede Miraflores</SelectItem><SelectItem value="sanisidro">Sede San Isidro</SelectItem></SelectContent></Select></Field>
            <Field label="Sillón"><Select value={chair} onValueChange={(v) => setChair(String(v))} items={chairs.map((c) => ({ value: c.id, label: c.label }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{chairs.map((c) => <SelectItem key={c.id} value={c.id}>{c.label} · {c.professional}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Profesional"><Select value={pro} onValueChange={(v) => setPro(String(v))} items={professionals.map((p) => ({ value: p, label: p }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{professionals.map((p) => <SelectItem key={p} value={p}>{p}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Tipo de atención"><Select value={service} onValueChange={(v) => setService(String(v))} items={services.map((s) => ({ value: s.id, label: `${s.label} · ${s.minutes} min · ${formatCurrency(s.price)}` }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{services.map((s) => <SelectItem key={s.id} value={s.id}>{s.label} · {s.minutes} min · {formatCurrency(s.price)}</SelectItem>)}</SelectContent></Select></Field>
          </section>
          <section>
            <p className="mb-1.5 text-xs font-medium text-muted-foreground uppercase">3 · Horario</p>
            <div className="grid gap-3 sm:grid-cols-[10rem_1fr]">
              <Field label="Fecha"><Input type="date" value={date} onChange={(e) => setDate(e.target.value)} /></Field>
              <Field label={`Horarios disponibles · ${svc.minutes} min`}>
                <div className="grid grid-cols-4 gap-1.5 sm:grid-cols-5">{slots.map((s) => { const b = busy.has(s); return <button key={s} type="button" disabled={b} onClick={() => setSlot(s)} aria-pressed={slot === s} className={cn("rounded-md border px-2 py-1.5 font-mono text-xs tabular-nums", b && "text-muted-foreground line-through opacity-60", slot === s && "border-primary bg-primary text-primary-foreground")}>{s}</button>; })}</div>
              </Field>
            </div>
          </section>
          <section className="grid gap-3">
            <Field label="Motivo / observaciones"><Textarea rows={2} placeholder="Control de obturación y pulido estético" /></Field>
            <Field label="Aseguradora / EPS"><Select defaultValue="rimac" items={[{ value: "ninguna", label: "Particular" }, { value: "rimac", label: "Rimac EPS · copago 20%" }, { value: "pacifico", label: "Pacífico EPS · copago 15%" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ninguna">Particular</SelectItem><SelectItem value="rimac">Rimac EPS · copago 20%</SelectItem><SelectItem value="pacifico">Pacífico EPS · copago 15%</SelectItem></SelectContent></Select></Field>
            <label className="flex items-center gap-2 text-sm"><Checkbox checked={whatsapp} onCheckedChange={(v) => setWhatsapp(Boolean(v))} /> <MessageCircle className="size-4 text-success" /> Enviar confirmación por WhatsApp</label>
            <label className="flex items-center gap-2 text-sm"><Checkbox checked={reminder} onCheckedChange={(v) => setReminder(Boolean(v))} /> Recordatorio 24 h antes</label>
          </section>
          <div className="rounded-md bg-accent/40 p-3 text-sm"><p className="text-xs text-muted-foreground uppercase">Resumen</p><p className="font-medium">{new Date(date + "T00:00:00").toLocaleDateString("es-PE", { weekday: "long", day: "2-digit", month: "2-digit", year: "numeric" })} · {slot}–{addMinutes(slot, svc.minutes)}</p><p className="text-xs text-muted-foreground">{chairLabel} · {pro} · {svc.label} · {formatCurrency(svc.price)}</p></div>
        </div>
        <SheetFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button><Button onClick={submit}><Check data-icon="inline-start" /> Agendar cita</Button></SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export function RescheduleDialog({ appointment, onClose }: { appointment: Appointment | null; onClose: () => void }) {
  const [reason, setReason] = React.useState("paciente");
  return (
    <Dialog open={Boolean(appointment)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader><DialogTitle>Reprogramar cita</DialogTitle><DialogDescription>{appointment?.code} · {appointment?.patient} · actual {appointment?.start}–{appointment?.end}</DialogDescription></DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Nueva fecha"><Input type="date" defaultValue="2026-09-19" /></Field>
          <Field label="Nueva hora"><Select defaultValue="10:30" items={slots.map((s) => ({ value: s, label: s }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{slots.map((s) => <SelectItem key={s} value={s} disabled={busy.has(s)}>{s}</SelectItem>)}</SelectContent></Select></Field>
          <Field label="Motivo" className="sm:col-span-2"><RadioGroup value={reason} onValueChange={(v) => setReason(String(v))} className="grid grid-cols-2 gap-2">{[["paciente", "Solicitud del paciente"], ["clinica", "Motivo de la clínica"]].map(([v, l]) => <label key={v} className={cn("flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm", reason === v && "border-primary bg-accent/40")}><RadioGroupItem value={v} /> {l}</label>)}</RadioGroup></Field>
          <label className="flex items-center gap-2 text-sm sm:col-span-2"><Checkbox defaultChecked /> Notificar al paciente por WhatsApp</label>
        </div>
        <DialogFooter><Button variant="outline" onClick={onClose}>Cancelar</Button><Button onClick={() => { toast.success(`Cita ${appointment?.code} reprogramada al 19/09/2026 10:30`); onClose(); }}><CalendarClock data-icon="inline-start" /> Reprogramar</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export const waitlist = [
  { name: "Renzo Valdivia Ortiz", wants: "Antes del 18/09 · tarde", reason: "Dolor pieza 4.8", phone: "+51 944 000 111" },
  { name: "Lucía Fernández Paz", wants: "Cualquier mañana", reason: "Control de brackets", phone: "+51 955 222 333" },
  { name: "Inv. Alpamayo S.A.C.", wants: "Semana del 21/09", reason: "Evaluación ocupacional x3", phone: "+51 (01) 442-8990" },
];
