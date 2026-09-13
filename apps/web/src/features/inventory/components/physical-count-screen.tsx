"use client";

import { ClipboardCheck, Filter, Gavel } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { CountLine } from "../mocks/reception";

const warehouses = ["Sede San Isidro – almacén central", "Sede Miraflores – farmacia dental", "Surquillo – depósito auxiliar"];
const families = ["Materiales e insumos restauradores", "Anestésicos y quirúrgicos", "Prótesis e implantes", "EPP y bioseguridad"];

export function PhysicalCountScreen({ lines: initial }: { lines: CountLine[] }) {
  const [lines, setLines] = React.useState(initial);
  const diffUnits = lines.reduce((s, l) => s + (l.counted - l.system), 0);
  const diffValue = lines.reduce((s, l) => s + (l.counted - l.system) * l.unitCost, 0);
  const audited = lines.filter((l) => l.counted !== l.system).length;

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Inventario · Auditoría y fiscalización" title="Toma de inventario físico cíclico / general" description="Confrontación de la existencia real contra el kardex SUNAT (R.S. 234-2006)." status={<StatusBadge tone="info" dot label="Corte 12/09/2026" />} />
      <SectionCard title="Parámetros de la toma" contentClassName="grid gap-3 p-4 sm:grid-cols-4">
        <Field label="Almacén auditado"><Select defaultValue={warehouses[0]} items={warehouses.map((w) => ({ value: w, label: w }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{warehouses.map((w) => <SelectItem key={w} value={w}>{w}</SelectItem>)}</SelectContent></Select></Field>
        <Field label="Familia de artículos"><Select defaultValue={families[0]} items={families.map((w) => ({ value: w, label: w }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{families.map((w) => <SelectItem key={w} value={w}>{w}</SelectItem>)}</SelectContent></Select></Field>
        <Field label="Fecha de corte"><Input type="date" defaultValue="2026-09-12" /></Field>
        <div className="flex items-end"><Button variant="outline" className="w-full"><Filter data-icon="inline-start" /> Filtrar</Button></div>
      </SectionCard>
      <SectionCard title="Conteo" contentClassName="p-0">
        <div className="hidden overflow-x-auto md:block"><Table className="min-w-[820px]"><TableHeader><TableRow><TableHead className="pl-4">Código</TableHead><TableHead className="text-left">Descripción</TableHead><TableHead className="text-right">Stock kardex</TableHead><TableHead className="text-right">Conteo físico real</TableHead><TableHead className="text-right">Diferencia</TableHead><TableHead className="text-right">Costo unit. prom.</TableHead><TableHead className="text-right">Dif. valorizada</TableHead></TableRow></TableHeader><TableBody>
          {lines.map((l) => { const d = l.counted - l.system; return (
            <TableRow key={l.id}><TableCell className="px-4 py-2 font-mono text-xs">{l.sku}</TableCell><TableCell className="px-2 py-2"><p className="font-medium">{l.name}</p><p className="text-xs text-muted-foreground">{l.presentation}</p></TableCell><TableCell className="px-2 py-2 text-right font-mono">{l.system}</TableCell><TableCell className="px-2 py-2 text-right"><Input type="number" min={0} inputMode="numeric" value={l.counted} onChange={(e) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, counted: Math.max(0, Number(e.target.value) || 0) } : x)))} className="ml-auto h-8 w-20 text-right font-mono text-base" aria-label={`Conteo ${l.sku}`} /></TableCell><TableCell className={cn("px-2 py-2 text-right font-mono font-semibold", d < 0 && "text-destructive", d > 0 && "text-warning")}>{d > 0 ? `+${d}` : d}</TableCell><TableCell className="px-2 py-2 text-right font-mono">{formatCurrency(l.unitCost)}</TableCell><TableCell className={cn("px-4 py-2 text-right font-mono font-medium", d < 0 && "text-destructive", d > 0 && "text-warning")}>{d === 0 ? formatCurrency(0) : `${d < 0 ? "− " : "+ "}${formatCurrency(Math.abs(d * l.unitCost))}`}</TableCell></TableRow>
          ); })}
        </TableBody></Table></div>
        <ul className="divide-y md:hidden">
          {lines.map((l) => { const d = l.counted - l.system; return (
            <li key={l.id} className="flex items-center gap-3 px-4 py-3">
              <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{l.name}</p><p className="font-mono text-xs text-muted-foreground">{l.sku} · kardex {l.system}</p><p className={cn("text-xs font-medium", d < 0 && "text-destructive", d > 0 && "text-warning", d === 0 && "text-muted-foreground")}>{d === 0 ? "Sin diferencia" : `${d > 0 ? "+" : ""}${d} · ${d < 0 ? "− " : "+ "}${formatCurrency(Math.abs(d * l.unitCost))}`}</p></div>
              <Input type="number" min={0} inputMode="numeric" value={l.counted} onChange={(e) => setLines((ls) => ls.map((x) => (x.id === l.id ? { ...x, counted: Math.max(0, Number(e.target.value) || 0) } : x)))} className="h-12 w-20 text-center font-mono text-lg" aria-label={`Conteo ${l.sku}`} />
            </li>
          ); })}
        </ul>
        <div className="flex flex-wrap items-center justify-between gap-3 border-t p-4">
          <div><p className="text-xs text-muted-foreground">Diferencia neta físico vs. kardex</p><p className={cn("text-2xl font-bold tabular-nums", diffValue < 0 ? "text-destructive" : diffValue > 0 ? "text-warning" : "text-success")} data-testid="count-diff">{diffValue < 0 ? "− " : ""}{formatCurrency(Math.abs(diffValue))}</p><p className="text-xs text-muted-foreground">({diffUnits > 0 ? "+" : ""}{diffUnits} unidades · {audited} ítems con diferencia)</p></div>
          <div className="flex flex-col items-end gap-2"><p className="flex items-center gap-1 text-xs text-muted-foreground"><Gavel className="size-3.5" /> Sujeto al límite tributario de desmedros acreditados con informe técnico.</p><Button disabled={audited === 0} onClick={() => toast.success(`Ajuste MOV-2026-001292 aplicado (código 99 SUNAT) · ${formatCurrency(Math.abs(diffValue))}`)}><ClipboardCheck data-icon="inline-start" /> Aplicar ajuste de inventario (código 99 SUNAT)</Button></div>
        </div>
      </SectionCard>
    </div>
  );
}
