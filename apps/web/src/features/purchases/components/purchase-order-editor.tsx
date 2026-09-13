"use client";

import { AlertCircle, AlertTriangle, ArrowLeft, Building2, Calculator, CalendarCheck, CalendarDays, Check, CheckCircle2, Clock, Coins, FilePlus2, FileText, ListOrdered, Pencil, Plus, Save, Send, ShieldCheck, SlidersHorizontal, Trash2, Truck, UserRound, Warehouse } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EntityHeader } from "@/components/shared/entity-header";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { stockMock } from "@/features/inventory/mocks/stock";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Supplier } from "../mocks/suppliers";
import { FilterBar } from "@/components/shared/filter-bar";

interface OrderLine { id: string; sku: string; name: string; qty: number; unit: string; cost: number; discount: number; taxable: boolean }
const r2 = (n: number) => Math.round(n * 100) / 100;

const initialLines: OrderLine[] = [
  { id: "o1", sku: "MED-0441", name: "Anestesia lidocaína 2% c/epinefrina · caja 50", qty: 10, unit: "caja", cost: 78.5, discount: 0, taxable: true },
  { id: "o2", sku: "MAT-0084", name: "Resina fotocurable Filtek Z350 XT A2", qty: 10, unit: "jer", cost: 210, discount: 5, taxable: true },
  { id: "o3", sku: "INS-0102", name: "Guantes de nitrilo talla M · caja x100", qty: 20, unit: "caja", cost: 38, discount: 0, taxable: true },
  { id: "o4", sku: "MAT-0112", name: "Hilo retractor #00", qty: 3, unit: "un", cost: 65, discount: 0, taxable: true },
];
const suggestions = [
  { sku: "INS-0210", name: "Mascarillas KN95 x 20", reason: "Stock crítico · 4 / mín. 15", qty: 15, cost: 24 },
  { sku: "QUI-0099", name: "Kit de exodoncia", reason: "En mínimo · 2 / mín. 2", qty: 1, cost: 890 },
];

