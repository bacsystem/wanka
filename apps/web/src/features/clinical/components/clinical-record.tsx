"use client";

import { AlertTriangle, ArrowLeft, FileDown, Image as ImageIcon, PenLine, Plus, Printer, ShieldCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { EntityHeader } from "@/components/shared/entity-header";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { initials } from "@/lib/tenant";
import { cn } from "@/lib/utils";
import type { Customer } from "@/types/domain";
import { evolutions, initialOdontogram, lowerLeft, lowerRight, radiographs, toothNames, toothStateMeta, treatmentPlan, upperLeft, upperRight, type ToothState } from "../mocks/odontogram";

const tools = Object.keys(toothStateMeta) as ToothState[];

export function ClinicalRecord({ customer }: { customer: Customer }) {
  const [teeth, setTeeth] = React.useState<Record<number, ToothState>>(initialOdontogram);
  const [tool, setTool] = React.useState<ToothState>("caries");
  const [selected, setSelected] = React.useState<number>(36);
  const [tab, setTab] = React.useState("soap");
  const [billToPos, setBillToPos] = React.useState(true);
  const stateOf = (n: number) => teeth[n] ?? "sano";
  const cpo = Object.values(teeth).reduce((a, s) => ({ c: a.c + (s === "caries" ? 1 : 0), p: a.p + (s === "ausente" || s === "extraer" ? 1 : 0), o: a.o + (s === "resina" || s === "corona" || s === "implante" || s === "endodoncia" ? 1 : 0) }), { c: 0, p: 0, o: 0 });
  const done = treatmentPlan.filter((t) => t.status === "Completado").length + 3;

  function paint(n: number) {
    setSelected(n);
    setTeeth((t) => ({ ...t, [n]: tool }));
  }

  const Tooth = ({ n }: { n: number }) => {
    const st = stateOf(n); const m = toothStateMeta[st];
    return (
      <button type="button" onClick={() => paint(n)} aria-label={`Pieza ${n}: ${m.label}`} aria-pressed={selected === n} className={cn("flex h-14 w-9 flex-col items-center justify-between rounded-md border p-1 text-[10px] font-semibold transition-shadow hover:shadow-md sm:w-10", m.className, selected === n && "ring-2 ring-primary ring-offset-1 ring-offset-background")}>
        <span className="font-mono text-[11px] text-foreground">{n}</span>
        <span className="grid size-5 grid-cols-3 grid-rows-3 gap-px" aria-hidden><span /><span className={cn("rounded-[1px]", st === "sano" ? "bg-border" : "bg-current opacity-70")} /><span /><span className={cn("rounded-[1px]", st === "sano" ? "bg-border" : "bg-current opacity-70")} /><span className={cn("rounded-[1px]", st === "sano" ? "bg-border" : "bg-current")} /><span className={cn("rounded-[1px]", st === "sano" ? "bg-border" : "bg-current opacity-70")} /><span /><span className={cn("rounded-[1px]", st === "sano" ? "bg-border" : "bg-current opacity-70")} /><span /></span>
        <span>{st === "sano" ? "" : m.short}</span>
      </button>
    );
  };

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href={`/clientes/${customer.id}`} />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Ficha del paciente</Button>
      <EntityHeader
        avatar={<InitialsAvatar initials={initials(customer.name)} size="xl" shape="square" />}
        title={customer.name}
        chips={<><StatusBadge tone="danger" icon={AlertTriangle} label="Alergia severa: penicilina / amoxicilina" /><Badge variant="outline"><ShieldCheck className="size-3" /> NTS N.º 139-MINSA</Badge></>}
        meta={<p className="flex flex-wrap gap-x-3"><span className="font-mono">{customer.clinicalRecord ?? "HC-2024-0392"}</span><span>{customer.documentType} {customer.documentNumber}</span><span>28 años</span><span>Rímac EPS (copago 20%)</span></p>}
        actions={<><Button variant="outline" onClick={() => window.print()}><Printer data-icon="inline-start" /> Imprimir odontograma</Button><Button variant="outline"><FileDown data-icon="inline-start" /> Exportar HC PDF</Button><Button onClick={() => setTab("soap")}><Plus data-icon="inline-start" /> Nueva evolución</Button></>}
      />

      <div className="grid gap-4 xl:grid-cols-5">
        <SectionCard title="Odontograma FDI" className="xl:col-span-3" action={<div className="flex flex-wrap gap-2"><ToggleGroup value={["perm"]} variant="outline" size="sm"><ToggleGroupItem value="perm">Permanente (32)</ToggleGroupItem><ToggleGroupItem value="dec">Decidua (20)</ToggleGroupItem></ToggleGroup><ToggleGroup value={["evo"]} variant="outline" size="sm"><ToggleGroupItem value="ini">Inicial</ToggleGroupItem><ToggleGroupItem value="evo">Evolutivo</ToggleGroupItem><ToggleGroupItem value="alta">Final / alta</ToggleGroupItem></ToggleGroup></div>} contentClassName="p-4">
          <div className="mb-3 flex flex-wrap items-center gap-1.5">
            <span className="mr-1 text-xs font-medium text-muted-foreground uppercase">Herramientas</span>
            {tools.map((t) => (<button key={t} type="button" onClick={() => setTool(t)} aria-pressed={tool === t} className={cn("rounded-md border px-2 py-1 text-xs font-medium", toothStateMeta[t].className, tool === t && "ring-2 ring-primary")}>{toothStateMeta[t].label}</button>))}
          </div>
          <div className="overflow-x-auto">
            <div className="min-w-[720px]">
              <div className="mb-1 flex justify-between text-[10px] text-muted-foreground uppercase"><span>Derecha del paciente</span><span>Maxilar superior</span><span>Izquierda del paciente</span></div>
              <div className="flex items-center gap-1"><div className="flex gap-1">{upperRight.map((n) => <Tooth key={n} n={n} />)}</div><span className="mx-1 h-14 w-px bg-border" /><div className="flex gap-1">{upperLeft.map((n) => <Tooth key={n} n={n} />)}</div></div>
              <div className="my-2 border-t border-dashed text-center text-[10px] text-muted-foreground">línea de oclusión</div>
              <div className="flex items-center gap-1"><div className="flex gap-1">{lowerRight.map((n) => <Tooth key={n} n={n} />)}</div><span className="mx-1 h-14 w-px bg-border" /><div className="flex gap-1">{lowerLeft.map((n) => <Tooth key={n} n={n} />)}</div></div>
              <div className="mt-1 text-center text-[10px] text-muted-foreground uppercase">Mandíbula inferior</div>
            </div>
          </div>
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2 rounded-md bg-muted/40 px-3 py-2 text-xs"><span>Pieza seleccionada: <span className="font-mono font-semibold">{selected}</span> · {toothNames[selected] ?? "—"} · {toothStateMeta[stateOf(selected)].label}</span><span>Índice CPO-D: <span className="font-mono font-semibold">{cpo.c + cpo.p + cpo.o}</span> (C:{cpo.c} P:{cpo.p} O:{cpo.o})</span></div>
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <div className="rounded-md border p-3 text-sm"><p className="flex items-center gap-2 font-medium"><ImageIcon className="size-4 text-primary" /> Estudios radiográficos (RVG) <Badge variant="outline">{radiographs.length}</Badge></p><ul className="mt-2 flex flex-col gap-1 text-xs">{radiographs.map((r) => <li key={r.name} className="flex justify-between"><span>{r.name} <span className="text-muted-foreground">· {r.kind}</span></span><span className="font-mono text-muted-foreground">{formatDate(r.date)}</span></li>)}</ul></div>
            <div className="rounded-md border p-3 text-sm"><p className="flex items-center justify-between font-medium">Plan de tratamiento activo <span className="font-mono text-xs">{Math.round((done / (treatmentPlan.length + 3)) * 100)}%</span></p><Progress value={(done / (treatmentPlan.length + 3)) * 100} className="mt-2" /><p className="mt-2 text-xs text-muted-foreground">Fase correctiva y restauradora · próxima cita 18/09 09:30 (Dra. Ramos)</p></div>
          </div>
        </SectionCard>

        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList><TabsTrigger value="soap">Evolución SOAP</TabsTrigger><TabsTrigger value="plan">Plan de tratamiento</TabsTrigger><TabsTrigger value="prev">Evoluciones previas</TabsTrigger></TabsList></Tabs>
          {tab === "soap" ? (
            <SectionCard title={`Acto clínico · pieza ${selected}`} action={<Badge variant="outline" className="font-mono">K02.1</Badge>} contentClassName="flex flex-col gap-3 p-4">
              <Field label="Diagnóstico CIE-10"><Input defaultValue="K02.1 Caries de la dentina" /></Field>
              <Field label="Procedimiento del catálogo"><Input defaultValue="Restauración resina 3M Z350 XT · SRV-0012" /></Field>
              <Field label="S · Subjetivo (molestia del paciente)"><Textarea rows={2} placeholder="Refiere sensibilidad al frío en molar inferior izquierdo…" /></Field>
              <Field label="O · Objetivo (hallazgo clínico)"><Textarea rows={2} defaultValue={`Pieza ${selected} presenta cavitación oclusal activa con fondo reblandecido, vitalidad al frío (+) reversible, percusión (−).`} /></Field>
              <Field label="A · Apreciación / diagnóstico"><Textarea rows={2} defaultValue="Caries dentinaria activa sin compromiso pulpar." /></Field>
              <Field label="P · Plan y procedimiento realizado"><Textarea rows={2} defaultValue="Aislamiento absoluto, eliminación de tejido cariado, grabado selectivo, adhesivo universal, resina compuesta tono A2." /></Field>
              <label className="flex items-start gap-2 rounded-md border p-3 text-sm"><Checkbox checked={billToPos} onCheckedChange={(v) => setBillToPos(Boolean(v))} className="mt-0.5" /><span>Generar orden de cobro automática a caja POS (<span className="font-mono">S/ 180.00</span> con 20% copago EPS)</span></label>
              <p className="text-xs text-muted-foreground">Odontólogo tratante: Dra. Claudia Ramos P. (COP 34892)</p>
              <Button onClick={() => toast.success(`Evolución registrada · pieza ${selected}`, { description: billToPos ? "Orden de cobro enviada a CAJA-01" : undefined })}><PenLine data-icon="inline-start" /> Firmar y registrar evolución</Button>
            </SectionCard>
          ) : tab === "plan" ? (
            <SectionCard title="Plan de tratamiento" contentClassName="p-0">
              <ul className="divide-y">{treatmentPlan.map((t) => (<li key={t.step} className="flex items-center gap-3 px-4 py-2.5 text-sm"><span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted font-mono text-xs">{t.step}</span><div className="min-w-0 flex-1"><p className="truncate font-medium">{t.procedure}</p><p className="text-xs text-muted-foreground">{t.session} · prioridad {t.priority}</p></div><span className="font-mono text-xs tabular-nums">{t.cost ? formatCurrency(t.cost) : "—"}</span><StatusBadge tone={t.status === "Programado" ? "info" : "neutral"} label={t.status} /></li>))}</ul>
              <div className="flex justify-between border-t px-4 py-2 text-sm"><span className="text-muted-foreground">Presupuesto total</span><span className="font-mono font-semibold">{formatCurrency(treatmentPlan.reduce((s, t) => s + t.cost, 0))}</span></div>
            </SectionCard>
          ) : (
            <SectionCard title="Evoluciones previas" action={<Button variant="ghost" size="sm">Ver todas (8)</Button>} contentClassName="p-0">
              <ul className="divide-y">{evolutions.map((e, i) => (<li key={i} className="px-4 py-3 text-sm"><div className="flex items-center justify-between gap-2"><p className="font-medium">{e.procedure}{e.tooth ? <span className="ml-1 font-mono text-xs text-muted-foreground">pieza {e.tooth}</span> : null}</p><span className="font-mono text-xs tabular-nums">{formatCurrency(e.amount)}</span></div><p className="mt-0.5 text-xs text-muted-foreground">{e.notes}</p><p className="mt-1 text-xs text-muted-foreground">{e.professional} · {e.document} · {formatDate(e.date)}</p></li>))}</ul>
            </SectionCard>
          )}
        </div>
      </div>
    </div>
  );
}
