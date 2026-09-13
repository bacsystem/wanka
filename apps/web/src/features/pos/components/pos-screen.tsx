"use client";

import { ScanLine, ShoppingCart, Store, Wallet } from "lucide-react";
import { Input } from "@/components/ui/input";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency, formatDocumentNumber } from "@/lib/format";
import { addToCart, cartTotals, changeQuantity, removeFromCart } from "@/features/pos/lib/pos";
import type { CartLine, CatalogItem, Customer, DocumentType, PaymentMethod } from "@/types/domain";
import { CartPanel, posDocumentTypes } from "./cart-panel";
import { CatalogGrid, CategoryChips } from "./catalog-grid";
import { CustomerPicker } from "./customer-picker";
import { FilterBar } from "@/components/shared/filter-bar";

interface PosScreenProps {
  items: CatalogItem[];
  categories: readonly string[];
  customers: Customer[];
  genericCustomer: Customer;
  /** Preselected context, e.g. coming from Agenda ("Cobrar en POS") or a customer file. */
  initialCustomerId?: string;
  initialItemIds?: string[];
}

export function PosScreen({ items, categories, customers, genericCustomer, initialCustomerId, initialItemIds = [] }: PosScreenProps) {
  const [lines, setLines] = React.useState<CartLine[]>(() => initialItemIds.reduce<CartLine[]>((acc, id) => { const it = items.find((x) => x.id === id); return it ? addToCart(acc, it) : acc; }, []));
  const [category, setCategory] = React.useState<string>(categories[0]);
  const [query, setQuery] = React.useState("");
  const [documentType, setDocumentType] = React.useState<DocumentType>("boleta");
  const [paymentMethod, setPaymentMethod] = React.useState<PaymentMethod>("efectivo");
  const [cashReceived, setCashReceived] = React.useState("");
  const [customer, setCustomer] = React.useState<Customer>(() => customers.find((c) => c.id === initialCustomerId) ?? genericCustomer);
  const [cartOpen, setCartOpen] = React.useState(false);
  const [correlative, setCorrelative] = React.useState(12904);
  const [scan, setScan] = React.useState("");
  const [lastScan, setLastScan] = React.useState<string | null>(null);
  const scanRef = React.useRef<HTMLInputElement>(null);

  function onScan(e: React.FormEvent) {
    e.preventDefault();
    const code = scan.trim().toLowerCase();
    if (!code) return;
    const hit = items.find((it) => it.sku.toLowerCase() === code || it.id === code || `77512345${it.id.replace(/\D/g, "").padStart(5, "0")}` === code);
    if (hit) { setLines((l) => addToCart(l, hit)); setLastScan(`${code} · ${hit.name}`); toast.success(`Escaneado: ${hit.name}`); }
    else { setLastScan(null); toast.error(`Código ${scan} no encontrado en el catálogo`); }
    setScan("");
  }

  const counts = React.useMemo(() => {
    const c: Record<string, number> = { [categories[0]]: items.length };
    for (const it of items) c[it.category] = (c[it.category] ?? 0) + 1;
    return c;
  }, [items, categories]);

  const visible = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter(
      (it) =>
        (category === categories[0] || it.category === category) &&
        (!q || it.name.toLowerCase().includes(q) || it.sku.toLowerCase().includes(q)),
    );
  }, [items, category, categories, query]);

  const totals = cartTotals(lines);

  function reset() {
    setLines([]);
    setCashReceived("");
    setCustomer(genericCustomer);
  }

  function emit() {
    if (documentType === "factura" && customer.documentType !== "RUC") {
      toast.error("Una factura requiere un cliente con RUC.");
      return;
    }
    const series = posDocumentTypes.find((d) => d.value === documentType)!.series;
    const number = formatDocumentNumber(series, correlative);
    setCorrelative((n) => n + 1);
    toast.success(`${documentType === "proforma" ? "Proforma" : "Comprobante"} ${number} emitido`, {
      description: `${customer.name} · ${formatCurrency(totals.total)} · enviado a SUNAT`,
    });
    reset();
    setCartOpen(false);
  }

  function saveProforma() {
    toast.info("Proforma guardada como borrador", { description: `${lines.length} líneas · ${formatCurrency(totals.total)}` });
  }

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "F10" && lines.length > 0) {
        e.preventDefault();
        emit();
      }
      if (e.key === "F2") { e.preventDefault(); scanRef.current?.focus(); }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const cartPanel = (
    <CartPanel
      lines={lines}
      documentType={documentType}
      paymentMethod={paymentMethod}
      cashReceived={cashReceived}
      onQuantity={(id, d) => setLines((l) => changeQuantity(l, id, d))}
      onRemove={(id) => setLines((l) => removeFromCart(l, id))}
      onClear={reset}
      onDocumentType={setDocumentType}
      onPaymentMethod={setPaymentMethod}
      onCashReceived={setCashReceived}
      onEmit={emit}
      onSaveProforma={saveProforma}
      customerSlot={<CustomerPicker customers={customers} value={customer} generic={genericCustomer} onChange={setCustomer} />}
    />
  );

  return (
    <div className="flex flex-1 flex-col xl:h-[calc(100svh-4rem)] xl:flex-row xl:overflow-hidden">
      {/* Catalog */}
      <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 pb-24 md:p-6 xl:overflow-y-auto xl:pb-6">
        {/* Stitch status strip: OSE, sede, caja, shortcuts */}
        <div className="-mx-4 -mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 border-b bg-card px-4 py-2 text-xs md:-mx-6 md:-mt-6 md:px-6">
          <h1 className="sr-only">Nueva venta</h1>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-accent px-2.5 py-1 text-[11px] font-semibold text-primary"><span className="size-1.5 rounded-full bg-emerald-500" />OSE / SUNAT CONECTADO</span>
          <span className="inline-flex items-center gap-1.5 text-muted-foreground"><Store className="size-3.5" /> Sede Central Miraflores</span>
          <span className="inline-flex items-center gap-1.5 text-muted-foreground"><Wallet className="size-3.5" /> Caja principal: <strong className="font-semibold text-foreground">CAJA-01 (Dra. M. Ramos)</strong></span>
          <span className="ml-auto hidden items-center gap-2 md:flex">
            <kbd className="rounded bg-muted px-1.5 py-0.5 font-sans text-[11px] text-muted-foreground"><b className="text-foreground">F2</b> Buscar</kbd>
            <kbd className="rounded bg-muted px-1.5 py-0.5 font-sans text-[11px] text-muted-foreground"><b className="text-foreground">F10</b> Cobrar</kbd>
          </span>
        </div>

        <FilterBar stack>
          <div className="flex flex-col gap-2 sm:flex-row">
            <form onSubmit={onScan} className="relative min-w-0 flex-1">
              <ScanLine className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input ref={scanRef} value={scan} onChange={(e) => setScan(e.target.value)} placeholder="Buscar por tratamiento, medicamento, insumo o SKU… (Presione / o F2)" aria-label="Escanear código" className="bg-muted/50 pr-12 pl-9 text-sm" />
              <kbd className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 rounded border bg-card px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">F2</kbd>
            </form>
            <SearchInput placeholder="Filtrar por nombre" value={query} onChange={setQuery} className="sm:max-w-56" />
          </div>
          {lastScan ? <p className="text-xs text-success">Último escaneo: {lastScan}</p> : null}
          <CategoryChips categories={categories} counts={counts} value={category} onChange={setCategory} />
        </FilterBar>

        <CatalogGrid items={visible} onAdd={(it) => setLines((l) => addToCart(l, it))} />
        <p className="text-xs text-muted-foreground">
          Mostrando {visible.length} de {items.length} productos/servicios
        </p>
      </div>

      {/* Cart: sidebar on xl, bottom bar + sheet below */}
      <aside className="hidden w-[27rem] shrink-0 overflow-y-auto border-l bg-card xl:block 2xl:w-[30rem]" aria-label="Carrito">
        {cartPanel}
      </aside>

      <div className="fixed inset-x-0 bottom-16 z-30 border-t bg-background/95 p-3 backdrop-blur md:bottom-0 xl:hidden">
        <Sheet open={cartOpen} onOpenChange={setCartOpen}>
          <SheetTrigger
            render={<Button size="lg" className="w-full justify-between text-base" />}
          >
            <span className="flex items-center gap-2">
              <ShoppingCart className="size-5" />
              Ver carrito
              <span className="rounded-full bg-primary-foreground/20 px-2 py-0.5 font-mono text-xs font-semibold text-primary-foreground">{totals.itemCount}</span>
            </span>
            <span className="font-mono tabular-nums" data-testid="cart-total-mobile">{formatCurrency(totals.total)}</span>
          </SheetTrigger>
          <SheetContent side="bottom" className="h-[92svh] overflow-y-auto rounded-t-xl p-0">
            <SheetHeader className="sr-only">
              <SheetTitle>Carrito</SheetTitle>
            </SheetHeader>
            {cartPanel}
          </SheetContent>
        </Sheet>
      </div>
    </div>
  );
}
