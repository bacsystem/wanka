"use client";

import { ArrowLeft, Ban, CheckCircle2, Clock, Cloud, Copy, Download, FileCode, FileDown, FileMinus, FilePlus, History, Mail, MessageCircle, Receipt, ShieldCheck, Wallet } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EntityHeader } from "@/components/shared/entity-header";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { formatCurrency, formatDate, formatDocumentNumber } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { SalesDocumentDetail } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";
import { VoidDialog } from "./credit-note-wizard";
import { SunatStatusBadge } from "./sunat-status-badge";

const tenant = {
  name: "CLÍNICA DENTAL SONRISA S.A.C.",
  ruc: "20608941235",
  address: "Av. Larco 743, Of. 402 – Miraflores, Lima, Perú",
  phone: "(01) 445-8920",
  email: "facturacion@dentalsonrisa.pe",
};

function amountInWords(total: number) {
  // Placeholder legend; the real implementation converts the amount to words per SUNAT rules.
  const [int, dec] = total.toFixed(2).split(".");
  return `SON: ${Number(int).toLocaleString("es-PE").toUpperCase()} CON ${dec}/100 SOLES`;
}

export function DocumentDetail({ doc }: { doc: SalesDocumentDetail }) {
  const number = formatDocumentNumber(doc.series, doc.number);
  const label = documentTypeLabel[doc.type];
  const time = doc.issuedAt.slice(11, 16);
  const [voidOpen, setVoidOpen] = React.useState(false);

  return (
    <div className="flex flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      {/* Stitch top strip: breadcrumb + status chips */}
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
        <nav className="flex items-center gap-2 text-muted-foreground" aria-label="Ruta">
          <Link href="/ventas/comprobantes" className="flex items-center gap-1 font-semibold text-foreground hover:text-primary"><ArrowLeft className="size-4" /> Volver al listado</Link>
          <span>/</span><span>Ventas</span><span>/</span><Link href="/ventas/comprobantes" className="hover:text-primary">Comprobantes</Link><span>/</span><span className="font-semibold text-foreground">{number}</span>
        </nav>
        <div className="flex flex-wrap items-center gap-2">
          <SunatStatusBadge status={doc.sunatStatus} />
          <span className="flex items-center gap-1.5 rounded-md bg-muted px-2.5 py-1 font-medium"><Clock className="size-3.5 text-muted-foreground" /> Emitido el {formatDate(doc.issuedAt)} {time}</span>
          <span className="hidden items-center gap-1.5 rounded-md bg-muted px-2.5 py-1 font-medium lg:flex"><Cloud className="size-3.5 text-muted-foreground" /> OSE Bizlinks sincronizado</span>
          <span className="rounded-md bg-muted px-2.5 py-1 font-semibold">{doc.currency} ({doc.currency === "PEN" ? "S/" : "$"})</span>
        </div>
      </div>

      <EntityHeader
        avatar={<span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent text-primary"><Receipt className="size-5" /></span>}
        title={<>{label} electrónica {number}</>}
        chips={<Tag tone="primary" solid label="Oficial" />}
        meta={<p>Cliente: <strong className="font-semibold text-foreground uppercase">{doc.customerName}</strong> · {doc.customerDocument.length === 11 ? "RUC" : "DNI"}: {doc.customerDocument}</p>}
        actions={<>
          {doc.type !== "nota_credito" ? (
            <Button render={<Link href={`/ventas/comprobantes/${doc.id}/nota-credito`} />} nativeButton={false}><FileMinus data-icon="inline-start" /> Emitir nota de crédito</Button>
          ) : null}
          <div className="flex flex-wrap items-center gap-1 rounded-lg bg-muted p-1">
            <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs" onClick={() => toast.info("Descargando PDF…")}><FileDown data-icon="inline-start" className="text-rose-600" /> PDF</Button>
            <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs"><FileCode data-icon="inline-start" className="text-primary" /> XML</Button>
            <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs" disabled={!doc.cdr?.acceptedAt}><ShieldCheck data-icon="inline-start" className="text-primary" /> CDR</Button>
            <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs" onClick={() => toast.success("Enviado por WhatsApp")}><MessageCircle data-icon="inline-start" className="text-emerald-600" /> WhatsApp</Button>
            <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs" onClick={() => toast.success("Correo enviado")}><Mail data-icon="inline-start" /> Correo</Button>
            {doc.type !== "nota_credito" ? <Button variant="ghost" size="sm" className="bg-card text-xs font-semibold shadow-xs" render={<Link href={`/ventas/comprobantes/${doc.id}/nota-debito`} />} nativeButton={false}><FilePlus data-icon="inline-start" /> Nota de débito</Button> : null}
            <Button variant="ghost" size="icon-sm" className="bg-rose-50 text-rose-600 hover:bg-rose-100 hover:text-rose-700 dark:bg-rose-950/60" aria-label="Anular comprobante" onClick={() => setVoidOpen(true)}><Ban /></Button>
          </div>
        </>}
      />

      <div className="grid gap-4 xl:grid-cols-3">
        {/* Printed-style preview */}
        <Card className="gap-0 py-0 xl:col-span-2">
          <CardContent className="p-0">
            <div className="flex min-w-0 flex-col gap-4 border-b p-5 sm:flex-row sm:items-start sm:justify-between">
              <div className="text-sm">
                <p className="flex items-center gap-2 text-base font-bold uppercase"><span className="flex size-8 items-center justify-center rounded-md bg-primary font-mono text-sm text-primary-foreground">{tenant.name[0]}</span>{tenant.name}</p>
                <p className="mt-2 text-xs text-muted-foreground">RUC: {tenant.ruc}</p>
                <p className="text-xs text-muted-foreground">{tenant.address}</p>
                <p className="mt-1 text-xs text-muted-foreground">{tenant.phone} · {tenant.email}</p>
              </div>
              <div className="w-full shrink-0 rounded-lg bg-accent/60 p-4 text-center sm:w-64 dark:bg-muted">
                <p className="text-sm font-bold tabular-nums">R.U.C. {tenant.ruc}</p>
                <p className="mt-2 rounded bg-primary py-1.5 text-xs font-bold tracking-wide text-primary-foreground uppercase">{label} electrónica</p>
                <p className="mt-2 font-mono text-lg font-bold text-primary tabular-nums">{number}</p>
              </div>
            </div>

            <dl className="m-5 grid grid-cols-1 gap-x-6 gap-y-2.5 rounded-lg bg-muted/50 p-4 text-sm sm:grid-cols-2">
              <Row k="Señor(es) / razón social" v={doc.customerName} />
              <Row k={doc.customerDocument.length === 11 ? "RUC cliente" : "Documento"} v={doc.customerDocument} mono />
              <Row k="Dirección fiscal" v={doc.customerAddress ?? "—"} />
              <Row k="Fecha de emisión" v={formatDate(doc.issuedAt)} mono />
              <Row k="Fecha de vencimiento" v={doc.dueAt ? formatDate(doc.dueAt) : formatDate(doc.issuedAt)} mono />
              <Row k="Moneda" v={doc.currency === "PEN" ? "Soles (PEN) – S/" : "Dólares (USD) – $"} />
              <Row k="Forma de pago" v={doc.paymentTerms} />
              <Row k="Tipo de operación" v={doc.operationType} />
              {doc.relatedGuide ? <Row k="Guía de remisión" v={doc.relatedGuide} mono /> : null}
            </dl>

            <div className="overflow-x-auto">
              <Table className="min-w-[720px]">
                <TableHeader className="bg-muted/50">
                  <TableRow>
                    <TableHead className="pl-5">Código</TableHead>
                    <TableHead className="text-right">Cant.</TableHead>
                    <TableHead>U.M.</TableHead>
                    <TableHead>Descripción</TableHead>
                    <TableHead className="text-right">V. unit.</TableHead>
                    <TableHead className="text-right">Desc.</TableHead>
                    <TableHead className="pr-5 text-right">Total</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {doc.lines.map((l) => (
                    <TableRow key={l.code}>
                      <TableCell className="pl-5 font-mono text-xs tabular-nums">{l.code}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{l.quantity}</TableCell>
                      <TableCell>{l.unit}</TableCell>
                      <TableCell>
                        <p className="font-medium">{l.description}</p>
                        {l.detail ? <p className="text-xs text-muted-foreground">{l.detail}</p> : null}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{formatCurrency(l.unitValue, doc.currency)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{formatCurrency(l.discount, doc.currency)}</TableCell>
                      <TableCell className="pr-5 text-right font-mono font-medium tabular-nums">{formatCurrency(l.total, doc.currency)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            <div className="grid min-w-0 gap-4 border-t p-5 lg:grid-cols-5">
              <div className="flex flex-col gap-3 lg:col-span-3">
                <div>
                  <p className="text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">Importe en letras</p>
                  <p className="mt-1 rounded-md bg-muted/50 px-3 py-2 text-xs font-semibold uppercase">{amountInWords(doc.total)}</p>
                </div>
                <div className="flex items-start gap-4 rounded-lg bg-muted/50 p-3">
                  <div className="grid size-20 shrink-0 grid-cols-6 gap-px rounded border bg-card p-1" aria-label="Código QR">
                    {Array.from({ length: 36 }).map((_, i) => (
                      <span key={i} className={cn("rounded-[1px]", (i * 7 + 3) % 5 < 2 ? "bg-foreground" : "bg-transparent")} />
                    ))}
                  </div>
                  <div className="min-w-0 text-xs text-muted-foreground">
                    <p className="text-[11px] font-semibold tracking-wide uppercase">Código hash SHA-256</p>
                    {doc.cdr?.hash ? <p className="mt-0.5 truncate font-mono font-semibold text-foreground">{doc.cdr.hash}</p> : null}
                    <p className="mt-1">Certificado: CN={tenant.name.toUpperCase()}, OU=SISTEMAS, O=LLAMA.PE</p>
                    <p className="mt-0.5">Representación impresa. Consulte en sunat.gob.pe</p>
                  </div>
                </div>
              </div>
              <dl className="flex flex-col justify-end rounded-lg bg-muted/50 p-4 text-sm lg:col-span-2">
                <div className="flex justify-between py-1 text-muted-foreground"><dt>Op. gravada:</dt><dd className="font-mono tabular-nums">{formatCurrency(doc.taxableBase, doc.currency)}</dd></div>
                <div className="flex justify-between py-1 text-muted-foreground"><dt>I.G.V. (18%):</dt><dd className="font-mono tabular-nums">{formatCurrency(doc.igv, doc.currency)}</dd></div>
                <Separator className="my-2" />
                <div className="flex flex-wrap items-baseline justify-between gap-2 py-1"><dt className="text-sm font-bold uppercase">Total a pagar:</dt><dd className="font-mono text-xl font-bold whitespace-nowrap text-primary tabular-nums 2xl:text-2xl">{formatCurrency(doc.total, doc.currency)}</dd></div>
              </dl>
            </div>
          </CardContent>
        </Card>

        {/* Side: CDR, timeline, cobranza */}
        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Certificación y CDR" icon={ShieldCheck} action={doc.cdr ? <StatusBadge tone={doc.cdr.code === "0" ? "success" : "danger"} label={`Código ${doc.cdr.code}`} /> : null} contentClassName="p-4">
            {doc.cdr ? (
              <dl className="flex flex-col gap-2 text-sm">
                <div className="rounded-lg bg-emerald-50/70 p-3 dark:bg-emerald-950/30"><dt className="flex items-center gap-2 text-sm font-semibold"><CheckCircle2 className="size-4 text-emerald-600" /> {doc.cdr.code === "0" ? "Aceptada sin observaciones" : "Observada"}</dt><dd className="mt-0.5 pl-6 text-xs text-muted-foreground">{doc.cdr.code} - {doc.cdr.message}</dd></div>
                {doc.cdr.acceptedAt ? (
                  <div><dt className="text-xs text-muted-foreground">Fecha de aceptación</dt><dd className="font-mono tabular-nums">{formatDate(doc.cdr.acceptedAt)} {doc.cdr.acceptedAt.slice(11, 19)}</dd></div>
                ) : null}
                <div>
                  <dt className="text-xs text-muted-foreground">Hash / firma digital</dt>
                  <dd className="flex items-center gap-1">
                    <code className="truncate font-mono text-xs">{doc.cdr.hash}</code>
                    <Button variant="ghost" size="icon-xs" aria-label="Copiar hash" onClick={() => { navigator.clipboard?.writeText(doc.cdr!.hash); toast.success("Hash copiado"); }}>
                      <Copy />
                    </Button>
                  </dd>
                </div>
                <Button variant="outline" size="sm" className="mt-1" disabled={!doc.cdr.acceptedAt}>
                  <Download data-icon="inline-start" /> Descargar CDR oficial (.xml)
                </Button>
              </dl>
            ) : (
              <p className="text-sm text-muted-foreground">Aún sin respuesta de SUNAT. El comprobante está en cola del OSE.</p>
            )}
          </SectionCard>

          <SectionCard title="Línea de tiempo fiscal" icon={History} action={<span className="text-xs text-muted-foreground">{doc.timeline.length} eventos</span>} contentClassName="p-4">
            <ol className="relative flex flex-col gap-4 border-l pl-4">
              {doc.timeline.map((e, i) => (
                <li key={i} className="relative text-sm">
                  <span
                    className={cn(
                      "absolute -left-[21px] top-1 size-2.5 rounded-full ring-2 ring-card",
                      e.tone === "success" && "bg-success",
                      e.tone === "warning" && "bg-warning",
                      e.tone === "danger" && "bg-destructive",
                      e.tone === "neutral" && "bg-muted-foreground",
                    )}
                  />
                  <p className="font-medium">{e.label}</p>
                  {e.detail ? <p className="text-xs text-muted-foreground">{e.detail}</p> : null}
                  <p className="font-mono text-[11px] text-muted-foreground tabular-nums">{formatDate(e.at)} {e.at.slice(11, 19)}</p>
                </li>
              ))}
            </ol>
          </SectionCard>

          <SectionCard title="Detalles de cobranza" icon={Wallet} iconTone="success" action={<StatusBadge tone="success" label="Pagado" />} contentClassName="p-4">
            <dl className="flex flex-col gap-2 text-sm">
              <div className="flex justify-between"><dt className="text-muted-foreground">Medio de pago</dt><dd>{doc.paymentMethod ?? "—"}</dd></div>
              <div className="flex justify-between"><dt className="text-muted-foreground">Pagado</dt><dd className="font-mono tabular-nums">{formatCurrency(doc.paid, doc.currency)}</dd></div>
              <div className="flex justify-between"><dt className="text-muted-foreground">Saldo</dt><dd className="font-mono font-medium tabular-nums">{formatCurrency(doc.total - doc.paid, doc.currency)}</dd></div>
              <div className="flex items-center justify-between">
                <dt className="text-muted-foreground">Estado</dt>
                <dd>{doc.total - doc.paid <= 0 ? <StatusBadge tone="success" dot label="Cancelado" /> : <StatusBadge tone="warning" dot label="Pendiente" />}</dd>
              </div>
            </dl>
            <Button variant="outline" size="sm" className="mt-3 w-full text-destructive hover:text-destructive" onClick={() => setVoidOpen(true)}>
              <Ban data-icon="inline-start" /> Anular / comunicar baja
            </Button>
            <VoidDialog open={voidOpen} onOpenChange={setVoidOpen} number={number} type={label} />
          </SectionCard>
        </div>
      </div>
    </div>
  );
}

function Row({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div className="flex flex-col">
      <dt className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">{k}</dt>
      <dd className={cn(mono && "font-mono tabular-nums")}>{v}</dd>
    </div>
  );
}
