"use client";

import { Bike, Check, Clock, LayoutGrid, MapPin, Phone, Printer, Receipt, UserCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useMediaQuery } from "@/hooks/use-media-query";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DeliveryLane, DeliveryOrder, Rider } from "../mocks/delivery";

const lanes: { id: DeliveryLane; label: string; tone: BadgeTone; dot: string; border: string }[] = [
  { id: "nuevo", label: "Nuevo", tone: "info", dot: "bg-blue-500", border: "border-l-blue-500" },
  { id: "cocina", label: "En cocina", tone: "warning", dot: "bg-amber-500", border: "border-l-amber-500" },
  { id: "listo", label: "Listo para despacho", tone: "success", dot: "bg-emerald-500", border: "border-l-emerald-500" },
  { id: "ruta", label: "En ruta", tone: "info", dot: "bg-primary", border: "border-l-primary" },
  { id: "entregado", label: "Entregado", tone: "neutral", dot: "bg-slate-400", border: "border-l-slate-400" },
];
const channelTone: Record<DeliveryOrder["channel"], string> = { WhatsApp: "bg-success/10 text-success", Rappi: "bg-orange-500/10 text-orange-600", PedidosYa: "bg-red-500/10 text-red-600", Web: "bg-accent text-accent-foreground" };

export function DeliveryScreen({ orders: initial, riders }: { orders: DeliveryOrder[]; riders: Rider[] }) {
  const [orders, setOrders] = React.useState(initial);
  const [selectedId, setSelectedId] = React.useState(initial[0]?.id ?? null);
  const [laneTab, setLaneTab] = React.useState<DeliveryLane>("nuevo");
  const [sheet, setSheet] = React.useState(false);
  const belowXl = useMediaQuery("(max-width: 1279px)");
  const belowLg = useMediaQuery("(max-width: 1023px)");
  const sel = orders.find((o) => o.id === selectedId) ?? null;
  const advance = (id: string) => setOrders((os) => os.map((o) => { if (o.id !== id) return o; const i = lanes.findIndex((l) => l.id === o.lane); return { ...o, lane: lanes[Math.min(lanes.length - 1, i + 1)].id }; }));
  const assign = (id: string, rider: string) => setOrders((os) => os.map((o) => (o.id === id ? { ...o, rider } : o)));
  const pick = (o: DeliveryOrder) => { setSelectedId(o.id); if (belowXl) setSheet(true); };

  const card = (o: DeliveryOrder) => (
    <button key={o.id} type="button" onClick={() => pick(o)} aria-pressed={selectedId === o.id} className={cn("flex w-full flex-col gap-2 rounded-lg border border-l-4 bg-card p-3 text-left text-sm shadow-xs hover:border-primary/50", lanes.find((l) => l.id === o.lane)?.border, selectedId === o.id && "ring-2 ring-primary")}>
      <div className="flex items-center justify-between gap-2"><span className="font-mono text-xs font-bold text-primary">#{o.code}</span><Tag label={o.channel} className={cn("border-transparent", channelTone[o.channel])} /></div>
      <div><p className="truncate font-bold">{o.customer}</p><p className="font-mono text-xs text-muted-foreground">{o.phone}</p></div>
      <p className="truncate rounded-md bg-muted/50 px-2 py-1.5 text-xs text-muted-foreground"><MapPin className="mr-1 inline size-3" />{o.address}</p>
      <ul className="flex flex-col gap-0.5 text-xs text-muted-foreground">{o.items.slice(0, 2).map((i) => <li key={i.name} className="flex justify-between"><span>{i.qty}x {i.name}</span></li>)}</ul>
      <div className="flex items-center justify-between border-t pt-2 text-xs"><span className="font-mono text-sm font-bold">{formatCurrency(o.total)}</span><Badge variant="outline" className="rounded-md text-[10px]">{o.payment}</Badge></div>
      <div className="flex items-center justify-between text-xs"><span className={cn("flex items-center gap-1 font-medium", o.minutes >= 30 && o.lane !== "entregado" ? "text-destructive" : "text-primary")}><Clock className="size-3" /> {o.minutes} min</span>{o.rider ? <span className="flex items-center gap-1 text-muted-foreground"><Bike className="size-3" /> {o.rider}</span> : <span className="text-destructive">Sin repartidor</span>}</div>
    </button>
  );

  const panel = sel ? (
    <div className="flex flex-col gap-3 p-4 text-sm">
      <div className="flex items-start justify-between gap-2"><div><p className="font-mono text-xs font-semibold text-primary">{sel.code}</p><h2 className="text-lg font-semibold">{sel.customer}</h2><p className="text-xs text-muted-foreground">{sel.phone}</p></div><StatusBadge tone={lanes.find((l) => l.id === sel.lane)!.tone} dot label={lanes.find((l) => l.id === sel.lane)!.label} /></div>
      <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Dirección</p><p className="font-medium">{sel.address}</p><p className="text-xs text-muted-foreground">Ref.: {sel.reference}</p></div>
      <ul className="divide-y rounded-md border">{sel.items.map((i) => <li key={i.name} className="flex justify-between px-3 py-2"><span><span className="font-mono font-semibold">{i.qty}×</span> {i.name}</span></li>)}<li className="flex justify-between px-3 py-2 font-semibold"><span>Total</span><span className="font-mono">{formatCurrency(sel.total)}</span></li></ul>
      <div className="grid grid-cols-2 gap-2 text-xs"><div className="rounded-md bg-muted/40 p-2"><p className="text-muted-foreground">Canal</p><p className="font-medium">{sel.channel}</p></div><div className="rounded-md bg-muted/40 p-2"><p className="text-muted-foreground">Pago</p><p className="font-medium">{sel.payment}</p></div></div>
      <div><p className="mb-1 text-xs text-muted-foreground">Repartidor</p><Select value={sel.rider ?? "none"} onValueChange={(v) => { assign(sel.id, String(v)); toast.success(`${v} asignado a ${sel.code}`); }} items={[{ value: "none", label: "Sin asignar" }, ...riders.map((r) => ({ value: r.name.split(" ")[0], label: `${r.name} · ${r.status === "disponible" ? "disponible" : r.status === "en_ruta" ? "en ruta" : "descanso"}` }))]}><SelectTrigger className="w-full" aria-label="Repartidor"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="none">Sin asignar</SelectItem>{riders.map((r) => <SelectItem key={r.id} value={r.name.split(" ")[0]}>{r.name} · {r.status === "disponible" ? "disponible" : r.status === "en_ruta" ? "en ruta" : "descanso"}</SelectItem>)}</SelectContent></Select></div>
      <div className="grid grid-cols-2 gap-2">
        <Button variant="outline" size="sm"><Printer data-icon="inline-start" /> Comanda</Button>
        <Button variant="outline" size="sm" onClick={() => toast.info(`Llamando a ${sel.phone}`)}><Phone data-icon="inline-start" /> Llamar</Button>
        {sel.lane !== "entregado" ? <Button className="col-span-2" onClick={() => { advance(sel.id); toast.success(`${sel.code} → ${lanes[Math.min(4, lanes.findIndex((l) => l.id === sel.lane) + 1)].label}`); }}><Check data-icon="inline-start" /> {sel.lane === "ruta" ? "Marcar entregado" : sel.lane === "listo" ? "Despachar" : "Avanzar estado"}</Button> : <Button className="col-span-2" render={<Link href="/ventas/nueva" />} nativeButton={false}><Receipt data-icon="inline-start" /> Emitir boleta</Button>}
      </div>
    </div>
  ) : null;

  return (
    <div className="grid gap-4 xl:grid-cols-[1fr_20rem]">
      <div className="flex min-w-0 flex-col gap-4">
        {belowLg ? (
          <SectionCard title="Pedidos" contentClassName="p-3">
            <Tabs value={laneTab} onValueChange={(v) => setLaneTab(v as DeliveryLane)}><TabsList>{lanes.map((l) => <TabsTrigger key={l.id} value={l.id}>{l.label} <span className="ml-1 font-mono text-[11px] opacity-70">{orders.filter((o) => o.lane === l.id).length}</span></TabsTrigger>)}</TabsList></Tabs>
            <div className="mt-3 flex flex-col gap-2">{orders.filter((o) => o.lane === laneTab).map(card)}{orders.filter((o) => o.lane === laneTab).length === 0 ? <p className="rounded-md border border-dashed p-6 text-center text-xs text-muted-foreground">Sin pedidos</p> : null}</div>
          </SectionCard>
        ) : (
          <SectionCard title={<span className="flex items-center gap-2">Tablero de despacho de pedidos <span className="text-xs font-normal text-muted-foreground">· Flujo en tiempo real</span></span>} icon={LayoutGrid} action={<span className="flex items-center gap-1 text-xs"><span className="mr-1 text-muted-foreground">Canal:</span>{["Todos", "WhatsApp", "Rappi", "PedidosYa"].map((c, i) => <span key={c} className={cn("rounded-full px-2.5 py-0.5 font-medium", i === 0 ? "bg-foreground text-background" : "bg-muted text-muted-foreground")}>{c}</span>)}</span>} contentClassName="p-3">
            <div className="no-scrollbar flex gap-3 overflow-x-auto pb-1">{lanes.map((l) => { const rows = orders.filter((o) => o.lane === l.id); return <section key={l.id} aria-label={l.label} className="flex w-64 shrink-0 flex-col gap-2 rounded-xl bg-muted/40 p-2"><header className="flex items-center justify-between px-1 py-1"><h3 className="flex items-center gap-2 text-sm font-bold"><span className={cn("size-2 rounded-full", l.dot)} />{l.label}</h3><span className="rounded bg-card px-1.5 py-0.5 font-mono text-[11px] font-semibold shadow-xs">{rows.length}</span></header>{rows.map(card)}{rows.length === 0 ? <p className="rounded-md border border-dashed p-4 text-center text-xs text-muted-foreground">—</p> : null}</section>; })}</div>
          </SectionCard>
        )}
        <SectionCard title="Repartidores" contentClassName="p-0"><ul className="divide-y text-sm">{riders.map((r) => <li key={r.id} className="flex items-center gap-3 px-4 py-2.5"><span className="flex size-8 items-center justify-center rounded-full bg-accent"><Bike className="size-4" strokeWidth={1.5} /></span><div className="min-w-0 flex-1"><p className="font-medium">{r.name}</p><p className="text-xs text-muted-foreground">{r.vehicle} · {r.orders.length ? r.orders.join(", ") : "sin pedidos"}</p></div><StatusBadge tone={r.status === "en_ruta" ? "info" : r.status === "disponible" ? "success" : "neutral"} dot label={r.status === "en_ruta" ? "En ruta" : r.status === "disponible" ? "Disponible" : "Descanso"} /></li>)}</ul></SectionCard>
      </div>
      <aside aria-label="Detalle del pedido" className="hidden rounded-xl border bg-card xl:block">{panel ?? <div className="p-6 text-center text-sm text-muted-foreground"><UserCheck className="mx-auto mb-2 size-6" /> Selecciona un pedido</div>}</aside>
      <Sheet open={sheet} onOpenChange={setSheet}><SheetContent side="bottom" className="max-h-[88svh] overflow-y-auto rounded-t-xl p-0 xl:hidden"><SheetHeader className="sr-only"><SheetTitle>Detalle del pedido</SheetTitle></SheetHeader>{panel}</SheetContent></Sheet>
    </div>
  );
}
