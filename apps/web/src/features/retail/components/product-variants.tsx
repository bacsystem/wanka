"use client";

import { Barcode, Printer, ScanLine, Tags, TrendingUp } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { colors, costHistory, priceLists, productMock, sizes, type Variant } from "../mocks/variants";

export function ProductVariants({ variants: initial }: { variants: Variant[] }) {
  const [variants, setVariants] = React.useState(initial);
  const [filter, setFilter] = React.useState("all");
  const [sel, setSel] = React.useState<Set<string>>(new Set());
  const [bulkOpen, setBulkOpen] = React.useState(false);
  const [pct, setPct] = React.useState("5");
  const total = (v: Variant) => v.stockMain + v.stockShop;
  const show = (v: Variant) => filter === "all" || (filter === "low" && total(v) < 10) || (filter === "noean" && !v.ean);
  const toggle = (id: string) => setSel((s) => { const n = new Set(s); if (n.has(id)) n.delete(id); else n.add(id); return n; });
  return (
    <>
      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Producto principal" contentClassName="grid gap-3 p-4 text-sm sm:grid-cols-2 xl:col-span-2">
          <div className="sm:col-span-2"><p className="text-lg font-semibold">{productMock.name}</p><p className="text-xs text-muted-foreground">SKU raíz <span className="font-mono">{productMock.sku}</span> · {productMock.category} · afectación {productMock.affectation}</p></div>
          <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Estructura de variación</p><p className="font-medium">Tallas: {sizes.join(", ")} ({sizes.length}) · Colores: {colors.map((c) => c.name.split(" ")[0]).join(", ")} ({colors.length})</p><p className="text-xs text-muted-foreground">Total celdas generadas: {variants.length} SKU</p></div>
          <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Stock consolidado</p><p className="font-mono text-xl font-semibold">{variants.reduce((s, v) => s + total(v), 0)} und <StatusBadge className="ml-1" tone="success" label="Óptimo" /></p><p className="text-xs text-muted-foreground">Alm. principal {variants.reduce((s, v) => s + v.stockMain, 0)} · Tienda mostrador {variants.reduce((s, v) => s + v.stockShop, 0)}</p></div>
          <div className="flex flex-wrap gap-2 sm:col-span-2"><Button variant="outline" onClick={() => toast.success("Códigos EAN-13 generados para 12 variantes")}><Barcode data-icon="inline-start" /> Generar códigos EAN-13</Button><Button variant="outline" onClick={() => toast.success(`${sel.size || variants.length} etiquetas enviadas a Zebra ZD220 (50×25 mm)`)}><Printer data-icon="inline-start" /> Imprimir etiquetas</Button><Button variant="outline" onClick={() => setBulkOpen(true)}><Tags data-icon="inline-start" /> Ajustar precios masivo</Button><Button render={<Link href="/ventas/nueva" />} nativeButton={false}><ScanLine data-icon="inline-start" /> Venta rápida POS</Button></div>
        </SectionCard>
        <SectionCard title="Historial de costo promedio" icon={TrendingUp} contentClassName="p-4 text-sm">
          <p className="font-mono text-2xl font-semibold">{formatCurrency(productMock.avgCost)}</p><p className="text-xs text-muted-foreground">Costo de reposición {formatCurrency(productMock.replacementCost)} · última compra RCE: {productMock.lastPurchase}</p>
          <ul className="mt-3 divide-y text-xs">{costHistory.map((c) => <li key={c.date} className="flex items-center justify-between py-1.5"><span className="font-mono">{formatDate(c.date)}</span><span className="flex-1 truncate px-2 text-muted-foreground">{c.doc} · {c.qty} und</span><span className="font-mono font-medium">{formatCurrency(c.cost)}</span></li>)}</ul>
        </SectionCard>
      </div>

      <SectionCard title="Matriz operativa de variantes (talla × color)" action={<Tabs value={filter} onValueChange={(v) => setFilter(String(v))}><TabsList><TabsTrigger value="all">Todos <span className="ml-1 font-mono text-[11px] opacity-70">{variants.length}</span></TabsTrigger><TabsTrigger value="low">Stock &lt; 10 <span className="ml-1 font-mono text-[11px] opacity-70">{variants.filter((v) => total(v) < 10).length}</span></TabsTrigger><TabsTrigger value="noean">Sin EAN</TabsTrigger></TabsList></Tabs>} contentClassName="p-0">
        <div className="hidden overflow-x-auto md:block"><Table className="min-w-[820px]"><TableHeader><TableRow><TableHead className="pl-4">Color / talla</TableHead>{sizes.map((s) => <TableHead key={s} className="text-left">Talla {s}</TableHead>)}</TableRow></TableHeader><TableBody>
          {colors.map((c) => <TableRow key={c.id} className="align-top"><TableCell className="px-4 py-3"><div className="flex items-center gap-2"><span className="size-4 rounded-full border" style={{ background: c.hex }} /><div><p className="font-medium">{c.name}</p><p className="font-mono text-xs text-muted-foreground">{c.code} · {variants.filter((v) => v.color === c.id).reduce((s, v) => s + total(v), 0)} und</p></div></div></TableCell>
            {sizes.map((s) => { const v = variants.find((x) => x.color === c.id && x.size === s)!; const low = total(v) < 10; const hidden = !show(v); return <TableCell key={s} className={cn("px-2 py-2", hidden && "opacity-30")}><label className={cn("flex cursor-pointer flex-col gap-0.5 rounded-md border p-2 text-xs hover:bg-muted/40", sel.has(v.id) && "border-primary bg-accent/40", low && "border-warning/50")}><span className="flex items-center justify-between"><span className="font-mono font-semibold">{formatCurrency(v.price)}</span><Checkbox checked={sel.has(v.id)} onCheckedChange={() => toggle(v.id)} aria-label={`Seleccionar ${v.sku}`} /></span><span className={cn("font-medium", low && "text-warning")}>{total(v)} und{low ? " (bajo)" : ""}</span><span className="font-mono text-muted-foreground">{v.ean}</span><span className="font-mono text-[10px] text-muted-foreground">{v.sku}</span><span className="text-muted-foreground">Alm. {v.stockMain} · Tda. {v.stockShop}</span></label></TableCell>; })}
          </TableRow>)}
        </TableBody></Table></div>
        <ul className="divide-y md:hidden">{colors.map((c) => <li key={c.id} className="px-4 py-3"><p className="mb-2 flex items-center gap-2 text-sm font-medium"><span className="size-3 rounded-full border" style={{ background: c.hex }} />{c.name}</p><ul className="grid grid-cols-2 gap-2">{variants.filter((v) => v.color === c.id && show(v)).map((v) => <li key={v.id} className={cn("rounded-md border p-2 text-xs", total(v) < 10 && "border-warning/50")}><p className="flex justify-between font-medium"><span>Talla {v.size}</span><span className="font-mono">{formatCurrency(v.price)}</span></p><p className={cn(total(v) < 10 && "text-warning")}>{total(v)} und</p><p className="font-mono text-[10px] text-muted-foreground">{v.ean}</p></li>)}</ul></li>)}</ul>
        {sel.size ? <div className="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2 text-xs"><span>{sel.size} variante{sel.size === 1 ? "" : "s"} seleccionada{sel.size === 1 ? "" : "s"}</span><div className="flex gap-2"><Button size="xs" variant="outline" onClick={() => toast.success(`${sel.size} etiquetas impresas`)}><Printer data-icon="inline-start" /> Imprimir seleccionadas</Button><Button size="xs" variant="outline" onClick={() => setBulkOpen(true)}><Tags data-icon="inline-start" /> Ajustar precio</Button></div></div> : null}
      </SectionCard>

      <SectionCard title="Listas de precios" contentClassName="p-0">
        <div className="overflow-x-auto"><Table className="min-w-[560px]"><TableHeader><TableRow><TableHead className="pl-4">Lista</TableHead><TableHead className="text-right">Precio (inc. IGV)</TableHead><TableHead className="text-right">Margen</TableHead><TableHead className="pl-4">Nota</TableHead></TableRow></TableHeader><TableBody>{priceLists.map((p) => <TableRow key={p.name}><TableCell className="px-4 py-2 font-medium">{p.name}</TableCell><TableCell className="px-2 py-2 text-right font-mono">{formatCurrency(p.price)}</TableCell><TableCell className={cn("px-2 py-2 text-right font-mono", p.margin < 35 && "text-warning")}>{p.margin.toFixed(1)}%</TableCell><TableCell className="px-4 py-2 text-xs text-muted-foreground">{p.note}</TableCell></TableRow>)}</TableBody></Table></div>
      </SectionCard>

      <Dialog open={bulkOpen} onOpenChange={setBulkOpen}>
        <DialogContent className="sm:max-w-md"><DialogHeader><DialogTitle>Ajustar precios masivo</DialogTitle><DialogDescription>{sel.size ? `${sel.size} variantes seleccionadas` : "Todas las variantes"} · precio minorista</DialogDescription></DialogHeader>
          <Field label="Variación (%)"><Input value={pct} onChange={(e) => setPct(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
          <p className="text-xs text-muted-foreground">Ejemplo: S/ 38.00 → {formatCurrency(38 * (1 + (Number(pct) || 0) / 100))}</p>
          <DialogFooter><Button variant="outline" onClick={() => setBulkOpen(false)}>Cancelar</Button><Button onClick={() => { const f = 1 + (Number(pct) || 0) / 100; setVariants((vs) => vs.map((v) => (sel.size === 0 || sel.has(v.id) ? { ...v, price: Math.round(v.price * f * 10) / 10 } : v))); setBulkOpen(false); toast.success(`Precios ajustados ${Number(pct) >= 0 ? "+" : ""}${pct}%`); }}>Aplicar</Button></DialogFooter>
        </DialogContent>
      </Dialog>
      <Badge variant="outline" className="w-fit"><Printer className="size-3" /> Zebra ZD220 · 50×25 mm · Lector láser USB conectado</Badge>
    </>
  );
}
