"use client";

import { AlertTriangle, ArrowLeft, BedDouble, Bell, CalendarPlus, LayoutGrid, MessageCircle, PawPrint, PlusCircle, Printer, Receipt, Scissors, Send, ShoppingCart, Stethoscope, Syringe, UserRound } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Line, LineChart, ResponsiveContainer, Tooltip, YAxis } from "recharts";
import { Button } from "@/components/ui/button";
import { EntityHeader } from "@/components/shared/entity-header";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { Scale, Wallet, CalendarCheck, ClipboardList } from "lucide-react";
import type { Pet, Vaccine, VetEvolution } from "../mocks/pets";

const vMeta: Record<Vaccine["status"], { label: string; tone: BadgeTone }> = { vigente: { label: "Vigente", tone: "success" }, por_vencer: { label: "Vence en 12 días", tone: "warning" }, vencida: { label: "Vencida", tone: "danger" } };

export function PetDetail({ pet, vaccines, evolutions, grooming }: { pet: Pet; vaccines: Vaccine[]; evolutions: VetEvolution[]; grooming: { date: string; service: string; groomer: string; amount: number; notes: string }[] }) {
  const [tab, setTab] = React.useState("carne");
  const [vacOpen, setVacOpen] = React.useState(false);
  const expiring = vaccines.find((v) => v.status === "por_vencer");
  const vacCols: Column<Vaccine>[] = [
    { key: "n", header: "Biológico / vacuna", cell: (v) => <span className="font-medium">{v.name}</span> },
    { key: "a", header: "Aplicada", cell: (v) => <span className="font-mono">{formatDate(v.appliedAt)}</span> },
    { key: "l", header: "Lote · laboratorio", cell: (v) => <TwoLine primary={<span className="font-mono text-xs">{v.lot}</span>} secondary={v.lab} /> },
    { key: "v", header: "Veterinario", cell: (v) => v.vet },
    { key: "nx", header: "Próxima dosis", cell: (v) => <span className="font-mono">{formatDate(v.nextAt)}</span> },
    { key: "s", header: "Estado", cell: (v) => <StatusBadge tone={vMeta[v.status].tone} dot label={vMeta[v.status].label} /> },
    { key: "act", header: <span className="sr-only">Acciones</span>, align: "right", cell: (v) => v.status !== "vigente" ? <Button size="xs" variant="outline" onClick={() => setVacOpen(true)}>Aplicar refuerzo</Button> : null },
  ];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/mascotas" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Pacientes (mascotas)</Button>
      <EntityHeader
        className="lg:items-start"
        avatar={<span className="relative flex size-20 shrink-0 flex-col items-center justify-center gap-1 rounded-xl border-2 border-amber-300 bg-amber-100 text-amber-900 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200"><PawPrint className="size-8" strokeWidth={2} /><Tag tone="warning" solid label={pet.species} /></span>}
        title={pet.name}
        chips={<><StatusBadge tone="info" label={pet.breed} /><Tag label={pet.sex} /><Tag tone="info" label={`${pet.age} (${formatDate(pet.birth)})`} /></>}
        meta={<>
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1"><span className="flex items-center gap-1 font-semibold text-foreground"><Scale className="size-3.5" /> {pet.weightKg.toFixed(2)} kg</span><span>·</span><span className="font-mono">Chip: {pet.chip}</span><span>·</span><span className="font-mono">HC: {pet.hc}</span></p>
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-muted/50 px-3 py-2"><span className="flex items-center gap-1"><UserRound className="size-3.5" /> Propietario: <Link href={`/clientes/${pet.ownerId}`} className="font-semibold text-foreground hover:underline">{pet.owner}</Link></span><span className="font-mono">{pet.ownerDoc} (RENIEC OK)</span><span className="flex items-center gap-1 font-semibold text-primary"><MessageCircle className="size-3.5" /> {pet.ownerPhone}</span></p>
        </>}
        aside={pet.alerts.length ? <ul className="flex flex-wrap gap-1.5 lg:justify-end">{pet.alerts.map((a, i) => <li key={a}><StatusBadge tone={i === 0 ? "danger" : "warning"} icon={AlertTriangle} label={a} /></li>)}</ul> : null}
        actions={<><Button onClick={() => setTab("hc")}><Stethoscope data-icon="inline-start" /> Nueva consulta</Button><Button variant="outline" render={<Link href="/agenda" />} nativeButton={false}><CalendarPlus data-icon="inline-start" /> Agendar cita</Button><Button variant="outline" render={<Link href={`/ventas/nueva?cliente=${pet.ownerId}`} />} nativeButton={false}><ShoppingCart data-icon="inline-start" /> Cobrar en POS</Button><Button variant="outline" onClick={() => toast.info("Abriendo WhatsApp del propietario…")}><MessageCircle data-icon="inline-start" /> WhatsApp</Button><Button variant="ghost" onClick={() => window.print()}><Printer data-icon="inline-start" /> Imprimir carné</Button></>}
      />

      <StatGrid>
        <StatCard label="Historial consultas" value={String(pet.visits)} suffix="atenciones" icon={ClipboardList} hint={<span className="font-semibold text-primary">Paciente regular activo</span>} />
        <StatCard label="Última visita" value={formatDate(pet.lastVisit)} icon={CalendarCheck} tone="info" hint="Hace 16 días (Dermatología)" />
        <StatCard label="Saldo / deuda" value={formatCurrency(pet.debt)} icon={Wallet} tone="success" emphasizeValue hint={<span className="font-semibold text-primary">{pet.debt ? "Pendiente de cobro" : "Al día · Sin saldos pendientes"}</span>} />
        <div className="flex flex-col gap-2 rounded-lg border bg-card p-4"><div className="flex items-center justify-between"><span className="text-xs font-medium tracking-wide text-muted-foreground uppercase">Control de peso</span><span className="flex size-7 items-center justify-center rounded-md bg-accent text-accent-foreground"><Scale className="size-4" strokeWidth={1.5} /></span></div><div className="flex items-end gap-3"><span className="font-mono text-2xl font-semibold tabular-nums">{pet.weightKg.toFixed(1)} kg</span><div className="flex-1"><ResponsiveContainer width="100%" height="100%"><LineChart data={pet.weights}><YAxis hide domain={["dataMin - 0.5", "dataMax + 0.5"]} /><Tooltip formatter={(v) => [`${v} kg`, "Peso"]} labelFormatter={(l) => String(l)} contentStyle={{ fontSize: 11 }} /><Line type="monotone" dataKey="kg" stroke="var(--primary)" strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} /></LineChart></ResponsiveContainer></div></div><p className="text-xs text-muted-foreground">+{(pet.weights.at(-1)!.kg - pet.weights[0].kg).toFixed(1)} kg en 6 meses</p></div>
      </StatGrid>

      <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="min-w-0 max-w-full"><TabsList variant="card"><TabsTrigger value="resumen"><LayoutGrid /> Resumen general</TabsTrigger><TabsTrigger value="carne"><Syringe /> Carné de vacunas y desparasitación <span className="rounded bg-emerald-100 px-1.5 font-mono text-[10px] text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">5</span></TabsTrigger><TabsTrigger value="hc"><ClipboardList /> Historia clínica (SOAP) <span className="rounded bg-muted px-1.5 font-mono text-[10px]">{pet.visits}</span></TabsTrigger><TabsTrigger value="hosp"><BedDouble /> Hospitalización</TabsTrigger><TabsTrigger value="groom"><Scissors /> Peluquería / grooming</TabsTrigger><TabsTrigger value="cpe"><Receipt /> Comprobantes CPE</TabsTrigger></TabsList></Tabs>

      {tab === "carne" || tab === "resumen" ? (
        <>
          {expiring ? <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm dark:border-amber-900 dark:bg-amber-950/30"><p className="flex items-center gap-3"><span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300"><Bell className="size-4" /></span><span><span className="font-bold">1 vacuna próxima a vencer:</span> <span className="font-semibold text-amber-800 underline dark:text-amber-300">{expiring.name}</span><br /><span className="text-xs text-muted-foreground">Vence el <strong className="text-foreground">{formatDate(expiring.nextAt)} (en 12 días)</strong>. Se programó recordatorio automático vía WhatsApp al dueño ({pet.ownerPhone}) con 48h de antelación.</span></span></p><div className="flex flex-wrap gap-2"><Button className="font-semibold" onClick={() => toast.success(`Recordatorio enviado a ${pet.ownerPhone}`)}><Send data-icon="inline-start" /> Notificar ahora por WhatsApp</Button><Button className="font-semibold" onClick={() => setVacOpen(true)}><PlusCircle data-icon="inline-start" /> Aplicar refuerzo</Button></div></div> : null}
          <SectionCard title="Inmunizaciones y desparasitación (normativa SENASA)" action={<Button size="sm" onClick={() => setVacOpen(true)}><Syringe data-icon="inline-start" /> Registrar vacuna</Button>} contentClassName="p-0">
            <DataTable columns={vacCols} rows={vaccines} rowKey={(v) => v.name} minWidth="900px" mobileCard={(v) => <div className="flex items-center justify-between gap-3"><div className="min-w-0"><p className="truncate text-sm font-medium">{v.name}</p><p className="text-xs text-muted-foreground">{formatDate(v.appliedAt)} · próxima {formatDate(v.nextAt)}</p></div><StatusBadge tone={vMeta[v.status].tone} dot label={vMeta[v.status].label} /></div>} />
          </SectionCard>
        </>
      ) : null}
      {tab === "hc" ? (
        <div className="grid gap-4 xl:grid-cols-3">
          <SectionCard title="Nueva consulta (SOAP)" className="xl:col-span-2" contentClassName="grid gap-3 p-4 sm:grid-cols-4">
            <Field label="Peso (kg)"><Input defaultValue={pet.weightKg} className="font-mono" /></Field><Field label="Temperatura (°C)"><Input defaultValue="38.5" className="font-mono" /></Field><Field label="FC (lpm)"><Input defaultValue="88" className="font-mono" /></Field><Field label="FR (rpm)"><Input defaultValue="22" className="font-mono" /></Field>
            <Field label="Motivo de consulta" className="sm:col-span-4"><Input placeholder="Prurito, vómitos, control…" /></Field>
            <Field label="S · Subjetivo" className="sm:col-span-2"><Textarea rows={2} /></Field><Field label="O · Objetivo" className="sm:col-span-2"><Textarea rows={2} /></Field>
            <Field label="A · Diagnóstico" className="sm:col-span-2"><Input placeholder="Dermatitis atópica canina (L20)" /></Field><Field label="P · Plan y receta (productos del petshop)" className="sm:col-span-2"><Input placeholder="Apoquel 16 mg · 14 tab" /></Field>
            <label className="flex items-center gap-2 text-sm sm:col-span-4"><Checkbox defaultChecked /> Generar orden de cobro en POS y descontar productos del inventario</label>
            <div className="sm:col-span-4"><Button onClick={() => toast.success("Consulta registrada · orden enviada a caja")}>Firmar y registrar consulta</Button></div>
          </SectionCard>
          <SectionCard title="Evoluciones previas" contentClassName="p-0"><ul className="divide-y">{evolutions.map((e) => <li key={e.date} className="px-4 py-3 text-sm"><div className="flex justify-between gap-2"><p className="font-medium">{e.reason}</p><span className="font-mono text-xs">{formatCurrency(e.amount)}</span></div><p className="text-xs text-muted-foreground">{e.diagnosis} · {e.vitals.kg} kg · {e.vitals.temp} °C · FC {e.vitals.fc} · FR {e.vitals.fr}</p><p className="mt-1 text-xs">{e.plan}</p><p className="text-xs text-muted-foreground">{e.vet} · {formatDate(e.date)}</p></li>)}</ul></SectionCard>
        </div>
      ) : null}
      {tab === "hosp" ? <SectionCard title="Hospitalización" action={<Button size="sm" variant="outline" render={<Link href="/hospitalizacion" />} nativeButton={false}>Ver tablero de kennels</Button>} contentClassName="p-6 text-sm text-muted-foreground">{pet.id === "p1" ? <p>Internado actualmente en <span className="font-mono font-medium text-foreground">K-01</span> · día 2 de 3 · gastroenteritis hemorrágica. <Link href="/hospitalizacion" className="text-primary hover:underline">Abrir hoja clínica →</Link></p> : <p>Sin internamientos activos. Última hospitalización: 12/03/2025 (gastroenteritis, 2 días, alta sin complicaciones).</p>}</SectionCard> : null}
      {tab === "groom" ? <SectionCard title="Peluquería y grooming" action={<Button size="sm" variant="outline" render={<Link href="/grooming" />} nativeButton={false}>Agenda de grooming</Button>} contentClassName="p-0"><ul className="divide-y text-sm">{grooming.map((g) => <li key={g.date} className="flex items-center gap-3 px-4 py-3"><span className="font-mono text-xs">{formatDate(g.date)}</span><div className="flex-1"><p className="font-medium">{g.service}</p><p className="text-xs text-muted-foreground">{g.groomer} · {g.notes}</p></div><span className="font-mono">{formatCurrency(g.amount)}</span></li>)}</ul></SectionCard> : null}
      {tab === "cpe" ? <SectionCard title="Comprobantes del propietario" contentClassName="p-6 text-center text-sm text-muted-foreground"><Link href={`/clientes/${pet.ownerId}`} className="text-primary hover:underline">Ver comprobantes en la ficha de {pet.owner} →</Link></SectionCard> : null}

      <Sheet open={vacOpen} onOpenChange={setVacOpen}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-lg">
          <SheetHeader><SheetTitle>Registrar vacuna / desparasitación</SheetTitle><SheetDescription>Descuenta el biológico del inventario y genera el cobro.</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4">
            <Field label="Producto del inventario"><Select defaultValue="kc" items={[{ value: "kc", label: "Bordetella KC (Bronchicine) · stock 12" }, { value: "sx", label: "Séxtuple Zoetis · stock 20" }, { value: "rb", label: "Antirrábica MSD · stock 30" }, { value: "dp", label: "Drontal Plus · stock 45" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="kc">Bordetella KC (Bronchicine) · stock 12</SelectItem><SelectItem value="sx">Séxtuple Zoetis · stock 20</SelectItem><SelectItem value="rb">Antirrábica MSD · stock 30</SelectItem><SelectItem value="dp">Drontal Plus · stock 45</SelectItem></SelectContent></Select></Field>
            <div className="grid grid-cols-2 gap-3"><Field label="Lote"><Input placeholder="KC-5612" className="font-mono" /></Field><Field label="Vencimiento del biológico"><Input type="month" defaultValue="2027-08" /></Field><Field label="Fecha de aplicación"><Input type="date" defaultValue="2026-09-12" /></Field><Field label="Próxima dosis"><Input type="date" defaultValue="2027-09-12" /></Field></div>
            <Field label="Veterinario"><Input defaultValue="M.V. Rodrigo Morales · CMVP 8492" /></Field>
            <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> Descontar del stock y cobrar S/ 65.00 en POS</label>
            <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> Programar recordatorio WhatsApp 48 h antes de la próxima dosis</label>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => setVacOpen(false)}>Cancelar</Button><Button onClick={() => { toast.success("Vacuna registrada · carné actualizado"); setVacOpen(false); }}><Syringe data-icon="inline-start" /> Registrar</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  );
}
