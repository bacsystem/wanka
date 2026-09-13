"use client";

import { AlertTriangle, ArrowDown, ArrowLeft, Camera, Check, FileCode, FileDown, Link2, Printer, RefreshCw, Route, ShieldCheck, Truck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { DataTable, TwoLine } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { SunatStatusBadge } from "@/features/documents/components/sunat-status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { guideDetailMock } from "../mocks/guide-detail";

type Guide = typeof guideDetailMock;

export function GuideDetail({ guide }: { guide: Guide }) {
  const [incident, setIncident] = React.useState(false);
  const [delivered, setDelivered] = React.useState(false);
  const timeline = delivered ? guide.timeline.map((t) => (t.pending ? { ...t, pending: false, current: true, tone: "success" as const, at: "2026-09-12T11:30:00", detail: "Confirmada por Dr. F. Zegarra · acta firmada" } : { ...t, current: false })) : guide.timeline;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/ventas/guias" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Guías de remisión</Button>
      <PageHeader
        eyebrow={`Ventas · Guías de remisión · ${guide.number}`}
        title={`Guía de remisión electrónica remitente ${guide.number}`}
        status={<span className="flex flex-wrap gap-1.5"><SunatStatusBadge status={guide.sunat} /><StatusBadge tone={delivered ? "success" : "warning"} dot label={delivered ? "Entregada" : guide.logistic} /></span>}
        description={`Emisión ${formatDate(guide.issuedAt)} ${guide.issuedAt.slice(11, 16)} · ${guide.modality} · ${guide.reason}`}
        actions={<>
          <Button variant="outline"><FileDown data-icon="inline-start" /> PDF</Button><Button variant="outline"><FileCode data-icon="inline-start" /> XML</Button><Button variant="outline"><ShieldCheck data-icon="inline-start" /> CDR</Button><Button variant="outline" onClick={() => window.print()}><Printer data-icon="inline-start" /> Imprimir</Button>
          <Button variant="outline" onClick={() => toast.success("Guía reenviada a SUNAT · CDR vigente")}><RefreshCw data-icon="inline-start" /> Reenviar a SUNAT</Button>
          <Button variant="outline" className="text-warning" onClick={() => setIncident(true)}><AlertTriangle data-icon="inline-start" /> Reportar incidencia</Button>
          <Button disabled={delivered} onClick={() => { setDelivered(true); toast.success(`Entrega de ${guide.number} confirmada en ${guide.destination.name.split(" (")[0]}`); }}><Check data-icon="inline-start" /> Confirmar entrega</Button>
        </>}
      />

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title="Trayecto y transporte" icon={Route} action={<span className="text-xs text-muted-foreground">Inicio de traslado {formatDate(guide.transferAt)}</span>} contentClassName="grid gap-4 p-4 md:grid-cols-2">
            <div className="rounded-md border p-3 text-sm"><p className="text-xs text-muted-foreground">Punto de partida (origen) · anexo <span className="font-mono">{guide.origin.annex}</span></p><p className="font-medium">{guide.origin.address}</p><p className="text-xs text-muted-foreground">{guide.origin.name}</p></div>
            <div className="rounded-md border p-3 text-sm"><p className="text-xs text-muted-foreground">Punto de llegada (destino) · anexo <span className="font-mono">{guide.destination.annex}</span></p><p className="font-medium">{guide.destination.address}</p><p className="text-xs text-muted-foreground">{guide.destination.name}</p></div>
            <dl className="grid grid-cols-2 gap-3 text-sm md:col-span-2 md:grid-cols-4">
              <div><dt className="text-xs text-muted-foreground">Conductor</dt><dd className="font-medium">{guide.driver.name}</dd><dd className="font-mono text-xs text-muted-foreground">Lic. {guide.driver.license}</dd></div>
              <div><dt className="text-xs text-muted-foreground">Vehículo / placa</dt><dd className="font-mono font-medium">{guide.vehicle.plate}</dd><dd className="text-xs text-muted-foreground">{guide.vehicle.model}</dd></div>
              <div><dt className="text-xs text-muted-foreground">Peso bruto total</dt><dd className="font-mono font-medium">{guide.weightKg.toFixed(2)} kg</dd><dd className="text-xs text-muted-foreground">KGM</dd></div>
              <div><dt className="text-xs text-muted-foreground">Bultos</dt><dd className="font-mono font-medium">{guide.packages}</dd><dd className="text-xs text-success">Precintados OK</dd></div>
            </dl>
          </SectionCard>

          <SectionCard title={`Bienes a trasladar · ${guide.items.length} ítems`} icon={Truck} action={<span className="text-xs text-muted-foreground">Total {guide.weightKg.toFixed(2)} kg · {guide.items.reduce((s, i) => s + i.qty, 0)} unidades</span>} contentClassName="p-0">
            <DataTable columns={[{ key: "sku", header: "SKU", cell: (i) => <span className="font-mono text-xs">{i.sku}</span> }, { key: "name", header: "Descripción del bien", cell: (i) => <TwoLine primary={<span className="font-medium">{i.name}</span>} secondary={i.detail} /> }, { key: "lot", header: "Lote / venc.", cell: (i) => <TwoLine primary={<span className="font-mono text-xs">{i.lot}</span>} secondary={i.expires} /> }, { key: "qty", header: "Cant.", align: "right", cell: (i) => <span className="font-mono">{i.qty}</span> }, { key: "unit", header: "U.M.", cell: (i) => i.unit }, { key: "kg", header: "Peso (kg)", align: "right", cell: (i) => <span className="font-mono">{i.kg.toFixed(2)}</span> }]} rows={guide.items} rowKey={(i) => i.sku} minWidth="720px" mobileCard={(i) => <div className="flex items-center gap-3"><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{i.name}</p><p className="font-mono text-xs text-muted-foreground">{i.sku} · {i.lot} · {i.expires}</p></div><span className="font-mono text-sm">{i.qty} {i.unit}</span></div>} />
            <p className="border-t px-4 py-2 text-xs text-muted-foreground">Todos los ítems cuentan con verificación pericial conforme a R.S. SUNAT 123-2022 y DIGEMID para dispositivos médicos.</p>
          </SectionCard>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Seguimiento de traslado" action={<StatusBadge tone={delivered ? "success" : "info"} dot label={delivered ? "Completado" : "En ruta activa"} />} contentClassName="p-4">
            <ol className="relative ml-1 flex flex-col gap-4 border-l pl-4">
              {timeline.map((t) => (
                <li key={t.label} className={cn("relative text-sm", t.pending && "opacity-60")}>
                  <span className={cn("absolute top-1 -left-[21px] size-2.5 rounded-full ring-2 ring-card", t.tone === "success" && "bg-success", t.tone === "warning" && "bg-warning", t.tone === "neutral" && "bg-muted-foreground", t.pending && "bg-border")} />
                  <p className="font-medium">{t.label}{t.current ? <Tag tone="primary" label="Actual" className="ml-2" /> : null}</p>
                  <p className="text-xs text-muted-foreground">{t.detail}</p>
                  {t.at ? <p className="font-mono text-[11px] text-muted-foreground">{formatDate(t.at)} {t.at.slice(11, 16)}</p> : null}
                </li>
              ))}
            </ol>
          </SectionCard>
          <SectionCard title="Documentos relacionados" icon={Link2} contentClassName="p-0">
            <ul className="divide-y text-sm">{guide.related.map((r) => <li key={r.label}><Link href={r.href} className="flex items-center justify-between px-4 py-2.5 hover:bg-muted/40"><span>{r.label}</span><ArrowDown className="size-3.5 -rotate-90 text-muted-foreground" /></Link></li>)}</ul>
          </SectionCard>
          <SectionCard title="QR y hash SUNAT" contentClassName="flex items-center gap-4 p-4">
            <div className="grid size-24 shrink-0 grid-cols-6 gap-px rounded border p-1" aria-label="Código QR">{Array.from({ length: 36 }).map((_, i) => <span key={i} className={cn("rounded-[1px]", (i * 5 + 2) % 4 < 2 ? "bg-foreground" : "bg-transparent")} />)}</div>
            <div className="min-w-0 text-xs text-muted-foreground"><p>Representación impresa de la GRE. Consulte en sunat.gob.pe</p><p className="mt-1 font-mono break-all">Hash: {guide.hash}</p></div>
          </SectionCard>
        </div>
      </div>

      <Dialog open={incident} onOpenChange={setIncident}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>Reportar incidencia en el traslado</DialogTitle><DialogDescription>{guide.number} · {guide.vehicle.plate} · {guide.driver.name}</DialogDescription></DialogHeader>
          <div className="grid gap-4">
            <Field label="Tipo de incidencia"><Select defaultValue="demora" items={[{ value: "demora", label: "Demora en ruta" }, { value: "dano", label: "Daño de mercadería" }, { value: "faltante", label: "Faltante en bultos" }, { value: "accidente", label: "Accidente / siniestro" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="demora">Demora en ruta</SelectItem><SelectItem value="dano">Daño de mercadería</SelectItem><SelectItem value="faltante">Faltante en bultos</SelectItem><SelectItem value="accidente">Accidente / siniestro</SelectItem></SelectContent></Select></Field>
            <Field label="Descripción"><Textarea rows={3} placeholder="Describe lo ocurrido, ubicación y hora aproximada…" /></Field>
            <label className="flex cursor-pointer flex-col items-center gap-1 rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground hover:bg-muted/40"><Camera className="size-5" /> Adjuntar foto (opcional)<input type="file" accept="image/*" className="sr-only" /></label>
            <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> Notificar al transportista</label>
            <label className="flex items-center gap-2 text-sm"><Checkbox /> Comunicar a SUNAT (solo si cambia ruta, vehículo o destinatario)</label>
          </div>
          <DialogFooter><Button variant="outline" onClick={() => setIncident(false)}>Cancelar</Button><Button onClick={() => { toast.success("Incidencia INC-2026-014 registrada y notificada"); setIncident(false); }}>Registrar incidencia</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