export function PurchaseOrderEditor({ suppliers }: { suppliers: Supplier[] }) {
  const [supplierId, setSupplierId] = React.useState(suppliers[0].id);
  const supplier = suppliers.find((s) => s.id === supplierId)!;
  const [lines, setLines] = React.useState(initialLines);
  const [editing, setEditing] = React.useState<string | null>(null);
  const [adding, setAdding] = React.useState(false);
  const [q, setQ] = React.useState("");
  const searchRef = React.useRef<HTMLInputElement>(null);
  const [detraction, setDetraction] = React.useState(false);
  const base = r2(lines.reduce((s, l) => s + l.qty * l.cost * (1 - l.discount / 100), 0));
  const igv = r2(lines.filter((l) => l.taxable).reduce((s, l) => s + l.qty * l.cost * (1 - l.discount / 100) * 0.18, 0));
  const total = r2(base + igv);
  const spot = detraction ? r2(total * 0.12) : 0;
  const results = q.trim() ? stockMock.filter((i) => i.name.toLowerCase().includes(q.toLowerCase()) || i.sku.toLowerCase().includes(q.toLowerCase())).slice(0, 5) : [];
  const update = (id: string, patch: Partial<OrderLine>) => setLines((ls) => ls.map((l) => (l.id === id ? { ...l, ...patch } : l)));
  const addSuggestion = (s: (typeof suggestions)[number]) => { if (lines.some((l) => l.sku === s.sku)) { toast.info("Ya está en la orden"); return; } setLines((ls) => [...ls, { id: s.sku, sku: s.sku, name: s.name, qty: s.qty, unit: "un", cost: s.cost, discount: 0, taxable: true }]); };

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/compras/ordenes" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Órdenes de compra</Button>
      <EntityHeader
        avatar={<span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent text-primary"><FilePlus2 className="size-5" /></span>}
        title="Nueva orden de compra OC-2026-0092"
        chips={<StatusBadge tone="warning" dot label="Borrador editable" />}
        meta={<p>Registro y valorización oficial de abastecimiento clínico odontológico</p>}
        actions={<>
          <span className="flex items-center gap-1.5 h-10 rounded-md border bg-muted/40 px-2.5 text-xs"><CalendarDays className="size-3.5 text-muted-foreground" /> Emisión: <strong className="font-semibold">12/09/2026</strong></span>
          <span className="flex items-center gap-1.5 h-10 rounded-md border bg-muted/40 px-2.5 text-xs"><CalendarCheck className="size-3.5 text-muted-foreground" /> Entrega est.: <strong className="font-semibold text-primary">19/09/2026</strong></span>
          <span className="flex items-center gap-1.5 h-10 rounded-md border bg-muted/40 px-2.5 text-xs"><Coins className="size-3.5 text-muted-foreground" /> Moneda: <strong className="font-semibold">Soles (PEN - S/)</strong></span>
          <Button variant="outline" onClick={() => toast.info("Borrador guardado")}><Save data-icon="inline-start" /> Guardar borrador</Button>
          <Button onClick={() => toast.success(`OC-2026-0092 enviada a ${supplier.email}`, { description: `${lines.length} ítems · ${formatCurrency(total)}` })}><Send data-icon="inline-start" /> Enviar orden al proveedor</Button>
        </>}
      />

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title={<span className="flex flex-wrap items-center gap-2">Proveedor homologado <StatusBadge tone={supplier.sunat === "habido" ? "success" : "danger"} label={supplier.sunat === "habido" ? "Habido y activo SUNAT" : "No habido"} className="rounded-md" /><Tag tone="info" label="Agente de percepción/retención" /></span>} icon={Building2} action={<Link href={`/compras/proveedores/${supplier.id}`} className="text-xs font-semibold text-primary hover:underline">Ver ficha completa →</Link>} contentClassName="grid gap-4 p-4 sm:grid-cols-3">
            <div><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Razón social y RUC</p><Select value={supplierId} onValueChange={(v) => setSupplierId(String(v))} items={suppliers.map((s) => ({ value: s.id, label: s.name }))}><SelectTrigger className="mt-1 w-full border-0 bg-transparent px-0 text-base font-bold shadow-none" aria-label="Proveedor"><SelectValue /></SelectTrigger><SelectContent>{suppliers.map((s) => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}</SelectContent></Select><p className="flex items-center gap-1 text-xs text-muted-foreground"><Building2 className="size-3.5" /> RUC: {supplier.ruc}</p></div>
            <div><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Contacto y cotización</p><p className="mt-1 flex items-center gap-1 text-sm font-semibold"><UserRound className="size-3.5 text-muted-foreground" /> {supplier.contact} (+51 984 123 456)</p><p className="flex items-center gap-1 font-mono text-xs text-muted-foreground"><FileText className="size-3.5" /> Ref: COT-DI-8841</p></div>
            <div><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Condición de pago</p><p className="mt-1"><StatusBadge tone="info" icon={Clock} label={supplier.terms} className="h-7 rounded-md text-sm" /></p><p className="mt-1 text-xs text-muted-foreground">Línea de crédito aprobada: S/ 50,000</p></div>
            <div className="grid gap-3 border-t pt-3 sm:col-span-3 sm:grid-cols-3">
              <Field label="Almacén destino"><Select defaultValue="w1" items={[{ value: "w1", label: "Almacén Central – Miraflores" }, { value: "w2", label: "Sede San Isidro" }, { value: "w3", label: "Surquillo logístico" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="w1">Almacén Central – Miraflores</SelectItem><SelectItem value="w2">Sede San Isidro</SelectItem><SelectItem value="w3">Surquillo logístico</SelectItem></SelectContent></Select></Field>
              <Field label="Entrega esperada"><Input type="date" defaultValue="2026-09-19" /></Field>
              <Field label="Comprador"><Input defaultValue="Lic. Patricia Vega" /></Field>
            </div>
          </SectionCard>

          <FilterBar>
            <SearchInput ref={searchRef} value={q} onChange={(v) => { setQ(v); setAdding(true); }} placeholder="Escribe SKU o nombre: Ej. Filtek, Lidocaína, Guantes, Sutura, Brackets… [Presiona F2]" aria-label="Buscar ítem" className="flex-1 sm:max-w-none" />
            <Button className="font-semibold" onClick={() => { if (results[0]) { const i = results[0]; setLines((ls) => [...ls, { id: i.id, sku: i.sku, name: i.name, qty: 1, unit: i.unit, cost: i.avgCost, discount: 0, taxable: true }]); setQ(""); } else toast.info("Escribe un SKU o nombre"); }}><Plus data-icon="inline-start" /> Insertar</Button>
            <Button variant="outline" className="font-semibold"><SlidersHorizontal data-icon="inline-start" /> Filtros</Button>
            {q.trim() && results.length ? <ul className="w-full rounded-md border bg-popover shadow-sm">{results.map((i) => <li key={i.id}><button type="button" onClick={() => { setLines((ls) => [...ls, { id: i.id, sku: i.sku, name: i.name, qty: 1, unit: i.unit, cost: i.avgCost, discount: 0, taxable: true }]); setAdding(false); setQ(""); }} className="flex w-full justify-between px-3 py-2 text-left text-sm hover:bg-accent"><span>{i.name}</span><span className="font-mono text-xs text-muted-foreground">{i.sku} · {formatCurrency(i.avgCost)}</span></button></li>)}</ul> : null}
          </FilterBar>

          <SectionCard title={<span className="flex items-center gap-2">Ítems requeridos en orden de compra <StatusBadge tone="info" label={`${lines.length} insumos`} className="rounded-md" /></span>} icon={ListOrdered} action={<span className="text-xs text-muted-foreground">Costos sin IGV</span>} contentClassName="p-0">
            <div className="hidden overflow-x-auto md:block">
              <Table className="min-w-[700px]">
                <TableHeader><TableRow><TableHead className="pl-4">SKU</TableHead><TableHead>Descripción</TableHead><TableHead className="text-right">Cant.</TableHead><TableHead>U.M.</TableHead><TableHead className="text-right">Costo unit.</TableHead><TableHead className="text-right">Desc.</TableHead><TableHead>IGV</TableHead><TableHead className="text-right">Total</TableHead><TableHead className="pr-4 text-right"><span className="sr-only">Acciones</span></TableHead></TableRow></TableHeader>
                <TableBody>
                  {lines.map((l) => { const ed = editing === l.id; return (
                    <TableRow key={l.id}>
                      <TableCell className="pl-4 font-mono text-xs">{l.sku}</TableCell>
                      <TableCell className="max-w-72 truncate font-medium">{l.name}</TableCell>
                      <TableCell className="text-right">{ed ? <Input value={l.qty} onChange={(e) => update(l.id, { qty: Number(e.target.value) || 0 })} className="ml-auto h-7 w-16 text-right font-mono" aria-label={`Cantidad ${l.sku}`} /> : <span className="font-mono">{l.qty}</span>}</TableCell>
                      <TableCell>{l.unit}</TableCell>
                      <TableCell className="text-right">{ed ? <Input value={l.cost} onChange={(e) => update(l.id, { cost: Number(e.target.value) || 0 })} className="ml-auto h-7 w-24 text-right font-mono" aria-label={`Costo ${l.sku}`} /> : <span className="font-mono">{l.cost.toFixed(2)}</span>}</TableCell>
                      <TableCell className="text-right">{ed ? <Input value={l.discount} onChange={(e) => update(l.id, { discount: Number(e.target.value) || 0 })} className="ml-auto h-7 w-14 text-right font-mono" aria-label={`Descuento ${l.sku}`} /> : <span className="font-mono">{l.discount}%</span>}</TableCell>
                      <TableCell><Badge variant="outline">{l.taxable ? "Gravado" : "Inafecto"}</Badge></TableCell>
                      <TableCell className="text-right font-mono font-medium">{formatCurrency(r2(l.qty * l.cost * (1 - l.discount / 100)))}</TableCell>
                      <TableCell className="pr-3 text-right"><div className="flex justify-end gap-0.5"><Button variant="ghost" size="icon-sm" aria-label={ed ? "Guardar" : "Editar"} onClick={() => setEditing(ed ? null : l.id)}>{ed ? <Check /> : <Pencil />}</Button><Button variant="ghost" size="icon-sm" aria-label={`Quitar ${l.sku}`} onClick={() => setLines((ls) => ls.filter((x) => x.id !== l.id))}><Trash2 /></Button></div></TableCell>
                    </TableRow>
                  ); })}
                </TableBody>
              </Table>
            </div>
            <ul className="divide-y md:hidden">{lines.map((l) => <li key={l.id} className="flex items-center gap-3 px-4 py-3"><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{l.name}</p><p className="font-mono text-xs text-muted-foreground">{l.sku} · {formatCurrency(l.cost)} × </p></div><Input value={l.qty} onChange={(e) => update(l.id, { qty: Number(e.target.value) || 0 })} className="h-9 w-16 text-center font-mono" aria-label={`Cantidad ${l.sku}`} /><span className="w-20 text-right font-mono text-sm font-medium">{formatCurrency(r2(l.qty * l.cost * (1 - l.discount / 100)))}</span></li>)}</ul>
            <div className="border-t p-3"><Button variant="outline" size="sm" onClick={() => { setAdding(true); searchRef.current?.focus(); }}><Plus data-icon="inline-start" /> Agregar ítem del catálogo</Button>{adding && !q ? <span className="ml-2 text-xs text-muted-foreground">Escribe en el buscador superior.</span> : null}</div>
          </SectionCard>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Almacén de destino" icon={Warehouse} action={<span className="text-xs text-muted-foreground">Recepción física</span>} contentClassName="flex flex-col gap-2 p-4 text-xs">
            <p className="font-semibold">Sede de ingreso</p>
            <Select defaultValue="w1" items={[{ value: "w1", label: "Almacén Central - Sede Miraflores (Av. Larco 1020)" }, { value: "w2", label: "Sede San Isidro" }]}><SelectTrigger className="w-full bg-muted/50" aria-label="Sede de ingreso"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="w1">Almacén Central - Sede Miraflores (Av. Larco 1020)</SelectItem><SelectItem value="w2">Sede San Isidro</SelectItem></SelectContent></Select>
            <p className="flex items-center justify-between gap-2 border-t pt-2 text-muted-foreground"><span className="flex items-center gap-1"><Truck className="size-3.5" /> Guía remisión proveedor:</span><em>Pendiente al despacho</em></p>
            <p className="flex items-center gap-1 text-muted-foreground"><ShieldCheck className="size-3.5" /> Inspección técnica por Regente Dental obligatoria</p>
          </SectionCard>
          <SectionCard title="Sugerencias por stock crítico" icon={AlertTriangle} iconTone="danger" action={<StatusBadge tone="warning" label="Kárdex sede" className="rounded-md" />} contentClassName="flex flex-col gap-2 p-3">
            {suggestions.map((s) => { const added = lines.some((l) => l.sku === s.sku); const crit = /agotad|crític/i.test(s.reason); return (
              <div key={s.sku} className={cn("flex items-center gap-3 rounded-lg border p-3 text-xs", added ? "border-emerald-200 bg-emerald-50/60 dark:border-emerald-900 dark:bg-emerald-950/30" : crit ? "border-rose-200 bg-rose-50/60 dark:border-rose-900 dark:bg-rose-950/30" : "border-amber-200 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/30")}>
                <div className="min-w-0 flex-1"><p className="flex items-center gap-1.5 text-sm font-semibold">{added ? <CheckCircle2 className="size-4 text-emerald-600" /> : crit ? <AlertCircle className="size-4 text-rose-600" /> : <AlertTriangle className="size-4 text-amber-600" />}{s.name}</p><p className="text-muted-foreground">{s.reason} · Sugerido: <strong className="text-foreground">{s.qty}</strong></p></div>
                {added ? <StatusBadge tone="success" label="Ya agregado ✓" className="rounded-md" /> : <Button size="xs" variant="outline" className="bg-card font-semibold" onClick={() => addSuggestion(s)}>+ Agregar</Button>}
              </div>
            ); })}
          </SectionCard>
          <SectionCard title="Liquidación tributaria SUNAT" icon={Calculator} action={<span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">PEN S/</span>} contentClassName="p-4 text-sm">
            <dl className="flex flex-col gap-1">
              <div className="flex justify-between"><dt className="text-muted-foreground">Subtotal / valor compra</dt><dd className="font-mono tabular-nums">{formatCurrency(base)}</dd></div>
              <div className="flex justify-between"><dt className="text-muted-foreground">IGV oficial (18.00%)</dt><dd className="font-mono tabular-nums">{formatCurrency(igv)}</dd></div>
              <Separator className="my-1" />
              <div className="flex items-baseline justify-between rounded-lg bg-accent/60 px-3 py-2 dark:bg-muted"><dt className="font-bold">Total orden</dt><dd className="font-mono text-xl font-bold text-primary tabular-nums" data-testid="po-total">{formatCurrency(total)}</dd></div>
              <label className="mt-2 flex items-center justify-between rounded-md border p-2 text-xs"><span>Sujeta a detracción (12%)</span><input type="checkbox" checked={detraction} onChange={(e) => setDetraction(e.target.checked)} aria-label="Aplicar detracción" /></label>
              {detraction ? <div className="flex justify-between text-xs"><dt className="text-muted-foreground">Depósito SPOT (BN {supplier.name.includes("Biom") ? "00-068-124859" : "—"})</dt><dd className="font-mono">{formatCurrency(spot)}</dd></div> : null}
            </dl>
          </SectionCard>
        </div>
      </div>
    </div>
  );
}
