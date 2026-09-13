"use client";

import { Briefcase, Copy, Download, FolderTree, LayoutGrid, MoreHorizontal, Pencil, Pill, PlusCircle, ShieldCheck, Tags, Upload } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { PageHeader } from "@/components/shared/page-header";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import { affectationLabel, type CatalogCategory, type CatalogEntry } from "../mocks/catalog-items";
import { FilterBar } from "@/components/shared/filter-bar";

export type CatalogTab = "todos" | "servicios" | "productos" | "categorias";
const tabHref: Record<CatalogTab, string> = { todos: "/catalogo", servicios: "/catalogo/servicios", productos: "/catalogo/productos", categorias: "/catalogo/categorias" };

const affectationOptions = [{ value: "all", label: "Afectación: todas" }, { value: "10", label: "Gravado 18% (10)" }, { value: "20", label: "Exonerado (20)" }, { value: "30", label: "Inafecto (30)" }];

interface Props { tab: CatalogTab; entries: CatalogEntry[]; categories: CatalogCategory[]; total: number; kpis?: React.ReactNode }

export function CatalogScreen({ tab, entries, categories, total, kpis }: Props) {
  const router = useRouter();
  const [query, setQuery] = React.useState("");
  const [affect, setAffect] = React.useState("all");
  const [category, setCategory] = React.useState("all");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [open, setOpen] = React.useState(false);

  const categoryOptions = [{ value: "all", label: "Categoría: todas" }, ...categories.map((c) => ({ value: c.name, label: c.name }))];
  const rows = entries.filter((e) => {
    const s = query.trim().toLowerCase();
    return (tab === "todos" || tab === "categorias" || (tab === "servicios" ? e.nature === "servicio" : e.nature === "producto")) &&
      (affect === "all" || e.affectation === affect) && (category === "all" || e.category === category) &&
      (!s || e.name.toLowerCase().includes(s) || e.sku.toLowerCase().includes(s) || e.unspsc.includes(s));
  });
  const counts = { todos: entries.length, servicios: entries.filter((e) => e.nature === "servicio").length, productos: entries.filter((e) => e.nature === "producto").length, categorias: categories.length };
  const allSelected = rows.length > 0 && rows.every((r) => selected.has(r.id));

  const columns: Column<CatalogEntry>[] = [
    { key: "sel", header: <Checkbox aria-label="Seleccionar todos" checked={allSelected} onCheckedChange={(v) => setSelected(v ? new Set(rows.map((r) => r.id)) : new Set())} />, cell: (e) => <Checkbox aria-label={`Seleccionar ${e.sku}`} checked={selected.has(e.id)} onCheckedChange={(v) => setSelected((s) => { const n = new Set(s); if (v) n.add(e.id); else n.delete(e.id); return n; })} />, className: "w-8" },
    { key: "sku", header: "Código / SKU", cell: (e) => <span className="font-mono text-xs font-bold text-primary tabular-nums">{e.sku}</span> },
    { key: "name", header: "Descripción del ítem", cell: (e) => <TwoLine primary={<span className={cn("text-sm font-bold", !e.active && "text-muted-foreground line-through")}>{e.name}</span>} secondary={e.description} />, className: "max-w-80" },
    { key: "type", header: "Tipo • categoría", cell: (e) => <TwoLine primary={<span className={cn("inline-flex rounded px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase", e.nature === "servicio" ? "bg-primary text-primary-foreground" : e.category.toLowerCase().includes("farmacia") ? "bg-blue-50 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300" : "bg-accent text-primary dark:bg-muted")}>{e.nature === "servicio" ? "Procedimiento" : e.category.toLowerCase().includes("farmacia") ? "Fármaco / insumo" : "Material / bien"}</span>} secondary={e.category} /> },
    { key: "unspsc", header: "Código SUNAT (UNSPSC)", cell: (e) => <span className="flex items-center gap-1 font-mono text-xs tabular-nums">{e.unspsc} <ShieldCheck className="size-3.5 text-muted-foreground" /></span> },
    { key: "price", header: "Precio venta (inc. IGV)", align: "right", cell: (e) => <TwoLine primary={<span className="font-mono text-sm font-bold tabular-nums">{formatCurrency(e.price)}</span>} secondary={e.affectation === "10" ? `V.V: ${formatCurrency(e.price / 1.18)}` : e.cost ? `Costo: ${formatCurrency(e.cost)}` : "Exonerado IGV"} /> },
    { key: "igv", header: "Afectación IGV", align: "center", cell: (e) => <span className={cn("inline-flex rounded-full px-2.5 py-1 text-center text-[10px] leading-tight font-semibold", e.affectation === "10" ? "bg-teal-100 text-teal-900 dark:bg-teal-900/50 dark:text-teal-100" : "bg-muted text-muted-foreground")}>{affectationLabel[e.affectation]}</span> },
    { key: "stock", header: "Kardex / stock", cell: (e) => e.stock ? <span className="flex items-center gap-2"><span className={cn("size-2 rounded-full", e.stock.critical ? "bg-rose-500" : "bg-emerald-500")} /><TwoLine primary={<span className={cn("font-mono text-sm font-bold tabular-nums", e.stock.critical && "text-destructive")}>{e.stock.qty} {e.stock.unit}</span>} secondary={e.stock.critical ? <span className="text-destructive">(Crítico)</span> : `(${e.stock.warehouse})`} /></span> : <span className="text-xs text-muted-foreground">No aplica (servicio)</span> },
    { key: "tariff", header: "Tarifario EPS", cell: (e) => <span className={cn("text-xs", e.tariff.toLowerCase().includes("rimac") && "font-semibold text-primary")}>{e.tariff}</span>, className: "max-w-48" },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (e) => (
      <div className="flex justify-end gap-0.5">
        {e.nature === "producto" ? <Button variant="ghost" size="icon-sm" aria-label="Variantes y códigos de barras" render={<Link href={`/catalogo/variantes/${e.sku}`} />} nativeButton={false}><Tags /></Button> : null}
        <Button variant="ghost" size="icon-sm" aria-label="Editar" onClick={() => setOpen(true)}><Pencil /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Duplicar" onClick={() => toast.info(`${e.sku} duplicado como borrador`)}><Copy /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Más"><MoreHorizontal /></Button>
      </div>
    ) },
  ];

  const categoryColumns: Column<CatalogCategory>[] = [
    { key: "name", header: "Categoría", cell: (c) => <span className="font-medium">{c.name}</span> },
    { key: "spec", header: "Especialidad", cell: (c) => c.specialty },
    { key: "items", header: "Ítems", align: "right", cell: (c) => <span className="font-mono tabular-nums">{c.items}</span> },
    { key: "status", header: "Estado", cell: (c) => <StatusBadge tone={c.active ? "success" : "neutral"} dot label={c.active ? "Activa" : "Inactiva"} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: () => <Button variant="ghost" size="icon-sm" aria-label="Editar categoría"><Pencil /></Button> },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Catálogo · Servicios, productos y categorías"
        title="Catálogo de productos y servicios"
        description="Gestión unificada de procedimientos clínicos, fármacos e insumos. Configure códigos UNSPSC, regímenes de afectación al IGV (Catálogo 07) y aranceles por convenio EPS o tarifas particulares."
        status={<StatusBadge tone="info" dot label="Catálogo SUNAT v2026.2" />}
        actions={
          <>
            <Button variant="outline"><Upload data-icon="inline-start" /> Importar Excel</Button>
            <Button variant="outline"><Download data-icon="inline-start" /> Exportar</Button>
            <Button variant="secondary" className="bg-teal-100 font-semibold text-teal-900 hover:bg-teal-200 dark:bg-teal-900/50 dark:text-teal-100" onClick={() => toast.info("Nueva categoría")}><Tags data-icon="inline-start" /> Nueva categoría</Button>
            <Button className="font-semibold" onClick={() => setOpen(true)}><PlusCircle data-icon="inline-start" /> Nuevo servicio / producto</Button>
          </>
        }
      />
      <FilterBar className="justify-between p-2">
        <Tabs value={tab} onValueChange={(v) => router.push(tabHref[v as CatalogTab])} className="min-w-0 max-w-full">
          <TabsList variant="pills">
            {([["todos", "Todos los ítems", LayoutGrid, counts.todos], ["servicios", "Servicios", Briefcase, counts.servicios], ["productos", "Productos (kardex)", Pill, counts.productos], ["categorias", "Categorías y especialidades", FolderTree, counts.categorias]] as const).map(([v, l, Icon, n]) => (
              <TabsTrigger key={v} value={v}><Icon /> {l} <span className={cn("rounded-full px-1.5 font-mono text-[11px]", tab === v ? "bg-primary-foreground/20" : "bg-accent text-primary dark:bg-muted")}>{n}</span></TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <span className="hidden items-center gap-2 pr-2 text-xs text-muted-foreground lg:flex">Submenú activo: <span className="flex items-center gap-1 font-semibold text-primary"><span className="size-1.5 rounded-full bg-primary" /> {tab === "categorias" ? "Categorías" : "Vista consolidada"}</span></span>
      </FilterBar>
      {kpis}

      {tab === "categorias" ? (
        <SectionCard title="Categorías y especialidades" contentClassName="p-0">
          <DataTable columns={categoryColumns} rows={categories} rowKey={(c) => c.id} minWidth="640px" mobileCard={(c) => (
            <div className="flex items-center justify-between gap-3"><div><p className="text-sm font-medium">{c.name}</p><p className="text-xs text-muted-foreground">{c.specialty} · {c.items} ítems</p></div><StatusBadge tone={c.active ? "success" : "neutral"} dot label={c.active ? "Activa" : "Inactiva"} /></div>
          )} />
        </SectionCard>
      ) : (
        <SectionCard
          title={<span className="sr-only">Vista consolidada</span>}
          className={cn(selected.size === 0 && "[&>div:first-child]:hidden")}
          action={selected.size > 0 ? (
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className="text-muted-foreground">{selected.size} seleccionado{selected.size === 1 ? "" : "s"}</span>
              <Button size="xs" variant="outline" onClick={() => toast.info("Ajuste de margen aplicado")}>Ajustar margen (%)</Button>
              <Button size="xs" variant="outline" onClick={() => toast.info("Afectación IGV actualizada")}>Cambiar afectación IGV</Button>
              <Button size="xs" variant="outline" className="text-destructive" onClick={() => { toast.success(`${selected.size} ítems desactivados`); setSelected(new Set()); }}>Desactivar</Button>
            </div>
          ) : null}
          contentClassName="p-0"
        >
          <div className="flex flex-col gap-2 border-b p-3">
            <div className="flex flex-wrap items-center gap-2">
              <SearchInput placeholder="Buscar por nombre, SKU, UNSPSC…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
              <Select value={category} onValueChange={(v) => setCategory(String(v))} items={categoryOptions}>
                <SelectTrigger className="w-52 bg-muted/50" aria-label="Categoría"><SelectValue /></SelectTrigger>
                <SelectContent>{categoryOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
              </Select>
              <Select value={affect} onValueChange={(v) => setAffect(String(v))} items={affectationOptions}>
                <SelectTrigger className="w-44 bg-muted/50" aria-label="Afectación IGV"><SelectValue /></SelectTrigger>
                <SelectContent>{affectationOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
              </Select>
              <Select defaultValue="all" items={[{ value: "all", label: "Tarifario: todos" }, { value: "rimac", label: "Tarifario: Rimac EPS" }, { value: "pacifico", label: "Tarifario: Pacífico" }]}>
                <SelectTrigger className="w-44 bg-muted/50" aria-label="Tarifario"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value="all">Tarifario: todos</SelectItem><SelectItem value="rimac">Tarifario: Rimac EPS</SelectItem><SelectItem value="pacifico">Tarifario: Pacífico</SelectItem></SelectContent>
              </Select>
            </div>
            <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
              <span className="flex flex-wrap items-center gap-2">Filtros aplicados: <span className="rounded-full border bg-card px-2.5 py-0.5 font-medium text-foreground">Tipo: {tab === "servicios" ? "Clínico • Tratamientos" : tab === "productos" ? "Productos con kardex" : "Todos"}</span><span className="rounded-full border bg-card px-2.5 py-0.5 font-medium text-foreground">Estado: Activo</span><button type="button" className="font-semibold text-primary hover:underline" onClick={() => { setQuery(""); setAffect("all"); setCategory("all"); }}>Restablecer filtros</button></span>
              <span>Mostrando 1 - {rows.length} de {total} resultados</span>
            </div>
          </div>
          <DataTable
            columns={columns} rows={rows} rowKey={(e) => e.id} minWidth="1240px"
            mobileCard={(e) => (
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="font-mono text-xs text-muted-foreground">{e.sku} · {e.unspsc}</p>
                  <p className={cn("truncate text-sm font-medium", !e.active && "line-through text-muted-foreground")}>{e.name}</p>
                  <p className="text-xs text-muted-foreground">{e.category}</p>
                  <div className="mt-1 flex flex-wrap gap-1"><StatusBadge tone={e.affectation === "10" ? "success" : "warning"} label={affectationLabel[e.affectation]} />{e.stock ? <Badge variant="outline" className={cn("font-mono", e.stock.critical && "text-destructive")}>{e.stock.qty} {e.stock.unit}</Badge> : null}</div>
                </div>
                <span className="font-mono text-sm font-semibold tabular-nums">{formatCurrency(e.price)}</span>
              </div>
            )}
          />
          <PaginationBar from={1} to={rows.length} total={total} label="ítems" />
        </SectionCard>
      )}

      <ItemSheet open={open} onOpenChange={setOpen} categories={categories} />
    </>
  );
}

function ItemSheet({ open, onOpenChange, categories }: { open: boolean; onOpenChange: (o: boolean) => void; categories: CatalogCategory[] }) {
  const [step, setStep] = React.useState(0);
  const [nature, setNature] = React.useState("servicio");
  const [price, setPrice] = React.useState("85");
  const [cost, setCost] = React.useState("33");
  const [affect, setAffect] = React.useState("10");
  const [trackStock, setTrackStock] = React.useState(false);
  const p = Number(price) || 0;
  const base = affect === "10" ? p / 1.18 : p;
  const igv = p - base;
  const margin = p ? ((base - (Number(cost) || 0)) / base) * 100 : 0;
  const steps = ["1. Datos generales", "2. Precios y SUNAT", "3. Kardex / stock"];

  return (
    <Sheet open={open} onOpenChange={(o) => { onOpenChange(o); if (!o) setStep(0); }}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Nuevo producto o procedimiento</SheetTitle>
          <SheetDescription>Catálogo unificado y reglas de facturación SUNAT.</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <ol className="mb-4 grid grid-cols-3 gap-1">
            {steps.map((s, i) => (
              <li key={s}><button type="button" onClick={() => setStep(i)} className={cn("w-full rounded-md border px-2 py-1.5 text-xs font-medium", i === step ? "border-primary bg-accent text-accent-foreground" : i < step ? "border-success/40 text-success" : "text-muted-foreground")}>{s}</button></li>
            ))}
          </ol>
          {step === 0 ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Naturaleza del ítem" className="sm:col-span-2">
                <RadioGroup value={nature} onValueChange={(v) => setNature(String(v))} className="grid grid-cols-2 gap-2">
                  {[["servicio", "Servicio / procedimiento", "Honorarios, cirugías, atenciones"], ["producto", "Bien / producto físico", "Fármacos, brackets, consumibles"]].map(([v, l, d]) => (
                    <label key={v} className={cn("flex cursor-pointer items-start gap-2 rounded-md border p-3", nature === v && "border-primary bg-accent/40")}><RadioGroupItem value={v} className="mt-0.5" /><span><span className="block text-sm font-medium">{l}</span><span className="block text-xs text-muted-foreground">{d}</span></span></label>
                  ))}
                </RadioGroup>
              </Field>
              <Field label="Nombre comercial o clínico" className="sm:col-span-2"><Input placeholder="Curación con resina fotocurable" /></Field>
              <Field label="Código interno (SKU)"><Input placeholder="SRV-0045" className="font-mono uppercase" /></Field>
              <Field label="Código SUNAT (UNSPSC)"><Input placeholder="85122001" className="font-mono" /></Field>
              <Field label="Especialidad / categoría">
                <Select defaultValue={categories[0].name} items={categories.map((c) => ({ value: c.name, label: c.name }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{categories.map((c) => <SelectItem key={c.id} value={c.name}>{c.name}</SelectItem>)}</SelectContent></Select>
              </Field>
              <Field label="Unidad de medida (cat. 03)">
                <Select defaultValue={nature === "servicio" ? "ZZ" : "NIU"} items={[{ value: "ZZ", label: "ZZ – Unidad de servicio" }, { value: "NIU", label: "NIU – Unidad (bienes)" }, { value: "BX", label: "BX – Caja" }, { value: "KT", label: "KT – Kit / set" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ZZ">ZZ – Unidad de servicio</SelectItem><SelectItem value="NIU">NIU – Unidad (bienes)</SelectItem><SelectItem value="BX">BX – Caja</SelectItem><SelectItem value="KT">KT – Kit / set</SelectItem></SelectContent></Select>
              </Field>
              <Field label="Descripción / glosa para boleta o factura" className="sm:col-span-2"><Textarea rows={2} /></Field>
            </div>
          ) : step === 1 ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="rounded-md border bg-accent/30 p-3 text-xs sm:col-span-2"><p className="font-medium">Cálculo automático de impuestos SUNAT</p><p className="text-muted-foreground">El sistema desglosa valor venta e IGV (18%) conforme a la Ley de Comprobantes de Pago.</p></div>
              <Field label="Tipo de afectación al IGV (catálogo 07)" className="sm:col-span-2">
                <Select value={affect} onValueChange={(v) => setAffect(String(v))} items={[{ value: "10", label: "10 – Gravado, operación onerosa (18%)" }, { value: "20", label: "20 – Exonerado, operación onerosa" }, { value: "30", label: "30 – Inafecto, operación onerosa" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="10">10 – Gravado, operación onerosa (18%)</SelectItem><SelectItem value="20">20 – Exonerado, operación onerosa</SelectItem><SelectItem value="30">30 – Inafecto, operación onerosa</SelectItem></SelectContent></Select>
              </Field>
              <Field label="Precio venta (inc. IGV)"><Input value={price} onChange={(e) => setPrice(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
              <Field label="Costo unitario estimado"><Input value={cost} onChange={(e) => setCost(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
              <dl className="grid grid-cols-3 gap-2 rounded-md bg-muted/40 p-3 text-sm sm:col-span-2">
                <div><dt className="text-xs text-muted-foreground">Valor venta (neto)</dt><dd className="font-mono tabular-nums">{formatCurrency(base)}</dd></div>
                <div><dt className="text-xs text-muted-foreground">Monto IGV</dt><dd className="font-mono tabular-nums">{formatCurrency(igv)}</dd></div>
                <div><dt className="text-xs text-muted-foreground">Margen bruto</dt><dd className={cn("font-mono tabular-nums", margin < 30 && "text-warning")}>{margin.toFixed(1)}%</dd></div>
              </dl>
              <Field label="Listas y convenios de aseguradoras" className="sm:col-span-2">
                <div className="grid gap-2 text-sm sm:grid-cols-2">
                  <div className="flex items-center justify-between rounded-md border px-3 py-2"><span>Rimac Seguros (−15%)</span><span className="font-mono tabular-nums">{formatCurrency(p * 0.85)}</span></div>
                  <div className="flex items-center justify-between rounded-md border px-3 py-2"><span>Pacífico Salud (−12%)</span><span className="font-mono tabular-nums">{formatCurrency(p * 0.88)}</span></div>
                </div>
              </Field>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex items-center justify-between rounded-md border p-3 sm:col-span-2">
                <div><p className="text-sm font-medium">Controlar stock en tiempo real</p><p className="text-xs text-muted-foreground">Descuenta existencias con cada boleta o factura emitida.</p></div>
                <Switch checked={trackStock} onCheckedChange={setTrackStock} aria-label="Controlar stock" />
              </div>
              <Field label="Stock inicial"><Input type="number" defaultValue={0} disabled={!trackStock} className="font-mono" /></Field>
              <Field label="Alerta de stock mínimo"><Input type="number" defaultValue={5} disabled={!trackStock} className="font-mono" /></Field>
              <Field label="Almacén principal asignado" className="sm:col-span-2">
                <Select defaultValue="central" disabled={!trackStock} items={[{ value: "central", label: "Almacén Central – Sede Miraflores" }, { value: "box1", label: "Botiquín consultorio 1 (operatoria)" }, { value: "box2", label: "Botiquín consultorio 2 (ortodoncia)" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="central">Almacén Central – Sede Miraflores</SelectItem><SelectItem value="box1">Botiquín consultorio 1 (operatoria)</SelectItem><SelectItem value="box2">Botiquín consultorio 2 (ortodoncia)</SelectItem></SelectContent></Select>
              </Field>
              <label className="flex items-center gap-2 text-sm sm:col-span-2"><Checkbox disabled={!trackStock} /> Exigir código de lote y vencimiento en cada ingreso</label>
            </div>
          )}
        </div>
        <SheetFooter className="flex-row justify-between">
          <Button variant="ghost" onClick={() => (step === 0 ? onOpenChange(false) : setStep(step - 1))}>{step === 0 ? "Cancelar" : "Atrás"}</Button>
          {step < 2 ? (
            <Button onClick={() => setStep(step + 1)}>Siguiente</Button>
          ) : (
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => { toast.success("Ítem guardado"); setStep(0); }}>Guardar y nuevo</Button>
              <Button onClick={() => { toast.success("Ítem guardado en el catálogo"); onOpenChange(false); }}>Guardar en catálogo</Button>
            </div>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
