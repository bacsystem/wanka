"use client";

import { ArrowDownToLine, ArrowRightLeft, ArrowUpFromLine, CalendarDays, Download, FileText, FilterX, History, PackageMinus, Plus, Printer, ShieldCheck, SlidersHorizontal, Upload } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { InventoryTabs } from "./inventory-tabs";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { sunatTable12, type InventoryMovement, type MovementKind } from "../mocks/movements";
import { FilterBar } from "@/components/shared/filter-bar";

const kindMeta: Record<MovementKind, { label: string; tone: BadgeTone; icon: typeof Plus }> = {
  entrada: { label: "Entrada", tone: "success", icon: ArrowDownToLine },
  salida: { label: "Salida", tone: "info", icon: ArrowUpFromLine },
  traslado: { label: "Traslado", tone: "warning", icon: ArrowRightLeft },
  merma: { label: "Merma / baja", tone: "danger", icon: PackageMinus },
  ajuste: { label: "Ajuste físico", tone: "neutral", icon: SlidersHorizontal },
};
const warehouseOptions = [{ value: "all", label: "Sede / almacén: todos" }, { value: "central", label: "Almacén Central Miraflores" }, { value: "farmacia", label: "Farmacia Clínica Miraflores" }, { value: "sanisidro", label: "Bodega San Isidro" }];
const opOptions = [{ value: "all", label: "Operación SUNAT: todas" }, ...sunatTable12.map((t) => ({ value: t.code, label: t.label }))];

