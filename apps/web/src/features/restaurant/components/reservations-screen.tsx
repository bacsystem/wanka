"use client";

import { CalendarPlus, MessageCircle, Phone } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import type { Reservation } from "../mocks/delivery";

const meta: Record<Reservation["status"], { label: string; tone: BadgeTone }> = { confirmada: { label: "Confirmada", tone: "success" }, pendiente: { label: "Pendiente", tone: "warning" }, no_show: { label: "No show", tone: "danger" }, sentada: { label: "En mesa", tone: "info" } };
const slots = ["12:00", "12:30", "13:00", "13:30", "14:00", "14:30", "15:00", "19:00", "19:30", "20:00", "20:30", "21:00", "21:30", "22:00"];

export function ReservationsScreen({ reservations: initial }: { reservations: Reservation[] }) {
  const [rows, setRows] = React.useState(initial);
  const [open, setOpen] = React.useState(false);
  const [people, setPeople] = React.useState("4");
  const [time, setTime] = React.useState("20:00");
  const suggested = Number(people) <= 2 ? "M1 / B2" : Number(people) <= 4 ? "M5" : Number(people) <= 6 ? "M12" : "T4 + T6 (unir)";
  return (
    <>
      <div className="grid gap-4 xl:grid-cols-[18rem_1fr]">
        <SectionCard title="Franjas del día" contentClassName="p-3">
          <ul className="grid grid-cols-4 gap-1.5 xl:grid-cols-2">{slots.map((s) => { const n = rows.filter((r) => r.time === s).length; return <li key={s}><button type="button" onClick={() => { setTime(s); setOpen(true); }} className={cn("flex w-full items-center justify-between rounded-md border px-2 py-1.5 text-xs hover:bg-muted/40", n >= 2 && "border-warning/50 bg-warning/10")}><span className="font-mono">{s}</span><span className="text-muted-foreground">{n ? `${n} res.` : "libre"}</span></button></li>; })}</ul>
        </SectionCard>
        <SectionCard title="Reservas de hoy" action={<Button size="sm" onClick={() => setOpen(true)}><CalendarPlus data-icon="inline-start" /> Nueva reserva</Button>} contentClassName="p-0">
          <ul className="divide-y">{[...rows].sort((a, b) => a.time.localeCompare(b.time)).map((r) => <li key={r.id} className="flex flex-wrap items-center gap-3 px-4 py-3 text-sm"><span className="w-12 font-mono font-semibold">{r.time}</span><div className="min-w-0 flex-1"><p className="font-medium">{r.name} <span className="font-normal text-muted-foreground">· {r.people} personas · {r.area}</span></p><p className="text-xs text-muted-foreground">{r.phone}{r.table ? ` · mesa ${r.table}` : " · sin mesa asignada"}{r.deposit ? ` · adelanto S/ ${r.deposit}` : ""}</p></div><StatusBadge tone={meta[r.status].tone} dot label={meta[r.status].label} /><div className="flex gap-1">{r.status === "pendiente" ? <Button size="xs" onClick={() => { setRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, status: "confirmada", table: x.table ?? "M5" } : x))); toast.success(`Reserva de ${r.name} confirmada · mesa asignada`); }}>Confirmar</Button> : null}<Button size="icon-xs" variant="ghost" aria-label="WhatsApp" onClick={() => toast.success("Recordatorio enviado por WhatsApp")}><MessageCircle /></Button><Button size="icon-xs" variant="ghost" aria-label="Llamar"><Phone /></Button></div></li>)}</ul>
        </SectionCard>
      </div>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-lg">
          <SheetHeader><SheetTitle>Nueva reserva</SheetTitle><SheetDescription>Confirmación automática por WhatsApp; la mesa se sugiere según el aforo.</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4 sm:grid-cols-2">
            <Field label="Nombre" className="sm:col-span-2"><Input placeholder="Nombre del cliente o empresa" /></Field>
            <Field label="Teléfono / WhatsApp"><Input placeholder="+51 9xx xxx xxx" /></Field>
            <Field label="Personas"><Input type="number" min={1} value={people} onChange={(e) => setPeople(e.target.value)} className="font-mono" /></Field>
            <Field label="Fecha"><Input type="date" defaultValue="2026-09-12" /></Field>
            <Field label="Hora"><Select value={time} onValueChange={(v) => setTime(String(v))} items={slots.map((s) => ({ value: s, label: s }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{slots.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}</SelectContent></Select></Field>
            <Field label="Ambiente preferido"><Select defaultValue="salon" items={[{ value: "salon", label: "Salón principal" }, { value: "terraza", label: "Terraza" }, { value: "barra", label: "Barra" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="salon">Salón principal</SelectItem><SelectItem value="terraza">Terraza</SelectItem><SelectItem value="barra">Barra</SelectItem></SelectContent></Select></Field>
            <Field label="Mesa sugerida"><div className="flex h-8 items-center rounded-md border bg-muted/40 px-2.5 font-mono text-sm">{suggested}</div></Field>
            <Field label="Adelanto (opcional)"><Input placeholder="S/ 0.00" className="font-mono" /></Field>
            <Field label="Notas" className="sm:col-span-2"><Textarea rows={2} placeholder="Cumpleaños, silla para bebé, alergias…" /></Field>
            <label className="flex items-center gap-2 text-sm sm:col-span-2"><Checkbox defaultChecked /> Enviar confirmación por WhatsApp</label>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => setOpen(false)}>Cancelar</Button><Button onClick={() => { setRows((rs) => [...rs, { id: `n${Date.now()}`, time, name: "Nueva reserva", people: Number(people) || 2, phone: "—", status: "confirmada", area: "Salón principal", table: suggested.split(" ")[0] }]); toast.success(`Reserva creada para las ${time} · mesa ${suggested.split(" ")[0]}`); setOpen(false); }}>Guardar reserva</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  );
}
