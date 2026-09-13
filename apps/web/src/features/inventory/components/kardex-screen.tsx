"use client";

import { ArrowDownToLine, ArrowUpFromLine, CheckCircle2, Download, FileSpreadsheet, FileText, Flag, ShieldCheck, Warehouse } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard } from "@/components/shared/stat-card";
import { InventoryTabs } from "./inventory-tabs";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { kardexItem as KardexItemType, KardexRow } from "../mocks/kardex";

const n4 = (v: number) => v.toLocaleString("en-US", { minimumFractionDigits: 4, maximumFractionDigits: 4 });
const n2 = (v: number) => v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const warehouses = [{ value: "0000", label: "0000 – Miraflores (principal)" }, { value: "0001", label: "0001 – San Isidro (anexo)" }, { value: "0002", label: "0002 – Surquillo logístico" }];
const periods = [{ value: "2026-09", label: "Setiembre 2026 (abierto)" }, { value: "2026-08", label: "Agosto 2026 (declarado)" }, { value: "2026-07", label: "Julio 2026 (declarado)" }];

export function KardexScreen({ item, rows }: { item: typeof KardexItemType; rows: KardexRow[] }) {
  const [filter, setFilter] = React.useState("all");
  const [mode, setMode] = React.useState("13.1");
  const visible = rows.filter((r) => filter === "all" || (filter === "in" ? r.kind === "entrada" : filter === "out" ? r.kind === "salida" && !r.opType.startsWith("11") : r.opType.startsWith("11")));
  const totals = rows.reduce((a, r) => ({ in: a.in + (r.kind === "entrada" ? r.total : 0), out: a.out + (r.kind === "salida" ? r.total : 0), inQty: a.inQty + (r.kind === "entrada" ? r.qty : 0), outQty: a.outQty + (r.kind === "salida" ? -r.qty : 0) }), { in: 0, out: 0, inQty: 0, outQty: 0 });
  const count = (k: string) => rows.filter((r) => k === "in" ? r.kind === "entrada" : k === "out" ? r.kind === "salida" && !r.opType.startsWith("11") : k === "loss" ? r.opType.startsWith("11") : true).length;

  return (
    <>
      <InventoryTabs active="/inventario/kardex" />
      <SectionCard title="Artículo y parámetros de valuación" contentClassName="grid gap-4 p-4 lg:grid-cols-[1fr_auto_auto]">
        <Field label="Artículo odontológico / existencia sujeta a valuación">
          <div className="flex flex-col gap-1 rounded-md border px-3 py-2">
            <span className="text-sm font-semibold">{item.name}</span>
            <span className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground"><span>Cód. <span className="font-mono">{item.code}</span></span><span>UNSPSC <span className="font-mono">{item.unspsc}</span></span><span>{item.sunatType}</span><span>Tabla 6: {item.unit}</span></span>
          </div>
        </Field>
        <Field label="Método de valuación (art. 62 LIR)">
          <div className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"><ShieldCheck className="size-4 text-success" /> {item.method} <Button variant="link" size="xs" className="px-1" onClick={() => toast.info("Cambio de método requiere comunicación a SUNAT")}>Cambiar</Button></div>
        </Field>
        <Field label="Rango operativo">
          <div className="flex gap-2"><Input type="date" defaultValue="2026-09-01" className="w-36" /><span className="self-center text-xs text-muted-foreground">al</span><Input type="date" defaultValue="2026-09-30" className="w-36" /></div>
        </Field>
      </SectionCard>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Saldo inicial del período" icon={Flag} value={String(item.opening.qty)} suffix="NIU" footer={<span className="flex w-full justify-between"><span>Costo unit. S/ {n4(item.opening.unitCost)}</span><strong className="font-mono text-foreground">{formatCurrency(item.opening.value)}</strong></span>} />
        <StatCard label="Entradas acumuladas" icon={ArrowDownToLine} tone="success" emphasizeValue value={`+${item.entries.qty}`} suffix={`NIU (${item.entries.docs} facturas)`} footer={<span className="flex w-full justify-between"><span>Validado SIRE / RCE</span><strong className="font-mono text-foreground">{formatCurrency(item.entries.value)}</strong></span>} />
        <StatCard label="Salidas / costo de ventas" icon={ArrowUpFromLine} tone="danger" emphasizeValue value={`-${item.exits.qty}`} suffix="NIU" footer={<span className="flex w-full justify-between"><span>{item.exits.detail}</span><strong className="font-mono text-foreground">{formatCurrency(item.exits.value)}</strong></span>} />
        <StatCard label="Stock final valorizado" icon={Warehouse} highlight value={String(item.closing.qty)} suffix="NIU disponibles" footer={<span className="flex w-full justify-between"><span>Costo ponderado S/ {n4(item.closing.avgCost)}</span><strong className="font-mono">{formatCurrency(item.closing.value)}</strong></span>} />
      </div>

      <SectionCard
        title="Kardex del período"
        action={
          <div className="flex flex-wrap items-center gap-2">
            <Tabs value={filter} onValueChange={(v) => setFilter(String(v))}>
              <TabsList><TabsTrigger value="all">Todos <span className="ml-1 font-mono text-[11px] opacity-70">{count("all")}</span></TabsTrigger><TabsTrigger value="in">Entradas <span className="ml-1 font-mono text-[11px] opacity-70">{count("in")}</span></TabsTrigger><TabsTrigger value="out">Consumos / ventas <span className="ml-1 font-mono text-[11px] opacity-70">{count("out")}</span></TabsTrigger><TabsTrigger value="loss">Mermas <span className="ml-1 font-mono text-[11px] opacity-70">{count("loss")}</span></TabsTrigger></TabsList>
            </Tabs>
            <ToggleGroup value={[mode]} onValueChange={(v) => v[0] && setMode(v[0])} variant="outline" size="sm">
              <ToggleGroupItem value="13.1">Físico y valorizado (13.1)</ToggleGroupItem>
              <ToggleGroupItem value="12.1">Físico (12.1)</ToggleGroupItem>
              <ToggleGroupItem value="lote">Trazabilidad lote</ToggleGroupItem>
            </ToggleGroup>
          </div>
        }
        contentClassName="p-0"
      >
        <div className="flex flex-wrap gap-2 border-b px-4 py-3">
          <Select defaultValue="0000" items={warehouses}><SelectTrigger className="w-60" aria-label="Establecimiento"><SelectValue /></SelectTrigger><SelectContent>{warehouses.map((w) => <SelectItem key={w.value} value={w.value}>{w.label}</SelectItem>)}</SelectContent></Select>
          <Select defaultValue="2026-09" items={periods}><SelectTrigger className="w-56" aria-label="Período"><SelectValue /></SelectTrigger><SelectContent>{periods.map((p) => <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>)}</SelectContent></Select>
        </div>
        <div className="overflow-x-auto">
          <Table className="min-w-[1180px]">
            <TableHeader className="bg-muted/40">
              <TableRow className="[&>th]:h-8 [&>th]:text-center [&>th]:text-[11px] [&>th]:font-semibold [&>th]:uppercase">
                <TableHead colSpan={5} className="border-r pl-4! text-left!">1. Documento de traslado, venta o uso interno (tablas 10 y 12)</TableHead>
                <TableHead colSpan={3} className="border-r text-success!">2. Entradas</TableHead>
                <TableHead colSpan={mode === "12.1" ? 1 : 3} className="border-r text-destructive!">3. Salidas</TableHead>
                <TableHead colSpan={mode === "12.1" ? 1 : 3} className="pr-4!">4. Saldo final / existencias</TableHead>
              </TableRow>
              <TableRow className="[&>th]:h-8 [&>th]:whitespace-nowrap">
                <TableHead className="pl-4">Fecha</TableHead><TableHead>Tipo CPE</TableHead><TableHead>Serie – número</TableHead><TableHead>Tercero / área</TableHead><TableHead className="border-r">Tipo op.</TableHead>
                <TableHead className="text-right">Cant.</TableHead><TableHead className="text-right">C. unit.</TableHead><TableHead className="border-r text-right">C. total</TableHead>
                <TableHead className="text-right">Cant.</TableHead>{mode !== "12.1" ? <><TableHead className="text-right">C. unit.</TableHead><TableHead className="border-r text-right">C. total</TableHead></> : <TableHead className="border-r" />}
                <TableHead className="text-right">Cant.</TableHead>{mode !== "12.1" ? <><TableHead className="text-right">C. ponderado</TableHead><TableHead className="pr-4 text-right">Valor total</TableHead></> : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {visible.map((r) => {
                const isIn = r.kind === "entrada"; const isOut = r.kind === "salida";
                return (
                  <TableRow key={r.id} className={cn(r.kind === "inicial" && "bg-muted/30")}>
                    <TableCell className="pl-4 font-mono tabular-nums">{formatDate(r.date)}</TableCell>
                    <TableCell><Badge variant="outline" className="font-mono">{r.cpeType}</Badge></TableCell>
                    <TableCell><div className="flex flex-col"><span className="font-mono text-xs font-medium tabular-nums">{r.number}</span>{r.flag && mode === "lote" ? <span className="text-[11px] text-muted-foreground">{r.flag}</span> : null}</div></TableCell>
                    <TableCell className="max-w-48 truncate">{r.party}</TableCell>
                    <TableCell className="border-r whitespace-nowrap">{r.opType}</TableCell>
                    <TableCell className={cn("text-right font-mono tabular-nums", isIn && "font-semibold text-success")}>{isIn ? `+${r.qty}` : ""}</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{isIn && r.unitCost ? n4(r.unitCost) : ""}</TableCell>
                    <TableCell className="border-r text-right font-mono tabular-nums">{isIn ? n2(r.total) : r.kind === "inicial" ? <span className="text-muted-foreground">{n2(r.total)}</span> : ""}</TableCell>
                    <TableCell className={cn("text-right font-mono tabular-nums", isOut && "font-semibold text-destructive")}>{isOut ? r.qty : ""}</TableCell>
                    {mode !== "12.1" ? <><TableCell className="text-right font-mono tabular-nums">{isOut ? n4(r.avgCost) : ""}</TableCell><TableCell className="border-r text-right font-mono tabular-nums">{isOut ? n2(r.total) : ""}</TableCell></> : <TableCell className="border-r" />}
                    <TableCell className="text-right font-mono font-medium tabular-nums">{r.balanceQty}</TableCell>
                    {mode !== "12.1" ? <><TableCell className="text-right font-mono tabular-nums">{n4(r.avgCost)}</TableCell><TableCell className="pr-4 text-right font-mono font-semibold tabular-nums">{n2(r.balanceValue)}</TableCell></> : null}
                  </TableRow>
                );
              })}
              <TableRow className="bg-muted/40 font-medium">
                <TableCell colSpan={5} className="border-r pl-4">Totales del período ({item.period.split(" (")[0]})</TableCell>
                <TableCell className="text-right font-mono text-success tabular-nums">+{totals.inQty}</TableCell><TableCell /><TableCell className="border-r text-right font-mono tabular-nums">{n2(totals.in)}</TableCell>
                <TableCell className="text-right font-mono text-destructive tabular-nums">−{totals.outQty}</TableCell>{mode !== "12.1" ? <><TableCell /><TableCell className="border-r text-right font-mono tabular-nums">{n2(totals.out)}</TableCell></> : <TableCell className="border-r" />}
                <TableCell className="text-right font-mono tabular-nums">{item.closing.qty}</TableCell>{mode !== "12.1" ? <><TableCell className="text-right font-mono tabular-nums">{n4(item.closing.avgCost)}</TableCell><TableCell className="pr-4 text-right font-mono tabular-nums">{n2(item.closing.value)}</TableCell></> : null}
              </TableRow>
            </TableBody>
          </Table>
        </div>
        <div className="flex items-center gap-1.5 border-t px-4 pt-2 text-xs text-muted-foreground"><CheckCircle2 className="size-3.5 text-success" /> Consistencia con balance contable: OK</div>
        <PaginationBar from={1} to={visible.length} total={visible.length} label="operaciones registradas en el periodo" />
      </SectionCard>

      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Auditoría fiscal SIRE / RCE" icon={ShieldCheck} className="xl:col-span-2" contentClassName="flex flex-wrap items-center justify-between gap-3 p-4">
          <div className="flex items-start gap-3"><StatusBadge tone="success" icon={CheckCircle2} label="0 discrepancias" /><p className="max-w-xl text-sm text-muted-foreground">Los costos unitarios de entrada concuerdan al 100% con los comprobantes de compra validados en el Registro de Compras Electrónico.</p></div>
          <Button variant="outline" onClick={() => toast.success("Reglas SUNAT re-validadas · sin observaciones")}>Re-validar reglas SUNAT</Button>
        </SectionCard>
        <SectionCard title="Estructura oficial SUNAT (formato 13.1)" icon={FileText} contentClassName="flex flex-col gap-3 p-4">
          <code className="rounded bg-muted px-2 py-1 font-mono text-[11px] break-all">{item.pleFile}</code>
          <div className="grid grid-cols-3 gap-2">
            <Button onClick={() => toast.success("TXT PLE 13.1 generado")}><Download data-icon="inline-start" /> PLE 13.1</Button>
            <Button variant="outline"><FileSpreadsheet data-icon="inline-start" /> Excel</Button>
            <Button variant="outline"><FileText data-icon="inline-start" /> PDF</Button>
          </div>
        </SectionCard>
      </div>
    </>
  );
}