export function MovementsScreen({ movements, total, kpis }: { movements: InventoryMovement[]; total: number; kpis?: React.ReactNode }) {
  const [query, setQuery] = React.useState("");
  const [op, setOp] = React.useState("all");
  const [open, setOpen] = React.useState(false);

  const rows = movements.filter((m) => {
    const s = query.trim().toLowerCase();
    return (op === "all" || m.sunatCode === op) && (!s || m.folio.toLowerCase().includes(s) || m.itemName.toLowerCase().includes(s) || m.sku.toLowerCase().includes(s) || m.reference.toLowerCase().includes(s));
  });

  const columns: Column<InventoryMovement>[] = [
    { key: "folio", header: "N° operación / folio", cell: (m) => { const k = kindMeta[m.kind]; return <span className={cn("flex items-center gap-2 border-l-4 pl-2", { entrada: "border-l-primary", salida: "border-l-rose-500", traslado: "border-l-blue-500", merma: "border-l-slate-400", ajuste: "border-l-amber-500" }[m.kind])}><TwoLine primary={<span className="font-mono text-sm font-bold tabular-nums">{m.folio}</span>} secondary={<StatusBadge tone={k.tone} label={k.label} className="rounded-md" />} /></span>; } },
    { key: "at", header: "Fecha y hora", cell: (m) => <TwoLine primary={formatDate(m.at)} secondary={m.at.slice(11, 16)} mono />, className: "whitespace-nowrap" },
    { key: "sunat", header: "Tipo y motivo SUNAT (T-12)", cell: (m) => <TwoLine primary={<span className="font-medium">{m.sunatCode} – {m.sunatLabel}</span>} secondary={m.reference} />, className: "max-w-64" },
    { key: "wh", header: "Almacén origen / destino", cell: (m) => <TwoLine primary={m.warehouse} secondary={m.location} /> },
    { key: "item", header: "Ítem / SKU", cell: (m) => <TwoLine primary={<span className="font-medium">{m.itemName}</span>} secondary={`SKU: ${m.sku} · ${m.presentation}`} />, className: "max-w-64" },
    { key: "lot", header: "Lote y caducidad", cell: (m) => <TwoLine primary={<span className="font-mono text-xs">{m.lot ?? "—"}</span>} secondary={m.expires ? `Venc. ${m.expires}` : "N/A"} /> },
    { key: "qty", header: "Cantidad", align: "right", cell: (m) => <TwoLine primary={<span className={cn("font-mono font-semibold tabular-nums", m.quantity < 0 ? "text-destructive" : "text-success")}>{m.quantity > 0 ? "+" : ""}{m.quantity} {m.unit}</span>} secondary={m.balance} /> },
    { key: "cost", header: "Costo / total", align: "right", cell: (m) => <TwoLine primary={<span className="font-mono tabular-nums">{formatCurrency(m.unitCost)}</span>} secondary={<span className="font-mono font-medium tabular-nums">{formatCurrency(Math.abs(m.quantity) * m.unitCost)}</span>} /> },
    { key: "who", header: "Responsable", cell: (m) => <TwoLine primary={m.responsible} secondary={m.role} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (m) => (
      <div className="flex justify-end gap-0.5">
        <Button variant="ghost" size="icon-sm" aria-label="Ver comprobante" onClick={() => toast.info(m.reference)}><FileText /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Imprimir"><Printer /></Button>
      </div>
    ) },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Inventario · Movimientos de almacén"
        title="Movimientos de inventario"
        description="Registro cronológico y trazabilidad de entradas, salidas, mermas, ajustes de inventario y transferencias entre sedes con códigos de operación SUNAT (Tabla 12 del RCE y Kardex Permanente Valorizado)."
        actions={
          <>
            <Button variant="outline" className="font-semibold" onClick={() => toast.info("Importa guías o el RCE para generar entradas")}><Upload data-icon="inline-start" /> Importar guías / RCE</Button>
            <Button variant="outline" className="font-semibold"><Download data-icon="inline-start" /> Exportar</Button>
            <Button className="font-semibold" onClick={() => setOpen(true)}><Plus data-icon="inline-start" /> Registrar movimiento</Button>
          </>
        }
      />
      <InventoryTabs active="/inventario/movimientos" />
      {kpis}

      <FilterBar stack className="gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <SearchInput placeholder="Buscar folio, SKU o insumo (ej: Resina Z350, Lidocaína)…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
          <Select defaultValue="all" items={warehouseOptions}>
            <SelectTrigger className="w-60 bg-muted/50" aria-label="Sede / almacén"><SelectValue /></SelectTrigger>
            <SelectContent>{warehouseOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <Select value={op} onValueChange={(v) => setOp(String(v))} items={opOptions}>
            <SelectTrigger className="w-64 bg-muted/50" aria-label="Operación SUNAT"><SelectValue /></SelectTrigger>
            <SelectContent>{opOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <Button variant="outline" className="bg-muted/50 font-mono text-xs"><CalendarDays data-icon="inline-start" /> 01/09 – 30/09</Button>
          <Button variant="outline" size="icon" className="bg-muted/50" aria-label="Limpiar filtros" onClick={() => { setQuery(""); setOp("all"); }}><FilterX /></Button>
        </div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">Filtros activos: <span className="rounded-full border bg-card px-2.5 py-0.5 font-medium text-foreground">Miraflores ×</span><span className="rounded-full border bg-card px-2.5 py-0.5 font-medium text-foreground">Mes actual: setiembre 2026 ×</span><button type="button" className="font-semibold text-primary hover:underline" onClick={() => { setQuery(""); setOp("all"); }}>Restablecer todo</button></div>
      </FilterBar>

      <SectionCard title={<span className="sr-only">Registro cronológico</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <DataTable
          columns={columns} rows={rows} rowKey={(m) => m.id} minWidth="1360px"
          mobileCard={(m) => { const k = kindMeta[m.kind]; return (
            <div className="flex items-start gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2"><span className="font-mono text-xs font-medium text-primary">{m.folio}</span><StatusBadge tone={k.tone} label={k.label} /></div>
                <p className="truncate text-sm font-medium">{m.itemName}</p>
                <p className="text-xs text-muted-foreground">{m.sunatCode} – {m.sunatLabel} · {m.warehouse}</p>
                <p className="text-xs text-muted-foreground">{formatDate(m.at)} {m.at.slice(11, 16)} · {m.responsible}</p>
              </div>
              <div className="text-right"><p className={cn("font-mono text-sm font-semibold tabular-nums", m.quantity < 0 ? "text-destructive" : "text-success")}>{m.quantity > 0 ? "+" : ""}{m.quantity} {m.unit}</p><p className="font-mono text-xs text-muted-foreground">{formatCurrency(Math.abs(m.quantity) * m.unitCost)}</p></div>
            </div>
          ); }}
        />
        <PaginationBar from={1} to={rows.length} total={total} label="registros valorizados" />
      </SectionCard>

      <FilterBar className="justify-between gap-3 p-4">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-accent text-primary"><ShieldCheck className="size-5" /></span>
          <div><p className="text-base font-bold">Kardex permanente valorizado conforme a SUNAT</p><p className="text-xs text-muted-foreground">Estructura 13.1 (Inventario Permanente Valorizado). Tipo de costeo aplicado: <span className="font-semibold text-foreground">Promedio ponderado móvil</span>. Sincronizado con el libro electrónico SIRE / RCE.</p></div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" className="bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted"><History data-icon="inline-start" /> Auditoría de saldos</Button>
          <Button className="font-semibold" onClick={() => toast.success("Libro 13.1 generado (TXT / PLE)")}><Download data-icon="inline-start" /> Generar libro TXT / PLE</Button>
        </div>
      </FilterBar>

      <MovementSheet open={open} onOpenChange={setOpen} />
    </>
  );
}

function MovementSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [flow, setFlow] = React.useState("entrada");
  const [qty, setQty] = React.useState("10");
  const [cost, setCost] = React.useState("78.50");
  const total = (Number(qty) || 0) * (Number(cost) || 0);
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Registrar movimiento de inventario</SheetTitle>
          <SheetDescription>Ingreso, salida, merma técnica o ajuste físico con código SUNAT (Tabla 12).</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Tipo de flujo" className="sm:col-span-2">
            <RadioGroup value={flow} onValueChange={(v) => setFlow(String(v))} className="grid grid-cols-3 gap-2">
              {[["entrada", "Entrada"], ["salida", "Salida"], ["ajuste", "Ajuste / merma"]].map(([v, l]) => (
                <label key={v} className={cn("flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm", flow === v && "border-primary bg-accent/40")}><RadioGroupItem value={v} /> {l}</label>
              ))}
            </RadioGroup>
          </Field>
          <Field label="Código SUNAT Tabla 12" className="sm:col-span-2">
            <Select defaultValue={flow === "entrada" ? "01" : flow === "salida" ? "02" : "99"} items={sunatTable12.map((t) => ({ value: t.code, label: t.label }))}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>{sunatTable12.map((t) => <SelectItem key={t.code} value={t.code}>{t.label}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Almacén origen / sede">
            <Select defaultValue="central" items={warehouseOptions.slice(1)}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{warehouseOptions.slice(1).map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select>
          </Field>
          <Field label="Comprobante de referencia"><Input placeholder="F001-4921 / GRE T001-000146" className="font-mono" /></Field>
          <Field label="Insumo odontológico / medicamento" className="sm:col-span-2" help="Stock actual: 140 cajas · costo promedio actual: S/ 78.50"><Input defaultValue="Lidocaína 2% c/epinefrina · MED-0441" /></Field>
          <Field label="Lote de fabricación"><Input placeholder="LT-98214" className="font-mono" /></Field>
          <Field label="Fecha de caducidad"><Input type="month" defaultValue="2027-11" /></Field>
          <Field label="Cantidad a mover"><Input value={qty} onChange={(e) => setQty(e.target.value)} inputMode="numeric" className="font-mono" /></Field>
          <Field label="Costo unitario (S/ sin IGV)"><Input value={cost} onChange={(e) => setCost(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
          <div className="flex items-center justify-between rounded-md bg-muted/40 p-3 text-sm sm:col-span-2"><span className="text-muted-foreground">Valor del movimiento</span><span className="font-mono font-semibold tabular-nums">{formatCurrency(total)}</span></div>
          <Field label="Responsable de auditoría"><Input defaultValue="Lic. Patricia Vega" /></Field>
          <Field label="Observaciones / justificación" className="sm:col-span-2"><Textarea rows={2} /></Field>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={() => { toast.success("Movimiento MOV-2026-001291 aplicado al kardex"); onOpenChange(false); }}>Guardar y aplicar al kardex</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
