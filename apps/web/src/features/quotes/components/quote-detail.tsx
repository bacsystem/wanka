"use client";

import { ArrowLeft, Check, Copy, FileDown, Landmark, Mail, MessageCircle, Pencil, Plus, Receipt, Save, Trash2 } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatusBadge } from "@/components/shared/status-badge";
import { catalogMock } from "@/features/pos/mocks/catalog";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { quoteDetailMock } from "../mocks/quote-detail";
import type { QuoteLine } from "../mocks/quote-detail";

type Quote = typeof quoteDetailMock;
const r2 = (n: number) => Math.round(n * 100) / 100;
const lineTotal = (l: QuoteLine) => r2(l.qty * l.unitValue * (1 - l.discountPct / 100) * (l.affectation === "10" ? 1.18 : 1));

export function QuoteDetail({ quote }: { quote: Quote }) {
  const [lines, setLines] = React.useState<QuoteLine[]>(quote.lines);
  const [editing, setEditing] = React.useState<string | null>(null);
  const [adding, setAdding] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [notes, setNotes] = React.useState(quote.notes);
  const [status, setStatus] = React.useState<"aprobada" | "facturada">(quote.status);

  const gross = lines.reduce((s, l) => s + l.qty * l.unitValue * (1 - l.discountPct / 100), 0);
  const taxable = r2(gross - quote.globalDiscount);
  const igv = r2(taxable * 0.18);
  const total = r2(taxable + igv);
  const detraction = r2(total * quote.detraction.rate / 100);
  const retention = r2(total * quote.retention.rate / 100);
  const daysLeft = Math.round((new Date(quote.expiresAt).getTime() - new Date("2026-09-12").getTime()) / 86_400_000);

  function update(id: string, patch: Partial<QuoteLine>) { setLines((ls) => ls.map((l) => (l.id === id ? { ...l, ...patch } : l))); }
  function addFromCatalog(itemId: string) {
    const it = catalogMock.find((c) => c.id === itemId)!;
    setLines((ls) => [...ls, { id: `n${Date.now()}`, code: it.sku, description: it.name, detail: it.note ?? it.category, qty: 1, unit: it.kind === "servicio" ? "ZZ" : "NIU", unitValue: r2(it.price / 1.18), discountPct: 0, affectation: it.taxable ? "10" : "20" }]);
    setAdding(false); setQuery("");
  }
  const results = query.trim() ? catalogMock.filter((c) => c.name.toLowerCase().includes(query.toLowerCase()) || c.sku.toLowerCase().includes(query.toLowerCase())).slice(0, 5) : [];

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/ventas/proformas" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Cotizaciones y proformas</Button>
      <PageHeader
        eyebrow={`Ventas · Cotizaciones · ${quote.code}`}
        title={`Cotización ${quote.code}`}
        status={status === "facturada" ? <StatusBadge tone="success" icon={Check} label="Facturada · F001-00000843" /> : <StatusBadge tone="success" label="Aprobada por cliente" />}
        description={`${quote.customer.name} · RUC ${quote.customer.ruc} · ${quote.customer.tags.join(" · ")} · ${quote.customer.site} · vendedor ${quote.seller}`}
        actions={<>
          <Button variant="outline" onClick={() => toast.success("Enviada por correo")}><Mail data-icon="inline-start" /> Correo</Button>
          <Button variant="outline" onClick={() => toast.success("Enviada por WhatsApp")}><MessageCircle data-icon="inline-start" /> WhatsApp</Button>
          <Button variant="outline"><FileDown data-icon="inline-start" /> PDF</Button>
          <Button variant="outline" onClick={() => toast.info("Duplicada como COT-2026-00129 (borrador)")}><Copy data-icon="inline-start" /> Duplicar</Button>
          <Button disabled={status === "facturada"} onClick={() => { setStatus("facturada"); toast.success("Factura F001-00000843 emitida y transmitida al OSE", { description: quote.customer.name }); }}><Receipt data-icon="inline-start" /> Aprobar y emitir factura F001</Button>
        </>}
      />

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        {[["Fecha de emisión", `${formatDate(quote.issuedAt)} ${quote.issuedAt.slice(11, 16)}`], ["Vencimiento", `${formatDate(quote.expiresAt)} · ${daysLeft > 0 ? `vence en ${daysLeft} días` : "vencida"}`], ["Moneda e IGV", "PEN (S/) · IGV 18%"], ["Forma de pago", quote.paymentTerms], ["Total cotizado", formatCurrency(total)]].map(([k, v], i) => (
          <div key={k} className={cn("rounded-lg border bg-card p-3", i === 4 && "col-span-2 border-primary/50 xl:col-span-1")}><p className="text-[11px] text-muted-foreground uppercase">{k}</p><p className={cn("mt-0.5 text-sm font-medium", i === 4 && "font-mono text-xl font-semibold tabular-nums")}>{v}</p></div>
        ))}
      </div>

      <ol className="flex items-center gap-2 overflow-x-auto rounded-lg border bg-card px-4 py-3">
        {quote.timeline.map((t, i) => {
          const done = t.done || (status === "facturada"); const current = status === "facturada" ? i === 4 : t.current;
          return (
            <li key={t.label} className="flex shrink-0 items-center gap-2">
              <span className={cn("flex size-6 items-center justify-center rounded-full border text-[11px]", done ? "border-primary bg-primary text-primary-foreground" : "text-muted-foreground", current && "ring-2 ring-primary/30")}>{done ? <Check className="size-3" /> : i + 1}</span>
              <span><span className={cn("block text-xs font-medium", !done && "text-muted-foreground")}>{t.label}</span><span className="block text-[10px] text-muted-foreground">{i === 4 && status === "facturada" ? "12/09 · F001-00000843" : t.at}</span></span>
              {i < quote.timeline.length - 1 ? <span className={cn("mx-2 h-px w-8", done ? "bg-primary" : "bg-border")} /> : null}
            </li>
          );
        })}
      </ol>

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title={`Ítems de la cotización · ${lines.length} servicios / insumos`} action={<span className="text-xs text-muted-foreground">Valores sin IGV · IGV desglosado en totales</span>} contentClassName="p-0">
            <div className="overflow-x-auto">
              <Table className="min-w-[860px]">
                <TableHeader><TableRow><TableHead className="pl-4">N°</TableHead><TableHead>Código</TableHead><TableHead>Descripción / tratamiento</TableHead><TableHead className="text-right">Cant.</TableHead><TableHead>U.M.</TableHead><TableHead className="text-right">V. unitario</TableHead><TableHead className="text-right">Desc.</TableHead><TableHead>Afect.</TableHead><TableHead className="text-right">Total (S/)</TableHead><TableHead className="pr-4 text-right"><span className="sr-only">Acciones</span></TableHead></TableRow></TableHeader>
                <TableBody>
                  {lines.map((l, i) => {
                    const ed = editing === l.id;
                    return (
                      <TableRow key={l.id}>
                        <TableCell className="pl-4 font-mono text-xs">{String(i + 1).padStart(2, "0")}</TableCell>
                        <TableCell className="font-mono text-xs">{l.code}</TableCell>
                        <TableCell className="max-w-72"><p className="font-medium">{l.description}</p><p className="truncate text-xs text-muted-foreground">{l.detail}</p></TableCell>
                        <TableCell className="text-right">{ed ? <Input value={l.qty} onChange={(e) => update(l.id, { qty: Number(e.target.value) || 0 })} className="h-7 w-16 text-right font-mono" aria-label="Cantidad" /> : <span className="font-mono tabular-nums">{l.qty}</span>}</TableCell>
                        <TableCell>{l.unit}</TableCell>
                        <TableCell className="text-right">{ed ? <Input value={l.unitValue} onChange={(e) => update(l.id, { unitValue: Number(e.target.value) || 0 })} className="h-7 w-24 text-right font-mono" aria-label="Valor unitario" /> : <span className="font-mono tabular-nums">{l.unitValue.toFixed(2)}</span>}</TableCell>
                        <TableCell className="text-right">{ed ? <Input value={l.discountPct} onChange={(e) => update(l.id, { discountPct: Number(e.target.value) || 0 })} className="h-7 w-14 text-right font-mono" aria-label="Descuento" /> : <span className="font-mono tabular-nums">{l.discountPct}%</span>}</TableCell>
                        <TableCell><Badge variant="outline">{l.affectation === "10" ? "10 Gravado" : "20 Exonerado"}</Badge></TableCell>
                        <TableCell className="text-right font-mono font-medium tabular-nums">{lineTotal(l).toLocaleString("en-US", { minimumFractionDigits: 2 })}</TableCell>
                        <TableCell className="pr-3 text-right"><div className="flex justify-end gap-0.5"><Button variant="ghost" size="icon-sm" aria-label={ed ? "Guardar línea" : "Editar línea"} onClick={() => setEditing(ed ? null : l.id)}>{ed ? <Check /> : <Pencil />}</Button><Button variant="ghost" size="icon-sm" aria-label="Quitar línea" onClick={() => setLines((ls) => ls.filter((x) => x.id !== l.id))}><Trash2 /></Button></div></TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
            <div className="border-t p-3">
              {adding ? (
                <div className="relative max-w-md"><SearchInput autoFocus value={query} onChange={setQuery} placeholder="Buscar en el catálogo dental (F2)…" aria-label="Buscar ítem del catálogo" className="sm:max-w-none" />{results.length ? <ul className="absolute z-10 mt-1 w-full rounded-md border bg-popover shadow-md">{results.map((c) => <li key={c.id}><button type="button" onClick={() => addFromCatalog(c.id)} className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-accent"><span>{c.name}</span><span className="font-mono text-xs text-muted-foreground">{c.sku} · {formatCurrency(c.price)}</span></button></li>)}</ul> : null}</div>
              ) : (
                <Button variant="outline" size="sm" onClick={() => setAdding(true)}><Plus data-icon="inline-start" /> Agregar ítem del catálogo dental <kbd className="ml-1 rounded border px-1 font-mono text-[10px]">F2</kbd></Button>
              )}
            </div>
          </SectionCard>

          <div className="grid gap-4 lg:grid-cols-2">
            <SectionCard title="Sujeto a detracción SPOT – SUNAT (12%)" icon={Landmark} contentClassName="flex flex-col gap-2 p-4 text-sm">
              <p className="text-xs text-muted-foreground">Código de servicio {quote.detraction.code} “{quote.detraction.label}”. Cta. Banco de la Nación <span className="font-mono">{quote.detraction.account}</span>.</p>
              <dl className="flex flex-col gap-1"><div className="flex justify-between"><dt className="text-muted-foreground">Monto detracción (12%)</dt><dd className="font-mono tabular-nums">{formatCurrency(detraction)}</dd></div><div className="flex justify-between font-medium"><dt>Saldo neto a pagar a la clínica</dt><dd className="font-mono tabular-nums">{formatCurrency(total - detraction)}</dd></div></dl>
              <p className="text-xs text-muted-foreground">Incluye garantía clínica de 12 meses en restauraciones de resina.</p>
            </SectionCard>
            <SectionCard title="Totales" contentClassName="p-4 text-sm">
              <dl className="flex flex-col gap-1">
                <div className="flex justify-between"><dt className="text-muted-foreground">Operación gravada</dt><dd className="font-mono tabular-nums">{formatCurrency(gross)}</dd></div>
                <div className="flex justify-between"><dt className="text-muted-foreground">Operación inafecta / exonerada</dt><dd className="font-mono tabular-nums">{formatCurrency(0)}</dd></div>
                <div className="flex justify-between"><dt className="text-muted-foreground">Descuento global (convenio) <Badge variant="outline" className="ml-1">Promo</Badge></dt><dd className="font-mono whitespace-nowrap text-destructive tabular-nums">− {formatCurrency(quote.globalDiscount)}</dd></div>
                <div className="flex justify-between"><dt className="text-muted-foreground">Base imponible neta</dt><dd className="font-mono tabular-nums">{formatCurrency(taxable)}</dd></div>
                <div className="flex justify-between"><dt className="text-muted-foreground">IGV oficial (18%)</dt><dd className="font-mono tabular-nums">{formatCurrency(igv)}</dd></div>
                <Separator className="my-1" />
                <div className="flex items-baseline justify-between"><dt className="font-semibold">Importe total cotizado</dt><dd className="font-mono text-xl font-semibold tabular-nums">{formatCurrency(total)}</dd></div>
                <p className="text-[11px] text-muted-foreground uppercase">Incluye impuestos de ley</p>
                <p className="text-xs text-muted-foreground">Retención 3% (agente): − {formatCurrency(retention)} · neto a percibir {formatCurrency(total - retention)}</p>
              </dl>
            </SectionCard>
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Condiciones comerciales" action={<Badge variant="outline">Contrato marco</Badge>} contentClassName="p-4 text-sm">
            <dl className="grid gap-2 text-xs">{[["Validez de la oferta", `15 días (al ${formatDate(quote.expiresAt)})`], ["Forma de pago", quote.paymentTerms], ["Moneda de pago", "Soles (PEN – S/)"], ["Tipo de cambio", "C: 3.742 · V: 3.750"], ["Sedes válidas", "Miraflores (principal) · San Isidro"], ["Régimen de retención", `Aplica ${quote.retention.rate}% · el cliente emitirá CRE por ${formatCurrency(retention)}`]].map(([k, v]) => <div key={k}><dt className="text-muted-foreground">{k}</dt><dd className="font-medium">{v}</dd></div>)}</dl>
          </SectionCard>
          <SectionCard title="Historial de seguimiento" action={<Badge variant="outline">{quote.history.length} eventos</Badge>} contentClassName="p-0">
            <ol className="relative ml-4 flex flex-col gap-3 border-l py-4 pr-4 pl-4">{quote.history.map((h) => <li key={h.title} className="relative text-sm"><span className="absolute top-1 -left-[21px] size-2.5 rounded-full bg-primary ring-2 ring-card" /><p className="font-medium">{h.title} <span className="font-mono text-[11px] font-normal text-muted-foreground">{h.at}</span></p><p className="text-xs text-muted-foreground">{h.text}</p></li>)}</ol>
          </SectionCard>
          <SectionCard title="Notas y observaciones" action={<Button size="xs" variant="outline" onClick={() => toast.success("Notas guardadas")}><Save data-icon="inline-start" /> Guardar</Button>} contentClassName="p-4"><Textarea rows={4} value={notes} onChange={(e) => setNotes(e.target.value)} /></SectionCard>
        </div>
      </div>
    </div>
  );
}
