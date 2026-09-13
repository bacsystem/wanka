"use client";

import { ArrowLeft, ArrowRight, Check, FileCode, FileDown, Save, Send, ShieldCheck, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate, formatDocumentNumber, parseNumber } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { SalesDocumentDetail } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";
import { SunatStatusBadge } from "./sunat-status-badge";

export const creditNoteTypes = [
  { code: "01", label: "Anulación de la operación", text: "Anula completamente el comprobante y extingue sus efectos tributarios y contables.", full: true },
  { code: "02", label: "Anulación por error en el RUC", text: "Corrige el documento de identidad emitiendo luego una nueva factura correlativa.", full: true },
  { code: "04", label: "Descuento global", text: "Aplica una rebaja sobre el importe global sin modificar cantidades físicas." },
  { code: "06", label: "Devolución total", text: "Restitución total de insumos y servicios con reingreso automático al kardex.", full: true },
  { code: "07", label: "Devolución por ítem", text: "Individualiza qué procedimiento o insumo se anula del comprobante original." },
  { code: "09", label: "Disminución en el valor", text: "Ajuste de precio por acuerdo médico-paciente o corrección de arancel." },
];

const steps = ["Motivo y sustento SUNAT", "Selección de ítems y montos", "Firma digital y envío OSE"];

