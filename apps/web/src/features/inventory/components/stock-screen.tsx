"use client";

import { ArrowRightLeft, Columns3, Download, Filter, History, MoreHorizontal, Package, Send, SlidersHorizontal, Trash2, Truck, Warehouse } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import { stockState, type StockItem, type StockState, type WarehouseInfo } from "../mocks/stock";
import { FilterBar } from "@/components/shared/filter-bar";

const stateMeta: Record<StockState, { label: string; tone: BadgeTone }> = {
  optimo: { label: "Óptimo", tone: "success" },
  alerta: { label: "Alerta mínimo", tone: "warning" },
  critico: { label: "Crítico / agotado", tone: "danger" },
};
const stateOptions = [{ value: "all", label: "Todos los estados" }, { value: "optimo", label: "Stock óptimo" }, { value: "alerta", label: "Alerta mínimo" }, { value: "critico", label: "Crítico / agotado" }];

interface Props { items: StockItem[]; warehouses: WarehouseInfo[]; categories: string[] }

export function StockScreen({ items, warehouses, categories, kpis }: Props & { kpis?: React.ReactNode }) {
  const [warehouse, setWarehouse] = React.useState("all");
  const [category, setCategory] = React.useState("Todos");
  const [state, setState] = React.useState("all");
  const [query, setQuery] = React.useState("");
  const [transferOpen, setTransferOpen] = React.useState(false);

  const rows = items.filter((i) => {
    const s = query.trim().toLowerCase();
    return (warehouse === "all" || i.warehouseId === warehouse) && (category === "Todos" || i.category === category) &&
      (state === "all" || stockState(i) === state) && (!s || i.name.toLowerCase().includes(s) || i.sku.toLowerCase().includes(s));
  });
  const whName = (id: string) => warehouses.find((w) => w.id === id)?.short ?? id;

  const columns: Column<StockItem>[] = [
    { key: "sku", header: "SKU / cód.", cell: (i) => <span className="font-mono text-xs font-bold text-primary tabular-nums">{i.sku}</span> },
    { key: "name", header: "Producto y presentación", cell: (i) => <span className="flex items-center gap-3"><span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent text-primary dark:bg-muted"><Package className="size-4" /></span><TwoLine primary={<span className="text-sm font-bold">{i.name}</span>} secondary={i.presentation} /></span>, className: "max-w-80" },
    { key: "cat", header: "Categoría", cell: (i) => <span className="rounded-full bg-accent px-2.5 py-1 text-[11px] font-semibold text-primary dark:bg-muted">{i.category}</span> },
    { key: "wh", header: "Almacén", cell: (i) => whName(i.warehouseId) },
    { key: "lot", header: "Lote / venc.", cell: (i) => i.lot ? <TwoLine primary={<span className="font-mono text-xs">{i.lot}</span>} secondary={i.expiresAt ? `Vence ${i.expiresAt}` : undefined} /> : <span className="text-muted-foreground">—</span> },
    { key: "stock", header: "Stock actual", align: "right", cell: (i) => { const st = stockState(i); return <span className={cn("font-mono text-base font-bold tabular-nums", st === "critico" && "text-destructive", st === "alerta" && "text-blue-600")}>{i.stock} <span className="font-sans text-xs font-normal text-muted-foreground">{i.unit}</span></span>; } },
    { key: "min", header: "Mínimo", align: "right", cell: (i) => <span className="text-xs text-muted-foreground tabular-nums">{i.minimum} {i.unit}</span> },
    { key: "state", header: "Estado", cell: (i) => { const m = stateMeta[stockState(i)]; return <StatusBadge tone={m.tone} dot label={m.label} />; } },
    { key: "cost", header: "Costo prom.", align: "right", cell: (i) => <span className="font-mono tabular-nums">{formatCurrency(i.avgCost)}</span> },
    { key: "value", header: "Valor total", align: "right", cell: (i) => <span className="font-mono text-sm font-bold tabular-nums">{formatCurrency(i.avgCost * i.stock)}</span> },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (i) => (
        <div className="flex justify-end gap-0.5">
          <Button variant="ghost" size="icon-sm" className="text-primary" aria-label="Transferir" onClick={() => setTransferOpen(true)}><Send /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Kardex" onClick={() => toast.info(`Kardex de ${i.sku}`)}><History /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Más"><MoreHorizontal /></Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Control de stock por almacén"
        description="Gestión de existencias multisede, valorización de inventario a costo promedio ponderado y traslados internos SUNAT."
        actions={<div className="flex w-full flex-col items-stretch gap-2 lg:w-auto lg:shrink-0 lg:items-end">
          <Select value={warehouse} onValueChange={(v) => setWarehouse(String(v))} items={warehouses.map((w) => ({ value: w.id, label: w.name }))}>
            <SelectTrigger className="h-auto w-full rounded-xl bg-card py-1.5 shadow-xs sm:w-80" aria-label="Almacén de consulta"><Warehouse className="size-4 text-muted-foreground" /><span className="flex flex-col text-left leading-tight"><span className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Almacén de consulta</span><SelectValue className="font-semibold" /></span></SelectTrigger>
            <SelectContent>{warehouses.map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectContent>
          </Select>
          <div className="flex flex-wrap justify-end gap-2">
            <Button variant="outline" className="font-semibold"><SlidersHorizontal data-icon="inline-start" /> Ajuste de stock</Button>
            <Button variant="outline" className="font-semibold"><Download data-icon="inline-start" /> Exportar kardex</Button>
            <Button className="font-semibold" onClick={() => setTransferOpen(true)}><ArrowRightLeft data-icon="inline-start" /> Nueva transferencia</Button>
          </div>
        </div>}
      />
      {kpis}

      <FilterBar stack>
        <div className="flex flex-wrap items-center gap-2">
          <SearchInput placeholder="Buscar por SKU, código de barra, lote o nombre de insumo…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
          <Select value={state} onValueChange={(v) => setState(String(v))} items={stateOptions}>
            <SelectTrigger className="w-56 bg-muted/50" aria-label="Estado de stock"><Filter className="size-4 text-muted-foreground" /><SelectValue /></SelectTrigger>
            <SelectContent>{stateOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <Button variant="outline" className="bg-muted/50 font-semibold"><Columns3 data-icon="inline-start" /> Columnas</Button>
        </div>
        <div className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-0.5">
          {categories.map((c) => { const n = c === "Todos" ? items.length : items.filter((i) => i.category === c).length; return (
            <button key={c} type="button" onClick={() => setCategory(c)} aria-pressed={category === c} className={cn("shrink-0 rounded-full px-3 py-1 text-xs font-semibold transition-colors", category === c ? "bg-primary text-primary-foreground" : "bg-accent text-foreground/80 hover:bg-accent/70 dark:bg-muted")}>{c} ({n})</button>
          ); })}
        </div>
      </FilterBar>

      <SectionCard title={<span className="sr-only">Existencias</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <DataTable
          columns={columns} rows={rows} rowKey={(i) => i.id} minWidth="1180px"
          mobileCard={(i) => {
            const m = stateMeta[stockState(i)];
            return (
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{i.name}</p>
                  <p className="text-xs text-muted-foreground">{i.sku} · {whName(i.warehouseId)}</p>
                  <p className="text-xs text-muted-foreground">{i.presentation}</p>
                </div>
                <div className="flex flex-col items-end gap-1">
                  <span className="font-mono text-sm font-semibold tabular-nums">{i.stock} {i.unit} <span className="text-xs font-normal text-muted-foreground">/ mín. {i.minimum}</span></span>
                  <StatusBadge tone={m.tone} dot label={m.label} />
                </div>
              </div>
            );
          }}
        />
        <PaginationBar from={1} to={rows.length} total={342} label="insumos" pageSize={10} />
      </SectionCard>

      <div className="grid gap-4 md:grid-cols-3">
        {warehouses.filter((w) => w.id !== "all").map((w, i) => { const occ = [76, 91, 42][i]; const val = [58410.2, 16820.3, 9000][i]; const note = ["Despachos al día", "1 traslado pendiente", "Recepción abierta"][i]; return (
          <div key={w.id} className="rounded-xl border bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between"><p className="flex items-center gap-2 text-sm font-bold"><Warehouse className="size-4 text-primary" /> {w.name.split(" (")[0]}</p><StatusBadge tone={occ > 85 ? "info" : "success"} dot label={occ > 85 ? "Alta ocupación" : "Operativo"} /></div>
            <div className="mt-3 flex items-center justify-between text-xs"><span className="text-muted-foreground">Ocupación de capacidad:</span><span className="text-sm font-bold">{occ}%</span></div>
            <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", occ > 85 ? "bg-blue-600" : occ > 60 ? "bg-primary" : "bg-slate-400")} style={{ width: `${occ}%` }} /></div>
            <div className="mt-3 flex items-center justify-between text-xs"><span className="text-muted-foreground">Valorizado: <span className="font-mono text-foreground">{formatCurrency(val)}</span></span><span className="font-semibold text-primary">{note}</span></div>
          </div>
        ); })}
      </div>

      <TransferSheet open={transferOpen} onOpenChange={setTransferOpen} warehouses={warehouses.filter((w) => w.id !== "all")} />
    </>
  );
}

function TransferSheet({ open, onOpenChange, warehouses }: { open: boolean; onOpenChange: (o: boolean) => void; warehouses: WarehouseInfo[] }) {
  const [gre, setGre] = React.useState(true);
  const lines = [
    { sku: "MED-0441", name: "Anestesia lidocaína 2%", qty: 20, unit: "amp" },
    { sku: "INS-0102", name: "Guantes de nitrilo M", qty: 5, unit: "caja" },
  ];
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Nueva transferencia interna</SheetTitle>
          <SheetDescription>Traslado entre almacenes con guía de remisión electrónica opcional.</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Almacén origen">
            <Select defaultValue="w3" items={warehouses.map((w) => ({ value: w.id, label: w.name }))}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>{warehouses.map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Almacén destino">
            <Select defaultValue="w2" items={warehouses.map((w) => ({ value: w.id, label: w.name }))}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>{warehouses.map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Motivo oficial de traslado (SUNAT)" className="sm:col-span-2">
            <Select defaultValue="04" items={[{ value: "04", label: "04 – Traslado entre establecimientos de la misma empresa" }, { value: "13", label: "13 – Otros" }]}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="04">04 – Traslado entre establecimientos de la misma empresa</SelectItem><SelectItem value="13">13 – Otros</SelectItem></SelectContent>
            </Select>
          </Field>
          <Field label="Profesional / solicitante"><Input defaultValue="Dra. Valeria Mendoza" /></Field>
          <Field label="Fecha de traslado"><Input type="date" defaultValue="2026-09-12" /></Field>
          <div className="sm:col-span-2">
            <p className="mb-1.5 text-xs font-medium text-muted-foreground">Insumos para envío ({lines.length})</p>
            <ul className="divide-y rounded-md border">
              {lines.map((l) => (
                <li key={l.sku} className="flex items-center gap-3 px-3 py-2 text-sm">
                  <span className="font-mono text-xs text-muted-foreground">{l.sku}</span>
                  <span className="flex-1 truncate">{l.name}</span>
                  <Input defaultValue={l.qty} className="h-7 w-16 text-right font-mono" aria-label={`Cantidad ${l.name}`} />
                  <span className="w-8 text-xs text-muted-foreground">{l.unit}</span>
                  <Button variant="ghost" size="icon-xs" aria-label="Quitar"><Trash2 /></Button>
                </li>
              ))}
            </ul>
            <Button variant="link" size="sm" className="px-0">+ Agregar ítem</Button>
          </div>
          <label className="flex items-start gap-3 rounded-md border p-3 sm:col-span-2">
            <Checkbox checked={gre} onCheckedChange={(v) => setGre(Boolean(v))} className="mt-0.5" />
            <span>
              <span className="block text-sm font-medium">Generar guía de remisión electrónica (GRE remitente)</span>
              <span className="block text-xs text-muted-foreground">Emite automáticamente el comprobante de transporte ante SUNAT con código QR y firma digital.</span>
            </span>
          </label>
          <Field label="Nota / observación de salida" className="sm:col-span-2"><Textarea rows={2} /></Field>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Guardar borrador</Button>
          <Button onClick={() => { toast.success(gre ? "Transferencia despachada · GRE T001-000146 emitida" : "Transferencia despachada"); onOpenChange(false); }}>
            <Truck data-icon="inline-start" /> Despachar transferencia
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
