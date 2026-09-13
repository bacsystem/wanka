"use client";

import { ArrowLeft, ArrowRight, Check, Copy, ExternalLink, Mail, MessageCircle, Plus, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatusBadge } from "@/components/shared/status-badge";
import { customersMock } from "@/features/customers/mocks/customers";
import { catalogMock } from "@/features/pos/mocks/catalog";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";

const templates = [
  { id: "eps", name: "Paquete corporativo EPS", desc: "Profilaxis + evaluación para colaboradores", items: ["c2", "c9"] },
  { id: "orto", name: "Ortodoncia en cuotas", desc: "Brackets + controles mensuales", items: ["c7", "c9"] },
  { id: "blanq", name: "Blanqueamiento promo", desc: "2 sesiones LED + profilaxis", items: ["c5", "c2"] },
  { id: "blank", name: "En blanco", desc: "Empieza desde cero", items: [] },
];
interface Line { id: string; sku: string; name: string; qty: number; unitValue: number; discount: number }
const r2 = (n: number) => Math.round(n * 100) / 100;

export function NewQuoteWizard() {
  const router = useRouter();
  const [step, setStep] = React.useState(0);
  const [tpl, setTpl] = React.useState<string | null>(null);
  const [customerId, setCustomerId] = React.useState<string | null>(null);
  const [q, setQ] = React.useState("");
  const [lines, setLines] = React.useState<Line[]>([]);
  const [itemQ, setItemQ] = React.useState("");
  const [globalDisc, setGlobalDisc] = React.useState("0");
  const [detraction, setDetraction] = React.useState(false);
  const customer = customersMock.find((c) => c.id === customerId) ?? null;
  const results = q.trim() ? customersMock.filter((c) => c.name.toLowerCase().includes(q.toLowerCase()) || c.documentNumber.includes(q)).slice(0, 4) : [];
  const itemResults = itemQ.trim() ? catalogMock.filter((c) => c.name.toLowerCase().includes(itemQ.toLowerCase()) || c.sku.toLowerCase().includes(itemQ.toLowerCase())).slice(0, 5) : [];
  const gross = lines.reduce((s, l) => s + l.qty * l.unitValue * (1 - l.discount / 100), 0);
  const base = r2(gross - (Number(globalDisc) || 0));
  const igv = r2(base * 0.18); const total = r2(base + igv);
  const applyTpl = (id: string) => { setTpl(id); const t = templates.find((x) => x.id === id)!; setLines(t.items.map((i) => { const c = catalogMock.find((x) => x.id === i)!; return { id: c.id, sku: c.sku, name: c.name, qty: 1, unitValue: r2(c.price / 1.18), discount: 0 }; })); };
  const add = (c: (typeof catalogMock)[number]) => { setLines((ls) => ls.some((l) => l.id === c.id) ? ls : [...ls, { id: c.id, sku: c.sku, name: c.name, qty: 1, unitValue: r2(c.price / 1.18), discount: 0 }]); setItemQ(""); };
  function next() {
    if (step === 0 && !customer) { toast.error("Selecciona un cliente"); return; }
    if (step === 1 && lines.length === 0) { toast.error("Agrega al menos un ítem"); return; }
    if (step < 2) setStep(step + 1); else { toast.success("Cotización COT-2026-00129 creada y enviada", { description: `${customer?.name} · ${formatCurrency(total)} · enlace de aprobación generado` }); router.push("/ventas/proformas"); }
  }
  const steps = ["Cliente y condiciones", "Ítems", "Revisión y envío"];
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/ventas/proformas" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Cotizaciones</Button>
      <PageHeader eyebrow="Ventas · Cotizaciones" title="Nueva cotización COT-2026-00129" status={<StatusBadge tone="neutral" label="Borrador" />} description="Asistente de 3 pasos con plantillas, condiciones comerciales y envío con enlace de aprobación en línea." />
      <ol className="grid gap-2 sm:grid-cols-3">{steps.map((s, i) => <li key={s} className={cn("flex items-center gap-2 rounded-md border px-3 py-2 text-sm", i === step && "border-primary bg-accent text-accent-foreground", i < step && "border-success/40 text-success")}><span className={cn("flex size-6 items-center justify-center rounded-full border text-[11px] font-semibold", i === step && "border-primary bg-primary text-primary-foreground", i < step && "border-success bg-success text-success-foreground")}>{i < step ? <Check className="size-3" /> : i + 1}</span>{s}</li>)}</ol>

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          {step === 0 ? (
            <>
              <SectionCard title="Plantilla de inicio" contentClassName="grid gap-2 p-4 sm:grid-cols-2">{templates.map((t) => <button key={t.id} type="button" onClick={() => applyTpl(t.id)} aria-pressed={tpl === t.id} className={cn("rounded-md border p-3 text-left text-sm hover:border-primary/50", tpl === t.id && "border-primary bg-accent/40")}><p className="font-medium">{t.name}</p><p className="text-xs text-muted-foreground">{t.desc}</p></button>)}</SectionCard>
              <SectionCard title="Cliente" contentClassName="grid gap-3 p-4">
                {customer ? <div className="flex items-center justify-between rounded-md border p-3 text-sm"><div><p className="font-medium">{customer.name}</p><p className="font-mono text-xs text-muted-foreground">{customer.documentType} {customer.documentNumber}{customer.email ? ` · ${customer.email}` : ""}</p></div><Button size="sm" variant="ghost" onClick={() => setCustomerId(null)}>Cambiar</Button></div> : <div><SearchInput value={q} onChange={setQ} placeholder="Buscar por RUC, DNI o razón social…" aria-label="Buscar cliente" className="sm:max-w-none" />{results.length ? <ul className="mt-1 w-full rounded-md border bg-popover shadow-sm">{results.map((c) => <li key={c.id}><button type="button" onClick={() => { setCustomerId(c.id); setQ(""); }} className="flex w-full justify-between px-3 py-2 text-left text-sm hover:bg-accent"><span>{c.name}</span><span className="font-mono text-xs text-muted-foreground">{c.documentType} {c.documentNumber}</span></button></li>)}</ul> : null}<Button variant="link" size="sm" className="px-0"><Plus data-icon="inline-start" /> Crear cliente rápido</Button></div>}
                <div className="grid gap-3 sm:grid-cols-3">
                  <Field label="Validez"><Select defaultValue="15" items={[{ value: "7", label: "7 días" }, { value: "15", label: "15 días" }, { value: "30", label: "30 días" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="7">7 días</SelectItem><SelectItem value="15">15 días</SelectItem><SelectItem value="30">30 días</SelectItem></SelectContent></Select></Field>
                  <Field label="Forma de pago"><Select defaultValue="contado" items={[{ value: "contado", label: "Contado" }, { value: "30", label: "Crédito 30 días" }, { value: "cuotas", label: "Cuotas mensuales" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="contado">Contado</SelectItem><SelectItem value="30">Crédito 30 días</SelectItem><SelectItem value="cuotas">Cuotas mensuales</SelectItem></SelectContent></Select></Field>
                  <Field label="Lista de precios"><Select defaultValue="particular" items={[{ value: "particular", label: "Particular" }, { value: "rimac", label: "Rimac EPS (−15%)" }, { value: "corp", label: "Convenio corporativo" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="particular">Particular</SelectItem><SelectItem value="rimac">Rimac EPS (−15%)</SelectItem><SelectItem value="corp">Convenio corporativo</SelectItem></SelectContent></Select></Field>
                  <Field label="Descuento global (S/)"><Input value={globalDisc} onChange={(e) => setGlobalDisc(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
                  <div className="flex items-center justify-between rounded-md border p-3 text-sm sm:col-span-2"><span>Sujeta a detracción SPOT (12%)</span><Switch checked={detraction} onCheckedChange={setDetraction} aria-label="Detracción" /></div>
                </div>
              </SectionCard>
            </>
          ) : step === 1 ? (
            <SectionCard title={`Ítems · ${lines.length}`} contentClassName="p-0">
              <ul className="divide-y">{lines.map((l) => <li key={l.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5 text-sm"><div className="min-w-0 flex-1"><p className="font-medium">{l.name}</p><p className="font-mono text-xs text-muted-foreground">{l.sku} · V.U. {formatCurrency(l.unitValue)}</p></div><label className="flex items-center gap-1 text-xs text-muted-foreground">Cant.<Input value={l.qty} onChange={(e) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, qty: Number(e.target.value) || 0 } : x)))} className="h-7 w-16 text-right font-mono" aria-label={`Cantidad ${l.sku}`} /></label><label className="flex items-center gap-1 text-xs text-muted-foreground">Desc. %<Input value={l.discount} onChange={(e) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, discount: Number(e.target.value) || 0 } : x)))} className="h-7 w-14 text-right font-mono" aria-label={`Descuento ${l.sku}`} /></label><span className="w-24 text-right font-mono font-medium">{formatCurrency(r2(l.qty * l.unitValue * (1 - l.discount / 100) * 1.18))}</span><Button size="icon-xs" variant="ghost" aria-label={`Quitar ${l.sku}`} onClick={() => setLines((ls) => ls.filter((x) => x.id !== l.id))}><Trash2 /></Button></li>)}{lines.length === 0 ? <li className="px-4 py-8 text-center text-sm text-muted-foreground">Agrega ítems del catálogo.</li> : null}</ul>
              <div className="border-t p-3"><SearchInput value={itemQ} onChange={setItemQ} placeholder="Buscar en el catálogo (F2)…" aria-label="Buscar ítem" className="sm:max-w-none" />{itemResults.length ? <ul className="mt-2 w-full rounded-md border bg-popover shadow-sm">{itemResults.map((c) => <li key={c.id}><button type="button" onClick={() => add(c)} className="flex w-full justify-between px-3 py-2 text-left text-sm hover:bg-accent"><span>{c.name}</span><span className="font-mono text-xs text-muted-foreground">{c.sku} · {formatCurrency(c.price)}</span></button></li>)}</ul> : null}</div>
            </SectionCard>
          ) : (
            <SectionCard title="Revisión y envío" contentClassName="flex flex-col gap-4 p-4 text-sm">
              <dl className="grid gap-2 sm:grid-cols-2"><div><dt className="text-xs text-muted-foreground">Cliente</dt><dd className="font-medium">{customer?.name}</dd></div><div><dt className="text-xs text-muted-foreground">Ítems</dt><dd className="font-medium">{lines.length} · total {formatCurrency(total)}</dd></div></dl>
              <div className="rounded-md border p-3"><p className="flex items-center gap-2 font-medium"><ExternalLink className="size-4" /> Enlace de aprobación en línea</p><p className="mt-1 font-mono text-xs text-muted-foreground">https://app.perusaas.pe/c/COT-2026-00129?t=8f3a…</p><div className="mt-3 rounded-md border bg-muted/30 p-3 text-xs"><p className="font-medium">Vista previa (lo que ve el cliente)</p><p className="text-muted-foreground">Clínica Dental Sonrisa S.A.C. le envía la cotización COT-2026-00129 por {formatCurrency(total)}. Válida 15 días.</p><Button size="xs" className="mt-2" disabled><Check data-icon="inline-start" /> Aprobar cotización</Button></div><Button size="xs" variant="ghost" className="mt-2" onClick={() => toast.success("Enlace copiado")}><Copy data-icon="inline-start" /> Copiar enlace</Button></div>
              <div className="flex flex-wrap gap-2"><Button variant="outline" onClick={() => toast.success(`Enviada por correo a ${customer?.email ?? "cliente"}`)}><Mail data-icon="inline-start" /> Enviar por correo</Button><Button variant="outline" onClick={() => toast.success("Enviada por WhatsApp")}><MessageCircle data-icon="inline-start" /> Enviar por WhatsApp</Button></div>
            </SectionCard>
          )}
          <div className="flex items-center justify-between rounded-lg border bg-card p-3"><Button variant="ghost" onClick={() => (step === 0 ? router.push("/ventas/proformas") : setStep(step - 1))}>{step === 0 ? "Cancelar" : <><ArrowLeft data-icon="inline-start" /> Anterior</>}</Button><Button onClick={next}>{step < 2 ? <>Siguiente <ArrowRight data-icon="inline-end" /></> : "Crear y enviar cotización"}</Button></div>
        </div>
        <SectionCard title="Totales en vivo" contentClassName="p-4 text-sm">
          <dl className="flex flex-col gap-1"><div className="flex justify-between"><dt className="text-muted-foreground">Op. gravada</dt><dd className="font-mono">{formatCurrency(gross)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Descuento global</dt><dd className="font-mono text-destructive">− {formatCurrency(Number(globalDisc) || 0)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">IGV (18%)</dt><dd className="font-mono">{formatCurrency(igv)}</dd></div><div className="flex items-baseline justify-between border-t pt-2"><dt className="font-semibold">Total</dt><dd className="font-mono text-xl font-semibold" data-testid="quote-total">{formatCurrency(total)}</dd></div>{detraction ? <div className="flex justify-between text-xs"><dt className="text-muted-foreground">Detracción 12%</dt><dd className="font-mono">{formatCurrency(r2(total * 0.12))}</dd></div> : null}</dl>
          {tpl ? <Badge variant="outline" className="mt-3">Plantilla: {templates.find((t) => t.id === tpl)?.name}</Badge> : null}
        </SectionCard>
      </div>
    </div>
  );
}