export function CreditNoteWizard({ doc }: { doc: SalesDocumentDetail }) {
  const router = useRouter();
  const [step, setStep] = React.useState(0);
  const [type, setType] = React.useState("01");
  const [reason, setReason] = React.useState("Paciente solicita resolución del contrato odontológico por viaje al exterior. No se llegó a realizar la fase quirúrgica programada según orden de atención #OA-2026-912.");
  const [qty, setQty] = React.useState<Record<string, number>>(Object.fromEntries(doc.lines.map((l) => [l.code, l.quantity])));
  const [restock, setRestock] = React.useState(true);
  const [sendMail, setSendMail] = React.useState(true);
  const number = formatDocumentNumber(doc.series, doc.number);
  const ncNumber = "FC01-00000105";
  const meta = creditNoteTypes.find((t) => t.code === type)!;
  const full = Boolean(meta.full);
  const affected = doc.lines.map((l) => ({ ...l, sel: full ? l.quantity : (qty[l.code] ?? 0) }));
  const total = affected.reduce((s, l) => s + (l.total / l.quantity) * l.sel, 0);
  const base = total / 1.18;

  function next() {
    if (step === 0 && reason.trim().length < 20) { toast.error("El sustento debe tener al menos 20 caracteres"); return; }
    if (step === 1 && total <= 0) { toast.error("Selecciona al menos un ítem o cantidad a anular"); return; }
    if (step < 2) setStep(step + 1);
    else { toast.success(`Nota de crédito ${ncNumber} emitida y transmitida a SUNAT`, { description: `${formatCurrency(total)} · ${meta.code} ${meta.label}` }); router.push(`/ventas/comprobantes/${doc.id}`); }
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href={`/ventas/comprobantes/${doc.id}`} />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Volver al comprobante</Button>
      <PageHeader eyebrow={`Ventas · Comprobantes · ${number}`} title="Emitir nota de crédito electrónica" description="Catálogo 09 SUNAT · R.S. 097-2012 · el sustento se consigna en cac:DiscrepancyResponse/cbc:Description del XML UBL 2.1." status={<StatusBadge tone="info" label={`Afecta a ${number}`} />} />

      <ol className="grid gap-2 sm:grid-cols-3">
        {steps.map((s, i) => (<li key={s} className={cn("flex items-center gap-2 rounded-md border px-3 py-2 text-sm", i === step && "border-primary bg-accent text-accent-foreground", i < step && "border-success/40 text-success")}><span className={cn("flex size-6 shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold", i === step && "border-primary bg-primary text-primary-foreground", i < step && "border-success bg-success text-success-foreground")}>{i < step ? <Check className="size-3" /> : i + 1}</span><span className="truncate">Paso {i + 1} · {s}</span></li>))}
      </ol>

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          {step === 0 ? (
            <>
              <SectionCard title="1. Configuración de serie y tipo de documento" contentClassName="grid gap-4 p-4 sm:grid-cols-3">
                <Field label="Serie de nota de crédito"><Select defaultValue="FC01" items={[{ value: "FC01", label: "FC01 – Electrónica factura" }, { value: "BC01", label: "BC01 – Electrónica boleta" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="FC01">FC01 – Electrónica factura</SelectItem><SelectItem value="BC01">BC01 – Electrónica boleta</SelectItem></SelectContent></Select></Field>
                <Field label="Correlativo estimado"><Input readOnly value={ncNumber} className="font-mono" /></Field>
                <Field label="Fecha de transmisión" help="Dentro del plazo legal SUNAT"><Input type="date" defaultValue="2026-09-12" /></Field>
              </SectionCard>
              <SectionCard title="Tipo de nota de crédito (catálogo 09 SUNAT)" contentClassName="p-4">
                <ul className="grid gap-2 sm:grid-cols-2">{creditNoteTypes.map((t) => (<li key={t.code}><button type="button" onClick={() => setType(t.code)} aria-pressed={type === t.code} className={cn("flex h-full w-full flex-col gap-1 rounded-md border p-3 text-left hover:border-primary/50", type === t.code && "border-primary bg-accent/40 ring-2 ring-primary/20")}><span className="flex items-center gap-2 text-sm font-medium"><Badge variant="outline" className="font-mono">{t.code}</Badge>{t.label}</span><span className="text-xs text-muted-foreground">{t.text}</span></button></li>))}</ul>
                <Field label="Sustento o motivo de emisión (exigido en el XML SUNAT)" className="mt-4" help={`${reason.length} / 500 caracteres`}><Textarea rows={3} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} /></Field>
              </SectionCard>
            </>
          ) : step === 1 ? (
            <SectionCard title={`2. Ítems afectados de ${number}`} action={full ? <StatusBadge tone="info" label="Anulación total · todos los ítems" /> : <Button size="xs" variant="outline" onClick={() => setQty(Object.fromEntries(doc.lines.map((l) => [l.code, l.quantity])))}>Seleccionar todos</Button>} contentClassName="p-0">
              <div className="overflow-x-auto"><Table className="min-w-[720px]"><TableHeader><TableRow><TableHead className="pl-4">Código / descripción</TableHead><TableHead className="text-left">Unidad</TableHead><TableHead className="text-right">Cant. orig.</TableHead><TableHead className="text-right">Cant. anul.</TableHead><TableHead className="text-right">Valor unit.</TableHead><TableHead className="text-right">IGV</TableHead><TableHead className="text-right">Total (S/)</TableHead></TableRow></TableHeader><TableBody>
                {affected.map((l) => (<TableRow key={l.code}><TableCell className="px-4 py-2"><p className="font-medium">{l.description}</p><p className="font-mono text-xs text-muted-foreground">{l.code}</p></TableCell><TableCell className="px-2 py-2">{l.unit}</TableCell><TableCell className="px-2 py-2 text-right font-mono">{l.quantity}</TableCell><TableCell className="px-2 py-2 text-right">{full ? <span className="font-mono">{l.quantity}</span> : <Input size="sm" type="number" min={0} max={l.quantity} value={l.sel} onChange={(e) => setQty({ ...qty, [l.code]: Math.min(l.quantity, parseNumber(e.target.value)) })} className="ml-auto w-16 text-right font-mono" aria-label={`Cantidad a anular ${l.code}`} />}</TableCell><TableCell className="px-2 py-2 text-right font-mono">{formatCurrency(l.unitValue)}</TableCell><TableCell className="px-2 py-2 text-right font-mono">{formatCurrency((l.total - l.total / 1.18) / l.quantity * l.sel)}</TableCell><TableCell className="px-4 py-2 text-right font-mono font-medium">{formatCurrency((l.total / l.quantity) * l.sel)}</TableCell></TableRow>))}
              </TableBody></Table></div>
              <div className="flex flex-col gap-2 border-t p-4 text-sm">
                <label className="flex items-center gap-2"><Checkbox checked={restock} onCheckedChange={(v) => setRestock(Boolean(v))} /> Reingresar existencias al kardex de Almacén Miraflores</label>
                <label className="flex items-center gap-2"><Checkbox checked={sendMail} onCheckedChange={(v) => setSendMail(Boolean(v))} /> Enviar PDF y XML al correo del cliente ({doc.customerEmail ?? "cliente@correo.pe"})</label>
              </div>
            </SectionCard>
          ) : (
            <SectionCard title="3. Firma digital y envío al OSE" contentClassName="flex flex-col gap-3 p-4 text-sm">
              <dl className="grid gap-2 sm:grid-cols-2">{[["Nota de crédito", ncNumber], ["Tipo", `${meta.code} – ${meta.label}`], ["Comprobante afectado", `${documentTypeLabel[doc.type]} ${number}`], ["Cliente", doc.customerName], ["Ítems afectados", `${affected.filter((l) => l.sel > 0).length} de ${doc.lines.length}`], ["Certificado", "Vigente hasta 14/05/2027 · SHA-256"]].map(([k, v]) => <div key={k}><dt className="text-xs text-muted-foreground">{k}</dt><dd className="font-medium">{v}</dd></div>)}</dl>
              <div className="rounded-md bg-muted/40 p-3 text-xs"><p className="font-medium">Sustento</p><p className="text-muted-foreground">{reason}</p></div>
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground"><ShieldCheck className="size-3.5 text-success" /> El XML se firmará con el certificado de la empresa y se transmitirá al OSE Digiflow (99.98% operativo).</p>
            </SectionCard>
          )}
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-card p-3">
            <Button variant="ghost" onClick={() => (step === 0 ? router.push(`/ventas/comprobantes/${doc.id}`) : setStep(step - 1))}>{step === 0 ? "Cancelar operación" : <><ArrowLeft data-icon="inline-start" /> Anterior</>}</Button>
            <div className="flex gap-2"><Button variant="outline" onClick={() => toast.info("Borrador guardado")}><Save data-icon="inline-start" /> Guardar borrador</Button><Button onClick={next}>{step < 2 ? <>Siguiente <ArrowRight data-icon="inline-end" /></> : <><Send data-icon="inline-start" /> Emitir y transmitir a SUNAT ({ncNumber})</>}</Button></div>
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Comprobante afectado" action={<SunatStatusBadge status={doc.sunatStatus} />} contentClassName="flex flex-col gap-2 p-4 text-sm">
            <p className="font-mono text-lg font-semibold">{number}</p>
            <dl className="grid gap-1.5 text-xs">{[["Tipo", `${documentTypeLabel[doc.type]} electrónica`], ["Emisión", `${formatDate(doc.issuedAt)} ${doc.issuedAt.slice(11, 16)}`], ["Cliente", doc.customerName], ["Documento", doc.customerDocument], ["Condición de pago", doc.paymentTerms], ["Moneda", "Soles (PEN – S/)"], ["Hash resumen", doc.cdr?.hash ? `${doc.cdr.hash.slice(0, 14)}…` : "—"]].map(([k, v]) => <div key={k} className="flex justify-between gap-2"><dt className="text-muted-foreground">{k}</dt><dd className="text-right font-medium">{v}</dd></div>)}</dl>
            <dl className="mt-2 border-t pt-2 text-xs"><div className="flex justify-between"><dt className="text-muted-foreground">Monto original</dt><dd className="font-mono">{formatCurrency(doc.total)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Notas de crédito previas</dt><dd className="font-mono">{formatCurrency(0)}</dd></div><div className="flex justify-between font-medium"><dt>Saldo neto disponible</dt><dd className="font-mono">{formatCurrency(doc.total)}</dd></div></dl>
            <div className="mt-1 flex gap-2"><Button variant="outline" size="sm" className="flex-1"><FileDown data-icon="inline-start" /> PDF</Button><Button variant="outline" size="sm" className="flex-1"><FileCode data-icon="inline-start" /> XML y CDR</Button></div>
          </SectionCard>
          <SectionCard title="Resumen de la nota" contentClassName="p-4 text-sm">
            <dl className="flex flex-col gap-1"><div className="flex justify-between"><dt className="text-muted-foreground">Op. gravada afectada</dt><dd className="font-mono tabular-nums">{formatCurrency(base)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">IGV (18%)</dt><dd className="font-mono tabular-nums">{formatCurrency(total - base)}</dd></div><div className="mt-1 flex items-baseline justify-between border-t pt-2"><dt className="font-semibold">Total nota de crédito</dt><dd className="font-mono text-xl font-semibold tabular-nums" data-testid="nc-total">{formatCurrency(total)}</dd></div></dl>
            <p className="mt-2 text-xs text-muted-foreground">Capacidad de anulación: {Math.round((total / doc.total) * 100)}% · {total >= doc.total ? `${number} quedará totalmente saldada con estado Anulada.` : "El comprobante mantendrá un saldo vigente."}</p>
          </SectionCard>
          <SectionCard title="Marco legal SUNAT vigente" contentClassName="p-4 text-xs text-muted-foreground">Plazo legal de emisión: hasta el decimoquinto (15.º) día hábil del mes siguiente a la emisión del comprobante original (R.S. 000193-2020/SUNAT).</SectionCard>
        </div>
      </div>
    </div>
  );
}

export function VoidDialog({ open, onOpenChange, number, type }: { open: boolean; onOpenChange: (o: boolean) => void; number: string; type: string }) {
  const [motive, setMotive] = React.useState("");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader><DialogTitle>Comunicación de baja de comprobante</DialogTitle><DialogDescription>Resumen diario de bajas SUNAT · herramienta legal para anular {type.toLowerCase()}s dentro de los 7 días calendario de su emisión.</DialogDescription></DialogHeader>
        <div className="grid gap-4">
          <div className="flex items-center justify-between rounded-md border p-3 text-sm"><span className="text-muted-foreground">Comprobante</span><span className="font-mono font-medium">{number}</span></div>
          <Field label="Motivo de la baja" help="Se consigna en el resumen RA-20260912-001."><Textarea rows={3} value={motive} onChange={(e) => setMotive(e.target.value)} placeholder="Error en la digitación del importe / cliente…" /></Field>
          <div className="flex items-start gap-2 rounded-md border border-warning/40 bg-warning/10 p-3 text-xs"><Trash2 className="mt-0.5 size-4 shrink-0 text-warning" /><p>La baja es irreversible: SUNAT marcará el comprobante como anulado y no podrá volver a usarse el correlativo. Para facturas fuera de plazo emite una nota de crédito tipo 01.</p></div>
        </div>
        <DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button><Button variant="destructive" disabled={motive.trim().length < 10} onClick={() => { toast.success(`Resumen de bajas enviado · ${number} en proceso de anulación`); onOpenChange(false); }}>Enviar resumen de bajas</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
