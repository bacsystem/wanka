"use client";

import { ArrowRight, Banknote, CalendarPlus, CornerDownLeft, FileText, Package, PlusCircle, Receipt, Search, Settings, ShieldCheck, Truck, Users, Warehouse, BarChart3 } from "lucide-react";
import { useRouter } from "next/navigation";
import * as React from "react";
import { StatusBadge } from "@/components/shared/status-badge";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { customersMock } from "@/features/customers/mocks/customers";
import { documentsMock } from "@/features/documents/mocks/documents";
import { catalogMock } from "@/features/pos/mocks/catalog";
import { quotesMock } from "@/features/quotes/mocks/quotes";
import { formatCurrency, formatDocumentNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

interface Item { id: string; group: string; title: string; subtitle?: string; badge?: string; href: string; icon: typeof Search; kbd?: string }

const actions: Item[] = [
  { id: "a1", group: "Acciones rápidas", title: "Nueva venta POS / facturación rápida", subtitle: "Boleta o factura directa con IGV 18%", href: "/ventas/nueva", icon: PlusCircle, kbd: "F1" },
  { id: "a2", group: "Acciones rápidas", title: "Emitir guía de remisión electrónica (GRE)", subtitle: "Traslado entre sedes", href: "/ventas/guias", icon: Truck, kbd: "F3" },
  { id: "a3", group: "Acciones rápidas", title: "Registrar cobro / abono de cliente", subtitle: "Amortización a cuenta corriente", href: "/finanzas/cuentas-por-cobrar", icon: Banknote },
  { id: "a4", group: "Acciones rápidas", title: "Agendar nueva cita", subtitle: "Asignación de sillón y profesional", href: "/agenda", icon: CalendarPlus },
];
const nav: Item[] = [
  { id: "n1", group: "Ir a…", title: "Kardex físico y valorizado SUNAT 13.1", href: "/inventario/kardex", icon: Warehouse },
  { id: "n2", group: "Ir a…", title: "Reportes tributarios RVIE / RCE SIRE", href: "/reportes", icon: BarChart3 },
  { id: "n3", group: "Ir a…", title: "Configuración de certificado digital y Clave SOL", href: "/configuracion/sunat", icon: ShieldCheck },
  { id: "n4", group: "Ir a…", title: "Usuarios, roles y permisos", href: "/configuracion/usuarios", icon: Settings },
];

function buildItems(q: string): Item[] {
  const s = q.trim().toLowerCase();
  const match = (...vals: (string | undefined)[]) => !s || vals.some((v) => v?.toLowerCase().includes(s));
  const docs: Item[] = documentsMock.filter((d) => match(d.id, d.customerName, d.customerDocument)).slice(0, 3).map((d) => ({ id: d.id, group: "Comprobantes", title: `${d.type === "factura" ? "Factura" : d.type === "boleta" ? "Boleta" : "Nota de crédito"} ${formatDocumentNumber(d.series, d.number)}`, subtitle: `${d.customerName} · ${formatCurrency(d.total)}`, badge: d.sunatStatus === "aceptado" ? "Aceptado" : d.sunatStatus === "pendiente" ? "Pendiente" : "Rechazado", href: `/ventas/comprobantes/${d.id}`, icon: Receipt }));
  const quotes: Item[] = quotesMock.filter((x) => match(x.code, x.customerName)).slice(0, 2).map((x) => ({ id: x.code, group: "Comprobantes", title: `Cotización ${x.code}`, subtitle: `${x.customerName} · ${formatCurrency(x.total)}`, href: `/ventas/proformas/${x.code}`, icon: FileText }));
  const customers: Item[] = customersMock.filter((c) => match(c.name, c.documentNumber, c.clinicalRecord)).slice(0, 3).map((c) => ({ id: c.id, group: "Clientes y pacientes", title: c.name, subtitle: `${c.documentType} ${c.documentNumber}${c.clinicalRecord ? ` · ${c.clinicalRecord}` : ""} · ${c.segment ?? ""}`, href: `/clientes/${c.id}`, icon: Users }));
  const products: Item[] = catalogMock.filter((p) => match(p.name, p.sku)).slice(0, 3).map((p) => ({ id: p.id, group: "Productos e insumos", title: `${p.name} (${p.sku})`, subtitle: p.stock !== undefined ? `Stock ${p.stock} ${p.unit} · ${formatCurrency(p.price)}` : formatCurrency(p.price), badge: p.stock !== undefined && p.stock <= 5 ? "Crítico" : undefined, href: "/inventario/stock", icon: Package }));
  return [...actions.filter((a) => match(a.title, a.subtitle)), ...docs, ...quotes, ...customers, ...products, ...nav.filter((n) => match(n.title))];
}

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? <PaletteBody onClose={() => onOpenChange(false)} /> : null}
    </Dialog>
  );
}

