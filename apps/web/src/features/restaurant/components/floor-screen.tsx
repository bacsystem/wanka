"use client";

import { Bike, CalendarPlus, ChefHat, Clock, Plus, Printer, Receipt, Split, Users, Utensils } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useMediaQuery } from "@/hooks/use-media-query";
import { SectionCard } from "@/components/shared/section-card";
import { SearchInput } from "@/components/shared/toolbar";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import { areas, menu, orderM7, type DiningTable, type OrderItem, type TableStatus } from "../mocks/floor";

const statusMeta: Record<TableStatus, { label: string; tone: BadgeTone; card: string; dot: string; chip: string }> = {
  libre: { label: "Libre", tone: "success", card: "border-emerald-200 bg-emerald-50/60 hover:bg-emerald-50 dark:border-emerald-900 dark:bg-emerald-950/30", dot: "bg-emerald-500 text-white", chip: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300" },
  ocupada: { label: "Ocupada", tone: "warning", card: "border-amber-200 bg-amber-50/60 hover:bg-amber-50 dark:border-amber-900 dark:bg-amber-950/30", dot: "bg-amber-500 text-white", chip: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300" },
  por_cobrar: { label: "Por cobrar", tone: "info", card: "border-primary/50 bg-accent/60 hover:bg-accent dark:bg-accent/20", dot: "bg-primary text-primary-foreground", chip: "bg-teal-100 text-teal-900 dark:bg-teal-900/50 dark:text-teal-100" },
  reservada: { label: "Reservada", tone: "neutral", card: "border-violet-200 bg-violet-50/60 hover:bg-violet-50 dark:border-violet-900 dark:bg-violet-950/30", dot: "bg-violet-500 text-white", chip: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300" },
  sucia: { label: "Sucia / limpieza", tone: "neutral", card: "border-border bg-muted/60 hover:bg-muted", dot: "bg-slate-400 text-white", chip: "bg-muted text-muted-foreground" },
};
const itemStatus = { en_cocina: { label: "En cocina", tone: "warning" as const }, listo: { label: "Listo", tone: "success" as const }, servido: { label: "Servido", tone: "neutral" as const } };

export function FloorScreen({ tables }: { tables: DiningTable[] }) {
  const [area, setArea] = React.useState("salon");
  const [selectedId, setSelectedId] = React.useState<string>("M7");
  const [order, setOrder] = React.useState<OrderItem[]>(orderM7);
  const [menuOpen, setMenuOpen] = React.useState(false);
  const [menuCat, setMenuCat] = React.useState<keyof typeof menu>("Fondos");
  const [q, setQ] = React.useState("");
  const [sheetOpen, setSheetOpen] = React.useState(false);
  const belowXl = useMediaQuery("(max-width: 1279px)");
  const selected = tables.find((t) => t.id === selectedId) ?? null;
  const counts = tables.reduce<Record<TableStatus, number>>((a, t) => ({ ...a, [t.status]: (a[t.status] ?? 0) + 1 }), { libre: 0, ocupada: 0, por_cobrar: 0, reservada: 0, sucia: 0 });
  const total = order.reduce((s, i) => s + i.qty * i.price, 0);
  const menuItems = menu[menuCat].filter((m) => !q || m.name.toLowerCase().includes(q.toLowerCase()));

  function pick(t: DiningTable) { setSelectedId(t.id); if (belowXl) setSheetOpen(true); }
  function add(m: { name: string; price: number }) { setOrder((o) => { const ex = o.find((i) => i.name === m.name); return ex ? o.map((i) => (i.name === m.name ? { ...i, qty: i.qty + 1 } : i)) : [...o, { id: `n${Date.now()}`, qty: 1, name: m.name, price: m.price, station: menuCat === "Bebidas" ? "Bar" : menuCat === "Entradas" ? "Entrada" : "Caliente", status: "en_cocina" }]; }); toast.success(`${m.name} agregado a la comanda`); }

  const panel = selected ? (
    <div className="flex h-full flex-col">
      <div className="border-b p-4">
        <div className="flex items-start justify-between gap-2"><div><p className="text-xs text-muted-foreground uppercase">Mesa activa</p><h2 className="text-lg font-semibold">Mesa {selected.label.replace(/^[MTB]/, "")} <span className="text-sm font-normal text-muted-foreground">· {selected.seats} personas</span></h2></div><StatusBadge tone={statusMeta[selected.status].tone} dot label={statusMeta[selected.status].label} /></div>
        <p className="mt-1 flex flex-wrap gap-x-3 text-xs text-muted-foreground">{selected.waiter ? <span>Mozo: {selected.waiter}</span> : null}{selected.since ? <span className="inline-flex items-center gap-1"><Clock className="size-3" /> {selected.since} en mesa</span> : null}{selected.note ? <span>{selected.note}</span> : null}</p>
      </div>
      {selected.status === "ocupada" || selected.status === "por_cobrar" ? (
        <>
          <div className="flex items-center justify-between px-4 py-2 text-xs text-muted-foreground"><span>Comanda · {order.length} ítems / {order.reduce((s, i) => s + i.qty, 0)} raciones</span><Button variant="ghost" size="xs"><Printer data-icon="inline-start" /> Imprimir</Button></div>
          <ul className="flex-1 divide-y overflow-y-auto">{order.map((i) => <li key={i.id} className="flex items-start gap-3 px-4 py-2.5 text-sm"><span className="font-mono font-semibold">{i.qty}×</span><div className="min-w-0 flex-1"><p className="font-medium">{i.name}</p>{i.detail ? <p className="text-xs text-muted-foreground">{i.detail}</p> : null}<StatusBadge className="mt-1" tone={itemStatus[i.status].tone} label={`${itemStatus[i.status].label} · ${i.station}`} /></div><span className="font-mono tabular-nums">{formatCurrency(i.qty * i.price)}</span></li>)}</ul>
          {menuOpen ? (
            <div className="border-t p-3">
              <div className="flex items-center gap-2"><SearchInput value={q} onChange={setQ} placeholder="Buscar plato…" aria-label="Buscar plato" className="flex-1 sm:max-w-none" /><Button variant="ghost" size="sm" onClick={() => setMenuOpen(false)}>Cerrar</Button></div>
              <Tabs value={menuCat} onValueChange={(v) => setMenuCat(v as keyof typeof menu)} className="mt-2"><TabsList>{(Object.keys(menu) as (keyof typeof menu)[]).map((c) => <TabsTrigger key={c} value={c}>{c}</TabsTrigger>)}</TabsList></Tabs>
              <ul className="mt-2 grid max-h-48 gap-1 overflow-y-auto">{menuItems.map((m) => <li key={m.name}><button type="button" onClick={() => add(m)} className="flex w-full items-center justify-between rounded-md border px-3 py-2 text-left text-sm hover:bg-accent"><span>{m.name}</span><span className="font-mono text-xs">{formatCurrency(m.price)}</span></button></li>)}</ul>
            </div>
          ) : null}
          <div className="border-t bg-card p-4">
            <div className="mb-3 flex items-baseline justify-between"><span className="text-sm text-muted-foreground">Total mesa</span><span className="font-mono text-2xl font-semibold tabular-nums" data-testid="table-total">{formatCurrency(total)}</span></div>
            <div className="grid grid-cols-2 gap-2">
              <Button variant="outline" size="sm" onClick={() => setMenuOpen(true)}><Plus data-icon="inline-start" /> Agregar plato</Button>
              <Button variant="outline" size="sm" onClick={() => { setOrder((o) => o.map((i) => (i.status === "en_cocina" ? i : i))); toast.success("Comanda enviada a cocina y barra"); }}><ChefHat data-icon="inline-start" /> Enviar a cocina</Button>
              <Button variant="outline" size="sm" onClick={() => toast.info("Dividir cuenta: por persona o por ítem")}><Split data-icon="inline-start" /> Dividir cuenta</Button>
              <Button variant="outline" size="sm" onClick={() => toast.success("Pre-cuenta impresa")}><Receipt data-icon="inline-start" /> Pre-cuenta</Button>
              <Button className="col-span-2" render={<Link href={`/ventas/nueva?items=${order.map(() => "c9").join(",")}`} />} nativeButton={false}><Receipt data-icon="inline-start" /> Cobrar y emitir boleta / factura</Button>
            </div>
          </div>
        </>
      ) : selected.status === "libre" ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center"><Utensils className="size-8 text-muted-foreground" strokeWidth={1.5} /><p className="text-sm text-muted-foreground">Mesa disponible</p><Button onClick={() => toast.success(`Mesa ${selected.label} abierta · mozo asignado`)}><Users data-icon="inline-start" /> Abrir mesa</Button></div>
      ) : selected.status === "reservada" ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center"><CalendarPlus className="size-8 text-muted-foreground" strokeWidth={1.5} /><p className="text-sm">{selected.note}</p><div className="flex gap-2"><Button variant="outline" size="sm">Reprogramar</Button><Button size="sm" onClick={() => toast.success("Reserva recibida · mesa abierta")}>Recibir reserva</Button></div></div>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center"><p className="text-sm text-muted-foreground">Mesa pendiente de limpieza</p><Button onClick={() => toast.success(`Mesa ${selected.label} habilitada`)}>Habilitar mesa</Button></div>
      )}
    </div>
  ) : null;

  return (
    <div className="grid gap-4 xl:grid-cols-[1fr_22rem]">
      <SectionCard
        title="Mapa del salón"
        action={<div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" render={<Link href="/salon/reservas" />} nativeButton={false}><CalendarPlus data-icon="inline-start" /> Reservas</Button><Button size="sm" variant="outline" render={<Link href="/salon/delivery" />} nativeButton={false}><Bike data-icon="inline-start" /> Delivery (4)</Button><Button size="sm" onClick={() => toast.info("Elige una mesa libre para abrirla")}><Plus data-icon="inline-start" /> Abrir mesa</Button></div>}
        contentClassName="p-0"
      >
        <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
          <Tabs value={area} onValueChange={(v) => setArea(String(v))}><TabsList>{areas.map((a) => <TabsTrigger key={a.id} value={a.id}>{a.label} <span className="ml-1 font-mono text-[11px] opacity-70">{a.count}</span></TabsTrigger>)}</TabsList></Tabs>
          <div className="flex flex-wrap gap-1.5">{(Object.keys(statusMeta) as TableStatus[]).map((s) => <Badge key={s} variant="outline" className="gap-1"><span className={cn("size-1.5 rounded-full", s === "libre" && "bg-emerald-500", s === "ocupada" && "bg-amber-500", s === "por_cobrar" && "bg-primary", s === "reservada" && "bg-violet-500", s === "sucia" && "bg-slate-400")} />{statusMeta[s].label} ({counts[s]})</Badge>)}</div>
        </div>
        <ul className="grid grid-cols-3 gap-3 p-4 sm:grid-cols-4 lg:grid-cols-6">
          {tables.filter((t) => t.area === area).map((t) => (
            <li key={t.id}>
              <button type="button" onClick={() => pick(t)} aria-pressed={selectedId === t.id} aria-label={`${t.label} ${statusMeta[t.status].label}`} className={cn("relative flex aspect-square w-full flex-col items-center rounded-xl border p-2.5 text-center text-xs transition-colors", statusMeta[t.status].card, selectedId === t.id && "border-primary ring-2 ring-primary")}>
                {selectedId === t.id ? <span className="absolute -top-2.5 left-1/2 -translate-x-1/2 rounded-md bg-primary px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-primary-foreground uppercase">Mesa activa</span> : null}
                <span className="flex w-full items-center justify-between"><span className={cn("flex size-7 items-center justify-center rounded-full text-[11px] font-bold", statusMeta[t.status].dot)}>{t.label}</span><span className="inline-flex items-center gap-0.5 text-muted-foreground"><Users className="size-3" />{t.seats}p</span></span>
                <span className="mt-auto flex w-full flex-col items-center gap-0.5">
                  {t.since ? <span className={cn("rounded px-1.5 py-0.5 font-mono text-[10px] font-semibold tabular-nums", statusMeta[t.status].chip)}>{t.since}</span> : <span className={cn("rounded px-1.5 py-0.5 text-[10px] font-semibold", statusMeta[t.status].chip)}>{t.status === "reservada" ? "Reserva" : statusMeta[t.status].label}</span>}
                  {t.amount ? <span className="font-mono text-sm font-bold tabular-nums">{formatCurrency(t.amount)}</span> : t.note ? <span className="truncate text-[11px] font-medium">{t.note.split(" · ")[0]}</span> : null}
                  {t.waiter ? <span className="truncate text-[11px] text-muted-foreground">Mozo: {t.waiter}</span> : t.status === "libre" ? <span className="text-[11px] text-primary">Disponible</span> : t.status === "sucia" ? <span className="text-[11px] font-medium text-primary">Habilitar mesa</span> : null}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </SectionCard>
      <aside aria-label="Detalle de mesa" className="hidden overflow-hidden rounded-xl border bg-card xl:block xl:min-h-[36rem]">{panel}</aside>
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}><SheetContent side="bottom" className="h-[88svh] overflow-y-auto rounded-t-xl p-0 xl:hidden"><SheetHeader className="sr-only"><SheetTitle>Detalle de mesa</SheetTitle></SheetHeader>{panel}</SheetContent></Sheet>
    </div>
  );
}
