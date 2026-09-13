"use client";

import { ArrowUpDown, Building2, Bus, Copy, Download, Eye, FileDown, Factory, Landmark, Mail, MessageCircle, Pencil, Printer, RefreshCw, School, Send, ShieldCheck, Zap } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Field } from "@/components/shared/field";
import { SearchInput } from "@/components/shared/toolbar";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { quoteLinesFor, type Quote, type QuoteStatus } from "../mocks/quotes";

const statusMeta: Record<QuoteStatus, { label: string; tone: BadgeTone }> = {
  aprobada: { label: "Aprobada por cliente", tone: "success" },
  enviada: { label: "Enviada", tone: "info" },
  borrador: { label: "Borrador interno", tone: "neutral" },
  vencida: { label: "Vencida", tone: "danger" },
};
const kindIcon = { aseguradora: Landmark, minera: Factory, colegio: School, transporte: Bus, otro: Building2 };

function daysUntil(iso: string, today = "2026-09-12") {
  return Math.round((new Date(iso).getTime() - new Date(today).getTime()) / 86_400_000);
}

export function QuotesBoard({ quotes }: { quotes: Quote[] }) {
  const [tab, setTab] = React.useState<string>("todas");
  const [query, setQuery] = React.useState("");
  const [converting, setConverting] = React.useState<Quote | null>(null);
  const [selectedId, setSelectedId] = React.useState(quotes[0]?.id);

  const counts = React.useMemo(() => {
    const c: Record<string, number> = { todas: quotes.length };
    for (const q of quotes) c[q.status] = (c[q.status] ?? 0) + 1;
    return c;
  }, [quotes]);

  const rows = quotes.filter((q) => {
    const s = query.trim().toLowerCase();
    return (tab === "todas" || q.status === tab) && (!s || q.code.toLowerCase().includes(s) || q.customerName.toLowerCase().includes(s) || q.customerRuc.includes(s));
  });

  const selected = rows.find((q) => q.id === selectedId) ?? rows[0];

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Tabs value={tab} onValueChange={(v) => setTab(String(v))}>
          <TabsList variant="pills">
            {(["todas", "aprobada", "enviada", "borrador", "vencida"] as const).map((k) => (
              <TabsTrigger key={k} value={k} className="capitalize">
                {k === "todas" ? "Todas" : statusMeta[k].label.split(" ")[0]}
                <span className={cn("ml-1.5 rounded-full px-1.5 font-mono text-[11px] tabular-nums", k === "vencida" ? "bg-rose-100 text-rose-700" : k === "aprobada" ? "bg-emerald-100 text-emerald-700" : "bg-muted text-muted-foreground")}>{counts[k] ?? 0}</span>
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <div className="flex items-center gap-2"><SearchInput placeholder="Buscar por cliente, RUC o N° cotización…" value={query} onChange={setQuery} className="sm:w-80 sm:max-w-none" /><Button variant="outline" size="icon-sm" aria-label="Exportar" onClick={() => toast.success("Listado exportado")}><Download /></Button></div>
      </div>

      <div className="grid items-start gap-4 xl:grid-cols-12">
      <div className="min-w-0 rounded-xl border bg-card shadow-xs xl:col-span-7">
      <div className="flex items-center justify-between gap-2 border-b px-4 py-2.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
        <span className="flex items-center gap-2">Listado operativo <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-foreground">{rows.length} registros</span></span>
        <span className="flex items-center gap-1">Filtro: orden descendente <ArrowUpDown className="size-3.5" /></span>
      </div>
      <ul className="divide-y">
        {rows.map((q) => {
          const Icon = kindIcon[q.customerKind];
          const meta = statusMeta[q.status];
          const days = daysUntil(q.expiresAt);
          const isSel = selected?.id === q.id;
          return (
            <li key={q.id} onClick={() => setSelectedId(q.id)} className={cn("flex min-w-0 cursor-pointer flex-col gap-3 p-4 transition-colors hover:bg-muted/40", isSel && "border-l-4 border-l-primary bg-accent/40 dark:bg-accent/20")}>
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <Link href={`/ventas/proformas/${q.code}`} onClick={(e) => e.stopPropagation()} className="font-mono text-sm font-bold tabular-nums hover:text-primary hover:underline">{q.code}</Link>
                  <StatusBadge tone={meta.tone} dot label={meta.label} />
                  <span className="text-muted-foreground">{formatDate(q.issuedAt)}</span>
                  {q.status === "vencida" ? (
                    <StatusBadge tone="danger" label={`Venció ${formatDate(q.expiresAt)}`} />
                  ) : days <= 1 ? (
                    <StatusBadge tone="danger" label={`Vence ${days === 0 ? "hoy" : "mañana"}`} />
                  ) : days <= 5 ? (
                    <StatusBadge tone="info" label={`Vence en ${days} días`} />
                  ) : (
                    <span className="text-muted-foreground">Vence {formatDate(q.expiresAt)}</span>
                  )}
                </div>
                <div className="text-right">
                  <p className="text-[11px] text-muted-foreground">{q.status === "borrador" ? "Estimado" : "Total neto"}</p>
                  <p className={cn("font-mono text-lg font-bold tabular-nums", q.status === "vencida" ? "text-muted-foreground line-through" : "text-primary")}>{formatCurrency(q.total)}</p>
                </div>
              </div>
              <div className="min-w-0">
                <h3 className="flex items-center gap-2 truncate text-[15px] font-bold"><Icon className="size-4 shrink-0 text-primary" strokeWidth={2} aria-hidden />{q.customerName}</h3>
                <p className="text-xs text-muted-foreground">RUC: {q.customerRuc} • {q.description}</p>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                <span className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  {q.seenByCustomer ? <><Eye className="size-3.5 text-primary" /> <span className="text-primary">Visto por cliente (web link)</span></> : q.status === "vencida" ? <span className="text-destructive">Venció hace {-days} días (re-cotizar)</span> : q.note ? <><Pencil className="size-3.5" /> {q.note}</> : <span className="italic">Sin novedades</span>}
                </span>
                <div className="flex flex-wrap gap-1.5" onClick={(e) => e.stopPropagation()}>
                {q.status === "aprobada" ? (
                  <Button size="sm" onClick={() => setConverting(q)}>
                    <Zap data-icon="inline-start" /> Facturar 1-clic
                  </Button>
                ) : q.status === "vencida" ? (
                  <Button size="sm" variant="outline" onClick={() => toast.success(`${q.code} reactivada por 7 días`)}>
                    <RefreshCw data-icon="inline-start" /> Reactivar oferta
                  </Button>
                ) : (
                  <Button size="sm" variant="outline" onClick={() => toast.success(`${q.code} enviada por correo`)}>
                    <Send data-icon="inline-start" /> {q.status === "borrador" ? "Enviar al cliente" : "Reenviar"}
                  </Button>
                )}
                <Button size="icon-sm" variant="outline" aria-label="Descargar PDF"><FileDown /></Button>
                <Button size="icon-sm" variant="outline" aria-label="Editar" render={<Link href={`/ventas/proformas/${q.code}`} />} nativeButton={false}><Pencil /></Button>
                <Button size="icon-sm" variant="outline" aria-label="Enviar por WhatsApp" onClick={() => toast.success("Enlace enviado por WhatsApp")}><MessageCircle /></Button>
                <Button size="icon-sm" variant="outline" aria-label="Duplicar" onClick={() => toast.info(`Duplicada como borrador`)}><Copy /></Button>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
      {rows.length === 0 ? <p className="py-12 text-center text-sm text-muted-foreground">No hay cotizaciones con ese filtro.</p> : null}
      <div className="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2 text-xs text-muted-foreground"><span>Mostrando {rows.length} de {quotes.length} cotizaciones</span></div>
      </div>

      {selected ? <QuoteSidePanel quote={selected} onConvert={() => setConverting(selected)} /> : null}
      </div>

      <ConvertDialog quote={converting} onClose={() => setConverting(null)} />
    </>
  );
}

/** Right-hand detail (Stitch master–detail). */
function QuoteSidePanel({ quote: q, onConvert }: { quote: Quote; onConvert: () => void }) {
  const lines = quoteLinesFor(q);
  const base = lines.reduce((s, l) => s + l.subtotal, 0);
  const igv = Math.round(base * 0.18 * 100) / 100;
  const meta = statusMeta[q.status];
  return (
    <aside className="flex min-w-0 flex-col gap-4 rounded-xl border bg-card p-4 shadow-xs xl:col-span-5" aria-label="Detalle de cotización">
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground"><ShieldCheck className="size-4" /></span>
          <div className="min-w-0"><p className="flex flex-wrap items-center gap-2 font-mono text-base font-bold">{q.code} <StatusBadge tone={meta.tone} label={meta.label.split(" ")[0]} /></p><p className="text-xs text-muted-foreground">Generado por Dr. Marco Salcedo (Dir. Médico)</p></div>
        </div>
        <div className="flex gap-1"><Button variant="outline" size="icon-sm" aria-label="Imprimir" onClick={() => toast.info("Imprimiendo…")}><Printer /></Button><Button variant="outline" size="icon-sm" aria-label="Descargar PDF"><Download /></Button></div>
      </div>
      <div className="flex flex-wrap items-start justify-between gap-3 rounded-lg bg-muted/50 p-3 text-xs">
        <div><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Empresa cliente</p><p className="text-sm font-bold">{q.customerName}</p><p className="text-muted-foreground">RUC: {q.customerRuc} • Lima, Perú</p><p className="text-muted-foreground">Contacto: Lic. Patricia Valdivia (RRHH)</p></div>
        <div className="text-right"><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Validez oferta</p><p className="text-sm font-bold text-primary">15 días calendario</p><p className="text-muted-foreground">Vence: {formatDate(q.expiresAt)}</p></div>
      </div>
      <div>
        <div className="mb-2 flex items-center justify-between text-[11px]"><span className="font-semibold tracking-wide uppercase">Condiciones comerciales B2B</span><span className="font-medium text-primary">Condición de pago: Factura 30 días</span></div>
        <div className="grid grid-cols-3 gap-2 text-center text-xs">{[["Moneda", "Soles (PEN)"], ["IGV", "Incluido (18%)"], ["Sede atención", "Miraflores & Surco"]].map(([k, v]) => <div key={k} className="rounded-lg bg-muted/50 px-2 py-2.5"><p className="text-[10px] text-muted-foreground">{k}</p><p className="font-bold">{v}</p></div>)}</div>
      </div>
      <div>
        <p className="mb-2 text-[11px] font-semibold tracking-wide uppercase">Detalle del presupuesto</p>
        <div className="overflow-hidden rounded-lg border">
          <Table>
            <TableHeader><TableRow><TableHead className="text-left">Servicio / tratamiento</TableHead><TableHead className="text-right">Cant.</TableHead><TableHead className="text-right">P. unit</TableHead><TableHead className="text-right">Subtotal</TableHead></TableRow></TableHeader>
            <TableBody>{lines.map((l) => <TableRow key={l.name}><TableCell className="px-3 py-2"><p className="font-semibold">{l.name}</p><p className="text-[11px] text-muted-foreground">{l.detail}</p></TableCell><TableCell className="px-2 py-2 text-right whitespace-nowrap">{l.qty}</TableCell><TableCell className="px-2 py-2 text-right font-mono whitespace-nowrap">{formatCurrency(l.unit)}</TableCell><TableCell className="px-3 py-2 text-right font-mono font-semibold whitespace-nowrap">{formatCurrency(l.subtotal)}</TableCell></TableRow>)}</TableBody>
          </Table>
        </div>
      </div>
      <dl className="rounded-lg bg-muted/50 p-3 text-xs">
        <div className="flex justify-between py-0.5 text-muted-foreground"><dt>Subtotal (valor venta):</dt><dd className="font-mono">{formatCurrency(base)}</dd></div>
        <div className="flex justify-between py-0.5 text-muted-foreground"><dt>I.G.V. (18.00% Ley Tributaria PE):</dt><dd className="font-mono">{formatCurrency(igv)}</dd></div>
        <div className="mt-1 flex items-baseline justify-between border-t pt-2"><dt className="text-sm font-bold uppercase">Total presupuestado:</dt><dd className="font-mono text-xl font-bold text-primary">{formatCurrency(q.total)}</dd></div>
      </dl>
      {q.seenByCustomer ? <div className="flex items-center justify-between gap-2 rounded-lg bg-accent/60 p-3 text-xs dark:bg-accent/20"><span className="flex items-center gap-2"><Eye className="size-4 text-primary" /><span><span className="block font-semibold">Cliente visualizó la propuesta</span><span className="text-muted-foreground">Hace 2 horas vía link web encriptado desde IP corporativa.</span></span></span><button type="button" className="font-semibold text-primary hover:underline" onClick={() => toast.info("Traza de visualización")}>Ver traza</button></div> : null}
      <div className="flex flex-col gap-2">
        {q.status === "aprobada" ? <Button size="lg" className="font-semibold" onClick={onConvert}><Zap data-icon="inline-start" /> Aprobar y emitir factura electrónica F001</Button> : q.status === "vencida" ? <Button size="lg" variant="outline" className="font-semibold" onClick={() => toast.success(`${q.code} reactivada por 7 días`)}><RefreshCw data-icon="inline-start" /> Reactivar oferta</Button> : <Button size="lg" className="font-semibold" onClick={() => toast.success(`${q.code} enviada al cliente`)}><Send data-icon="inline-start" /> {q.status === "borrador" ? "Enviar al cliente" : "Reenviar propuesta"}</Button>}
        <div className="grid grid-cols-2 gap-2">
          <Button variant="secondary" className="bg-teal-100 font-semibold text-teal-900 hover:bg-teal-200 dark:bg-teal-900/50 dark:text-teal-100" onClick={() => toast.success("Reenviado por WhatsApp")}><MessageCircle data-icon="inline-start" /> Reenviar WhatsApp</Button>
          <Button variant="secondary" className="bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted" onClick={() => toast.success("Correo enviado")}><Mail data-icon="inline-start" /> Enviar correo</Button>
        </div>
      </div>
    </aside>
  );
}

function ConvertDialog({ quote, onClose }: { quote: Quote | null; onClose: () => void }) {
  const [detraction, setDetraction] = React.useState(false);
  const total = quote?.total ?? 0;
  const detractionAmount = detraction ? total * 0.12 : 0;
  return (
    <Dialog open={Boolean(quote)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Conversión rápida a comprobante SUNAT</DialogTitle>
          <DialogDescription>
            {quote?.code} · {quote?.customerName} · {formatCurrency(total)}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Tipo de comprobante">
            <Select defaultValue="factura" items={[{ value: "factura", label: "Factura electrónica (01)" }, { value: "boleta", label: "Boleta de venta (03)" }]}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="factura">Factura electrónica (01)</SelectItem>
                <SelectItem value="boleta">Boleta de venta (03)</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field label="Serie / correlativo">
            <Input readOnly value="F001 – 00000843" className="font-mono" />
          </Field>
          <Field label="Forma de pago" className="sm:col-span-2">
            <Select defaultValue="credito30" items={[{ value: "contado", label: "Contado" }, { value: "credito30", label: "Crédito 30 días" }, { value: "credito60", label: "Crédito 60 días" }]}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="contado">Contado</SelectItem>
                <SelectItem value="credito30">Crédito 30 días</SelectItem>
                <SelectItem value="credito60">Crédito 60 días</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <div className="flex items-center justify-between rounded-md border p-3 sm:col-span-2">
            <div>
              <p className="text-sm font-medium">Detracción SPOT SUNAT (12%)</p>
              <p className="text-xs text-muted-foreground">Servicios &gt; S/ 700 · depósito en Banco de la Nación</p>
            </div>
            <Switch checked={detraction} onCheckedChange={setDetraction} aria-label="Aplicar detracción" />
          </div>
          <dl className="rounded-md bg-muted/40 p-3 text-sm sm:col-span-2">
            <div className="flex justify-between"><dt className="text-muted-foreground">Total comprobante</dt><dd className="font-mono tabular-nums">{formatCurrency(total)}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Detracción</dt><dd className="font-mono tabular-nums">{formatCurrency(detractionAmount)}</dd></div>
            <div className="flex justify-between font-medium"><dt>Neto a cobrar</dt><dd className="font-mono tabular-nums">{formatCurrency(total - detractionAmount)}</dd></div>
          </dl>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancelar</Button>
          <Button onClick={() => { toast.success(`Factura F001-00000843 emitida y transmitida al OSE`, { description: quote?.customerName }); onClose(); }}>
            <Send data-icon="inline-start" /> Emitir y transmitir
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