/** Mounted only while open, so query/active state resets naturally on each opening. */
function PaletteBody({ onClose }: { onClose: () => void }) {
  const router = useRouter();
  const [q, setQState] = React.useState("");
  const [active, setActive] = React.useState(0);
  const setQ = (v: string) => { setQState(v); setActive(0); };
  const items = React.useMemo(() => buildItems(q), [q]);
  const groups = Array.from(new Set(items.map((i) => i.group)));

  function go(item: Item) { onClose(); router.push(item.href); }
  function onKey(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => Math.min(items.length - 1, a + 1)); }
    if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(0, a - 1)); }
    if (e.key === "Enter" && items[active]) { e.preventDefault(); go(items[active]); }
  }
  let index = -1;
  return (
      <DialogContent className="top-[10%] translate-y-0 gap-0 p-0 sm:max-w-2xl" onKeyDown={onKey}>
        <DialogTitle className="sr-only">Búsqueda global</DialogTitle>
        <DialogDescription className="sr-only">Busca comprobantes, clientes, productos o acciones.</DialogDescription>
        <div className="flex items-center gap-2 border-b px-3"><Search className="size-4 text-muted-foreground" /><input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder="Buscar comprobante, cliente, producto o acción…" aria-label="Buscar" className="h-12 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground" /><kbd className="rounded border px-1.5 font-mono text-[10px] text-muted-foreground">ESC</kbd></div>
        <div className="max-h-[60svh] overflow-y-auto p-2" role="listbox" aria-label="Resultados">
          {items.length === 0 ? <p className="px-3 py-8 text-center text-sm text-muted-foreground">Sin resultados para “{q}”.</p> : null}
          {groups.map((g) => (
            <div key={g} className="mb-2">
              <p className="px-2 py-1 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">{g}</p>
              {items.filter((i) => i.group === g).map((it) => { index += 1; const i = index; return (
                <button key={it.id} type="button" role="option" aria-selected={active === i} onMouseEnter={() => setActive(i)} onClick={() => go(it)} className={cn("flex w-full items-center gap-3 rounded-md px-2 py-2 text-left text-sm", active === i && "bg-accent text-accent-foreground")}>
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted"><it.icon className="size-4" strokeWidth={1.5} /></span>
                  <span className="min-w-0 flex-1"><span className="block truncate font-medium">{it.title}</span>{it.subtitle ? <span className="block truncate text-xs text-muted-foreground">{it.subtitle}</span> : null}</span>
                  {it.badge ? <StatusBadge tone={it.badge === "Crítico" || it.badge === "Rechazado" ? "danger" : it.badge === "Aceptado" ? "success" : "neutral"} label={it.badge} /> : null}
                  {it.kbd ? <kbd className="rounded border px-1.5 font-mono text-[10px] text-muted-foreground">{it.kbd}</kbd> : <ArrowRight className="size-3.5 text-muted-foreground" />}
                </button>
              ); })}
            </div>
          ))}
        </div>
        <div className="flex items-center gap-4 border-t px-3 py-2 text-[11px] text-muted-foreground"><span><kbd className="rounded border px-1 font-mono">↑↓</kbd> navegar</span><span><kbd className="rounded border px-1 font-mono"><CornerDownLeft className="inline size-3" /></kbd> seleccionar</span><span><kbd className="rounded border px-1 font-mono">ESC</kbd> cerrar</span><span className="ml-auto inline-flex items-center gap-1 text-success"><span className="size-1.5 rounded-full bg-success" /> SUNAT OSE conectado</span></div>
      </DialogContent>
  );
}

/** Wires Ctrl/Cmd+K and exposes a trigger to the topbar search. */
export function useCommandPalette() {
  const [open, setOpen] = React.useState(false);
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setOpen((o) => !o); } }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  return { open, setOpen };
}
