"use client";

import { AlertTriangle, ArrowDown, ArrowLeft, Camera, Check, FileText, Plus, QrCode, ScanLine, Truck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { receptionMock } from "../mocks/reception";

type Reception = typeof receptionMock;
type LineState = "ok" | "faltante" | "danado";

export function ReceptionScreen({ data }: { data: Reception }) {
  const [lines, setLines] = React.useState(data.lines);
  const [states, setStates] = React.useState<Record<string, LineState>>({ r2: "faltante", r3: "danado" });
  const [scan, setScan] = React.useState("");
  const [lastScan, setLastScan] = React.useState<string | null>("Resina Filtek Z350 XT (lote L-9024)");
  const [confirm, setConfirm] = React.useState(false);
  const [agree, setAgree] = React.useState(false);
  const stateOf = (id: string): LineState => states[id] ?? "ok";
  const sent = lines.reduce((s, l) => s + l.sent, 0);
  const received = lines.reduce((s, l) => s + l.received, 0);
  const missing = lines.reduce((s, l) => s + Math.max(0, l.sent - l.received), 0);
  const damaged = lines.filter((l) => stateOf(l.id) === "danado").length;
  const stateMeta: Record<LineState, { label: string; tone: BadgeTone }> = { ok: { label: "Conforme OK", tone: "success" }, faltante: { label: "Faltante en bulto", tone: "warning" }, danado: { label: "Dañado / merma", tone: "danger" } };

  function doScan(e: React.FormEvent) {
    e.preventDefault();
    const hit = lines.find((l) => l.sku.toLowerCase() === scan.trim().toLowerCase() || l.lot.toLowerCase() === scan.trim().toLowerCase());
    if (hit) { setLastScan(`${hit.name} (lote ${hit.lot})`); toast.success(`Escaneado: ${hit.sku}`); } else toast.error("Código no pertenece a esta guía");
    setScan("");
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/inventario/almacenes" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Almacenes</Button>
      <PageHeader eyebrow={`Inventario · Almacenes · ${data.destination}`} title={`Recepción de traslado GRE ${data.guide}`} status={<StatusBadge tone="warning" dot label={data.status} />} description="Confronta los bienes recibidos con la guía de remisión electrónica y registra la conformidad del custodio." actions={<><Button variant="outline"><FileText data-icon="inline-start" /> Ver GRE (XML / PDF)</Button><Button variant="outline" className="text-warning" onClick={() => toast.info("Incidencia registrada para el transportista")}><AlertTriangle data-icon="inline-start" /> Reportar incidencia</Button><Button onClick={() => setConfirm(true)}><Check data-icon="inline-start" /> Confirmar ingreso a almacén</Button></>} />

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title="Trayecto fiscal SUNAT" icon={Truck} contentClassName="grid gap-3 p-4 text-sm sm:grid-cols-4">
            <div className="sm:col-span-1"><p className="text-xs text-muted-foreground">Origen → destino</p><p className="font-medium">{data.origin}</p><ArrowDown className="my-0.5 size-3 text-muted-foreground" /><p className="font-medium">{data.destination}</p></div>
            <div><p className="text-xs text-muted-foreground">Transportista</p><p className="font-medium">{data.carrier.name}</p><p className="font-mono text-xs text-muted-foreground">RUC {data.carrier.ruc}</p></div>
            <div><p className="text-xs text-muted-foreground">Conductor y placa</p><p className="font-medium">{data.driver.name}</p><p className="font-mono text-xs text-muted-foreground">{data.driver.plate} ({data.driver.license})</p></div>
            <div><p className="text-xs text-muted-foreground">Fecha de despacho</p><p className="font-medium">{formatDate(data.dispatchedAt)}</p><StatusBadge className="mt-1" tone="success" icon={QrCode} label="QR GRE validado" /></div>
          </SectionCard>

          <form onSubmit={doScan} className="flex flex-wrap items-center gap-2 rounded-lg border bg-card p-3">
            <div className="relative min-w-0 flex-1"><ScanLine className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-primary" /><Input value={scan} onChange={(e) => setScan(e.target.value)} placeholder="Escanear código QR / barras o escribir SKU / lote…" aria-label="Escanear código" className="pl-8" /></div>
            <Button type="submit" variant="outline"><QrCode data-icon="inline-start" /> Foco activo</Button>
            <Button type="button" variant="outline" onClick={() => toast.info("Cámara no disponible en esta demo")}><Camera data-icon="inline-start" /> Cámara</Button>
            {lastScan ? <p className="w-full text-xs text-success sm:w-auto"><Check className="mr-1 inline size-3" /> Escaneado con éxito: {lastScan}</p> : null}
          </form>

          <SectionCard title="Detalle de bienes en tránsito" action={<Badge variant="outline">{lines.length} renglones</Badge>} contentClassName="p-0">
            <div className="hidden overflow-x-auto md:block"><Table className="min-w-[880px]"><TableHeader><TableRow><TableHead className="pl-4">SKU / producto</TableHead><TableHead className="text-left">Lote y venc.</TableHead><TableHead className="text-right">Env.</TableHead><TableHead className="text-right">Recibido</TableHead><TableHead className="text-right">Dif.</TableHead><TableHead className="text-left">Ubicación destino</TableHead><TableHead className="pl-4">Estado físico</TableHead></TableRow></TableHeader><TableBody>
              {lines.map((l) => { const diff = l.received - l.sent; const st = stateOf(l.id); return (
                <TableRow key={l.id}>
                  <TableCell className="px-4 py-2"><p className="font-medium">{l.name}</p><p className="font-mono text-xs text-muted-foreground">{l.sku}{l.note ? ` · ${l.note}` : ""}</p></TableCell>
                  <TableCell className="px-2 py-2"><p className="font-mono text-xs">{l.lot}</p><p className="text-xs text-muted-foreground">Venc. {l.expires}</p></TableCell>
                  <TableCell className="px-2 py-2 text-right font-mono">{l.sent}</TableCell>
                  <TableCell className="px-2 py-2 text-right"><Input type="number" min={0} value={l.received} onChange={(e) => { const v = Math.max(0, Number(e.target.value) || 0); setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, received: v } : x))); if (v < l.sent && st === "ok") setStates((s) => ({ ...s, [l.id]: "faltante" })); if (v >= l.sent && st === "faltante") setStates((s) => ({ ...s, [l.id]: "ok" })); }} className="ml-auto h-7 w-16 text-right font-mono" aria-label={`Recibido ${l.sku}`} /></TableCell>
                  <TableCell className={cn("px-2 py-2 text-right font-mono font-semibold", diff < 0 && "text-destructive", diff > 0 && "text-warning")}>{diff === 0 ? "0" : diff}</TableCell>
                  <TableCell className="px-2 py-2"><Select value={l.location} onValueChange={(v) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, location: String(v) } : x)))} items={data.locations.map((x) => ({ value: x, label: x }))}><SelectTrigger className="w-44" aria-label={`Ubicación ${l.sku}`}><SelectValue /></SelectTrigger><SelectContent>{data.locations.map((x) => <SelectItem key={x} value={x}>{x}</SelectItem>)}</SelectContent></Select></TableCell>
                  <TableCell className="px-4 py-2"><Select value={st} onValueChange={(v) => setStates((s) => ({ ...s, [l.id]: v as LineState }))} items={Object.entries(stateMeta).map(([v, m]) => ({ value: v, label: m.label }))}><SelectTrigger className="w-40" aria-label={`Estado ${l.sku}`}><SelectValue /></SelectTrigger><SelectContent>{Object.entries(stateMeta).map(([v, m]) => <SelectItem key={v} value={v}>{m.label}</SelectItem>)}</SelectContent></Select></TableCell>
                </TableRow>
              ); })}
            </TableBody></Table></div>
            <ul className="divide-y md:hidden">
              {lines.map((l) => { const diff = l.received - l.sent; const st = stateOf(l.id); return (
                <li key={l.id} className="flex flex-col gap-2 px-4 py-3">
                  <div className="flex items-start justify-between gap-2"><div className="min-w-0"><p className="text-sm font-medium">{l.name}</p><p className="font-mono text-xs text-muted-foreground">{l.sku} · {l.lot} · venc. {l.expires}</p></div><StatusBadge tone={stateMeta[st].tone} label={stateMeta[st].label} /></div>
                  <div className="flex items-center gap-3"><span className="text-xs text-muted-foreground">Enviado <span className="font-mono font-medium text-foreground">{l.sent}</span></span><label className="ml-auto flex items-center gap-2 text-xs text-muted-foreground">Recibido <Input type="number" min={0} value={l.received} onChange={(e) => { const v = Math.max(0, Number(e.target.value) || 0); setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, received: v } : x))); if (v < l.sent && st === "ok") setStates((s) => ({ ...s, [l.id]: "faltante" })); if (v >= l.sent && st === "faltante") setStates((s) => ({ ...s, [l.id]: "ok" })); }} className="h-11 w-20 text-center font-mono text-lg" aria-label={`Recibido ${l.sku}`} /></label><span className={cn("w-10 text-right font-mono font-semibold", diff < 0 && "text-destructive", diff > 0 && "text-warning")}>{diff === 0 ? "0" : diff}</span></div>
                  <Select value={l.location} onValueChange={(v) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, location: String(v) } : x)))} items={data.locations.map((x) => ({ value: x, label: x }))}><SelectTrigger className="w-full" aria-label={`Ubicación ${l.sku}`}><SelectValue /></SelectTrigger><SelectContent>{data.locations.map((x) => <SelectItem key={x} value={x}>{x}</SelectItem>)}</SelectContent></Select>
                </li>
              ); })}
            </ul>
            <div className="flex flex-wrap items-center justify-between gap-2 border-t p-3 text-xs text-muted-foreground"><span>El recálculo de diferencias actualiza la valorización de guías rechazadas o en merma.</span><Button size="xs" variant="outline"><Plus data-icon="inline-start" /> Agregar ítem no documentado</Button></div>
          </SectionCard>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Balance de recepción" contentClassName="grid grid-cols-2 gap-3 p-4">
            {[["Total despachado", sent, ""], ["Conformes OK", received - damaged, "text-success"], ["Faltantes", missing, missing ? "text-warning" : ""], ["Dañados / merma", damaged, damaged ? "text-destructive" : ""]].map(([k, v, c]) => <div key={String(k)} className="rounded-lg border bg-muted/30 p-3"><p className="text-xs font-medium text-muted-foreground">{k}</p><p className={cn("text-2xl font-bold tabular-nums", String(c))} data-testid={String(k) === "Faltantes" ? "missing" : undefined}>{v}</p></div>)}
          </SectionCard>
          <SectionCard title="Conformidad del custodio" contentClassName="flex flex-col gap-3 p-4">
            <Field label="Nombre del receptor autorizado"><Input defaultValue="Dr. Fernando Zegarra" /></Field>
            <Field label="Cargo de almacén"><Input defaultValue="Responsable de sede San Isidro" /></Field>
            <label className="flex items-start gap-2 text-sm"><Checkbox checked={agree} onCheckedChange={(v) => setAgree(Boolean(v))} className="mt-0.5" /> Declaro conformidad física y pericial de los bienes recibidos según R.S. SUNAT 123-2022.</label>
            <Field label="Observaciones de recepción / reclamo"><Textarea rows={3} defaultValue="Se levanta observación por 1 caja faltante de agujas dentales y 1 ampolla de lidocaína fisurada. Se suscribe guía con sello condicionado para reintegro por Logística Express." /></Field>
            <Button disabled={!agree} onClick={() => setConfirm(true)}><Check data-icon="inline-start" /> Confirmar ingreso a almacén</Button>
          </SectionCard>
        </div>
      </div>

      <Dialog open={confirm} onOpenChange={setConfirm}>
        <DialogContent>
          <DialogHeader><DialogTitle>Confirmar ingreso a almacén</DialogTitle><DialogDescription>Documento GRE {data.guide}</DialogDescription></DialogHeader>
          <dl className="grid gap-2 text-sm"><div className="flex justify-between"><dt className="text-muted-foreground">Unidades conformadas</dt><dd className="font-mono">{received - damaged} de {sent}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Faltantes con acta</dt><dd className="font-mono">{missing}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Dañados / merma</dt><dd className="font-mono">{damaged}</dd></div></dl>
          <p className="text-xs text-muted-foreground">Se emitirá el acta de recepción con firma digital del custodio y se transmitirá la conformidad con observación al buzón SUNAT del transportista (RUC {data.carrier.ruc}).</p>
          <DialogFooter><Button variant="outline" onClick={() => setConfirm(false)}>Cancelar</Button><Button onClick={() => { toast.success(`GRE ${data.guide} recepcionada · stock actualizado en ${data.destination}`); setConfirm(false); }}>Firmar y registrar en kardex</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
