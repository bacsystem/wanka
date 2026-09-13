"use client";

import { Activity, AlertTriangle, Check, LogOut, Pill, Plus, Sparkles } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Field } from "@/components/shared/field";
import { DataTable } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Kennel, Medication, Vital } from "../mocks/hospital";

const areas = [{ id: "general", label: "Internamiento" }, { id: "uci", label: "UCI" }, { id: "aislamiento", label: "Aislamiento" }];
const kMeta: Record<Kennel["status"], { label: string; tone: BadgeTone; card: string }> = { ocupado: { label: "Ocupado", tone: "info", card: "border-primary/40 bg-accent/30" }, libre: { label: "Disponible", tone: "success", card: "border-success/40 bg-success/5" }, limpieza: { label: "En desinfección", tone: "warning", card: "border-warning/40 bg-warning/5" } };

export function HospitalScreen({ kennels: initial, vitals: v0, meds: m0 }: { kennels: Kennel[]; vitals: Vital[]; meds: Medication[] }) {
  const [kennels, setKennels] = React.useState(initial);
  const [area, setArea] = React.useState("general");
  const [selId, setSelId] = React.useState("K-01");
  const [vitals, setVitals] = React.useState(v0);
  const [meds, setMeds] = React.useState(m0);
  const [dischargeOpen, setDischargeOpen] = React.useState(false);
  const [vitalOpen, setVitalOpen] = React.useState(false);
  const [form, setForm] = React.useState({ temp: "38.5", fc: "100", fr: "24", kg: "32.4" });
  const sel = kennels.find((k) => k.id === selId);
  const p = sel?.patient;
  const days = p ? p.day : 0; const dayCost = 120; const consumables = 186.5;

  return (
    <>
      <div className="grid gap-4 xl:grid-cols-5">
        <SectionCard title="Tablero de kennels" className="xl:col-span-2" action={<span className="text-xs text-muted-foreground">{kennels.filter((k) => k.status === "ocupado").length} ocupados · {kennels.filter((k) => k.status === "libre").length} libres · {kennels.filter((k) => k.status === "limpieza").length} desinfección</span>} contentClassName="p-3">
          <Tabs value={area} onValueChange={(v) => setArea(String(v))}><TabsList>{areas.map((a) => <TabsTrigger key={a.id} value={a.id}>{a.label} <span className="ml-1 font-mono text-[11px] opacity-70">{kennels.filter((k) => k.area === a.id).length}</span></TabsTrigger>)}</TabsList></Tabs>
          <ul className="mt-3 grid grid-cols-2 gap-2">
            {kennels.filter((k) => k.area === area).map((k) => (
              <li key={k.id}>
                <KennelTile interactive={Boolean(k.patient)} onSelect={() => setSelId(k.id)} pressed={selId === k.id} label={`${k.id} ${kMeta[k.status].label}${k.patient ? ` ${k.patient.name}` : ""}`} className={cn("flex h-full w-full flex-col gap-1 rounded-lg border p-2.5 text-left text-xs", kMeta[k.status].card, selId === k.id && "ring-2 ring-primary")}>
                  <span className="flex items-center justify-between"><span className="font-mono font-semibold">{k.id}</span><StatusBadge tone={k.patient?.critical ? "danger" : kMeta[k.status].tone} dot label={k.patient?.critical ? "Crítico" : kMeta[k.status].label} /></span>
                  {k.patient ? <><span className="text-sm font-semibold">{k.patient.name} <span className="font-normal text-muted-foreground">· día {k.patient.day} de {k.patient.ofDays}</span></span><span className="truncate text-muted-foreground">{k.patient.breed} · {k.patient.kg} kg</span><span className="truncate">{k.patient.reason}</span><span className="text-muted-foreground"><Pill className="mr-1 inline size-3" />{k.patient.nextMed}</span>{k.patient.alert ? <span className="truncate text-destructive"><AlertTriangle className="mr-1 inline size-3" />{k.patient.alert}</span> : null}</> : k.status === "limpieza" ? <><span className="text-muted-foreground">Amonio cuaternario · 15 min restantes</span><Button size="xs" variant="outline" className="mt-1 w-fit" onClick={() => { setKennels((ks) => ks.map((x) => (x.id === k.id ? { ...x, status: "libre" } : x))); toast.success(`${k.id} habilitado`); }}><Sparkles data-icon="inline-start" /> Habilitar</Button></> : <><span className="text-muted-foreground">Limpio y esterilizado</span><Button size="xs" className="mt-1 w-fit" onClick={() => { toast.info(`Ingresar mascota en ${k.id}`); }}><Plus data-icon="inline-start" /> Ingresar</Button></>}
                </KennelTile>
              </li>
            ))}
          </ul>
        </SectionCard>

        <div className="flex min-w-0 flex-col gap-4 xl:col-span-3">
          {p ? (
            <>
              <SectionCard title={`Hoja clínica: ${p.name} (${sel!.id})`} action={<div className="flex flex-wrap items-center gap-2"><StatusBadge tone={p.critical ? "danger" : "success"} dot label={p.critical ? "Crítico" : "Evolución favorable"} /><Button size="sm" variant="outline" onClick={() => setDischargeOpen(true)}><LogOut data-icon="inline-start" /> Dar de alta</Button></div>} contentClassName="grid gap-2 p-4 text-sm sm:grid-cols-3">
                <div><p className="text-xs text-muted-foreground">Paciente</p><Link href={`/mascotas/${p.petId}`} className="font-medium hover:underline">{p.name}</Link><p className="text-xs text-muted-foreground">{p.breed} · {p.kg} kg</p></div>
                <div><p className="text-xs text-muted-foreground">Propietario</p><p className="font-medium">{p.owner}</p><p className="text-xs text-muted-foreground">{p.ownerDoc}</p></div>
                <div><p className="text-xs text-muted-foreground">Responsable</p><p className="font-medium">{p.vet}</p><p className="text-xs text-muted-foreground">Día {p.day} de {p.ofDays} · {p.reason}</p></div>
              </SectionCard>
              <SectionCard title="Monitor de constantes vitales" icon={Activity} action={<Button size="sm" onClick={() => setVitalOpen(true)}><Plus data-icon="inline-start" /> Registrar signos</Button>} contentClassName="p-0">
                <DataTable columns={[{ key: "time", header: "Hora", cell: (v) => <span className="font-mono">{v.time}</span> }, { key: "temp", header: "Temp (°C)", align: "right", cell: (v) => <span className={cn("font-mono", v.temp >= 39.3 && "font-semibold text-destructive")}>{v.temp.toFixed(1)}</span> }, { key: "fc", header: "FC", align: "right", cell: (v) => <span className="font-mono">{v.fc}</span> }, { key: "fr", header: "FR", align: "right", cell: (v) => <span className="font-mono">{v.fr}</span> }, { key: "kg", header: "Peso", align: "right", cell: (v) => <span className="font-mono">{v.kg.toFixed(1)}</span> }, { key: "mucosa", header: "Mucosas", cell: (v) => v.mucosa }, { key: "tllc", header: "TLLC", cell: (v) => v.tllc }, { key: "gluc", header: "Gluc.", align: "right", cell: (v) => <span className="font-mono">{v.gluc}</span> }, { key: "note", header: "Observación", cell: (v) => <span className="text-xs text-muted-foreground">{v.note}</span> }]} rows={vitals} rowKey={(v) => v.time} minWidth="720px" />
              </SectionCard>
              <SectionCard title="Plan terapéutico y medicación programada" icon={Pill} contentClassName="p-0">
                <ul className="divide-y">{meds.map((m) => <li key={m.id} className="flex items-center gap-3 px-4 py-2.5 text-sm"><Badge variant="outline" className="w-10 justify-center font-mono">{m.route}</Badge><div className="min-w-0 flex-1"><p className="font-medium">{m.drug}</p><p className="text-xs text-muted-foreground">{m.dose} · {m.shift}</p></div><span className="font-mono text-xs">{m.time}</span>{m.done ? <StatusBadge tone="success" icon={Check} label="Administrado" /> : <Button size="xs" onClick={() => { setMeds((ms) => ms.map((x) => (x.id === m.id ? { ...x, done: true } : x))); toast.success(`${m.drug.split(" ")[0]} administrado · firmado M.V. Morales`); }}>Confirmar</Button>}</li>)}</ul>
              </SectionCard>
            </>
          ) : null}
        </div>
      </div>

      <Dialog open={vitalOpen} onOpenChange={setVitalOpen}>
        <DialogContent className="sm:max-w-md"><DialogHeader><DialogTitle>Registrar signos vitales</DialogTitle><DialogDescription>{p?.name} · {sel?.id} · turno tarde</DialogDescription></DialogHeader>
          <div className="grid grid-cols-2 gap-3">{([["temp", "Temperatura (°C)"], ["fc", "FC (lpm)"], ["fr", "FR (rpm)"], ["kg", "Peso (kg)"]] as const).map(([k, l]) => <Field key={k} label={l}><Input value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} inputMode="decimal" className="font-mono" /></Field>)}</div>
          <DialogFooter><Button variant="outline" onClick={() => setVitalOpen(false)}>Cancelar</Button><Button onClick={() => { setVitals((vs) => [{ time: "16:00", temp: Number(form.temp), fc: Number(form.fc), fr: Number(form.fr), kg: Number(form.kg), mucosa: "Rosadas", tllc: "< 2 s", gluc: 96, note: "Registro turno tarde (M.V. Morales)" }, ...vs]); setVitalOpen(false); toast.success("Signos registrados"); }}>Guardar</Button></DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={dischargeOpen} onOpenChange={setDischargeOpen}>
        <DialogContent className="sm:max-w-md"><DialogHeader><DialogTitle>Dar de alta a {p?.name}</DialogTitle><DialogDescription>Liquidación de días y consumos enviada a caja POS.</DialogDescription></DialogHeader>
          <dl className="grid gap-1 text-sm"><div className="flex justify-between"><dt className="text-muted-foreground">Días de internamiento × {formatCurrency(dayCost)}</dt><dd className="font-mono">{days} · {formatCurrency(days * dayCost)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Fármacos y consumibles</dt><dd className="font-mono">{formatCurrency(consumables)}</dd></div><div className="flex justify-between border-t pt-1 font-semibold"><dt>Total a cobrar</dt><dd className="font-mono">{formatCurrency(days * dayCost + consumables)}</dd></div></dl>
          <DialogFooter><Button variant="outline" onClick={() => setDischargeOpen(false)}>Cancelar</Button><Button render={<Link href={`/ventas/nueva?cliente=${p?.petId === "p1" ? "u1" : "u2"}&items=c9`} />} nativeButton={false}>Alta y cobrar en POS</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

/** Occupied kennels are selectable buttons; free/cleaning ones are plain tiles so their action buttons are not nested inside a button. */
function KennelTile({ interactive, onSelect, pressed, label, className, children }: { interactive: boolean; onSelect: () => void; pressed: boolean; label: string; className: string; children: React.ReactNode }) {
  if (!interactive) return <div aria-label={label} className={className}>{children}</div>;
  return <button type="button" onClick={onSelect} aria-pressed={pressed} aria-label={label} className={className}>{children}</button>;
}
