"use client";

import { ArrowLeftRight, Banknote, CheckCircle2, CreditCard, FileText, Minus, Plus, QrCode, ShoppingBasket, Trash2, UserRound, X, Zap } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { Input } from "@/components/ui/input";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { formatCurrency } from "@/lib/format";
import { cartTotals, lineTotal } from "@/features/pos/lib/pos";
import { cn } from "@/lib/utils";
import type { CartLine, DocumentType, PaymentMethod } from "@/types/domain";

export const posDocumentTypes: { value: DocumentType; label: string; series: string }[] = [
  { value: "boleta", label: "Boleta", series: "B001" },
  { value: "factura", label: "Factura", series: "F001" },
  { value: "proforma", label: "Proforma", series: "P001" },
];

export const paymentMethods: { value: PaymentMethod; label: string; icon: typeof Banknote }[] = [
  { value: "efectivo", label: "Efectivo", icon: Banknote },
  { value: "tarjeta", label: "Tarjeta", icon: CreditCard },
  { value: "yape_plin", label: "Yape / Plin", icon: QrCode },
  { value: "transferencia", label: "Transferencia", icon: ArrowLeftRight },
];

interface CartPanelProps {
  lines: CartLine[];
  documentType: DocumentType;
  paymentMethod: PaymentMethod;
  cashReceived: string;
  onQuantity: (itemId: string, delta: number) => void;
  onRemove: (itemId: string) => void;
  onClear: () => void;
  onDocumentType: (t: DocumentType) => void;
  onPaymentMethod: (m: PaymentMethod) => void;
  onCashReceived: (v: string) => void;
  onEmit: () => void;
  onSaveProforma: () => void;
  customerSlot: React.ReactNode;
}

export function CartPanel(p: CartPanelProps) {
  const totals = cartTotals(p.lines);
  const cash = Number(p.cashReceived) || 0;
  const change = Math.max(0, cash - totals.total);
  const series = posDocumentTypes.find((d) => d.value === p.documentType)?.series;

  return (
    <div className="flex min-h-full flex-col">
      <div className="flex items-center justify-between gap-2 border-b px-4 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground"><ShoppingBasket className="size-4" /></span>
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-base font-bold"><span className="truncate">Orden #OV-2026-0892</span><Tag tone="neutral" solid label="Abierta" /></p>
            <p className="text-xs text-muted-foreground">Emisión inmediata | POS-01 · {totals.itemCount} ítem{totals.itemCount === 1 ? "" : "s"}</p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={p.onClear} disabled={p.lines.length === 0}>
          <Trash2 data-icon="inline-start" /> Limpiar
        </Button>
      </div>

      <div className="flex min-w-0 flex-col gap-3 p-4">
        <section aria-label="Cliente">
          <p className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold"><UserRound className="size-3.5 text-muted-foreground" /> Datos del cliente / paciente</p>
          {p.customerSlot}
        </section>

        <section aria-label="Tipo de comprobante">
          <div className="mb-1.5 flex items-center justify-between text-xs"><p className="font-semibold tracking-wide uppercase">Comprobante SUNAT:</p><span className="font-semibold text-primary">Serie: {series} - Autonumérico</span></div>
          <ToggleGroup
            value={[p.documentType]}
            onValueChange={(v) => v[0] && p.onDocumentType(v[0] as DocumentType)}
            className="w-full rounded-lg bg-accent p-1 dark:bg-muted"
          >
            {posDocumentTypes.map((d) => (
              <ToggleGroupItem key={d.value} value={d.value} className="h-8 flex-1 rounded-md border-transparent text-xs font-medium hover:bg-transparent aria-pressed:border-transparent aria-pressed:bg-card aria-pressed:font-semibold aria-pressed:text-primary aria-pressed:shadow-xs data-pressed:border-transparent data-pressed:bg-card data-pressed:text-primary" aria-label={d.label}>
                {d.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </section>
      </div>

      <ul className="flex flex-1 flex-col gap-2 px-4 pb-3" aria-label="Líneas del carrito">
        {p.lines.length === 0 ? (
          <li className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
            Agrega productos o servicios desde el catálogo.
          </li>
        ) : (
          p.lines.map((l) => (
            <li key={l.itemId} className="flex items-center gap-3 rounded-lg bg-muted/60 p-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">{l.name}</p>
                <p className="text-xs text-muted-foreground">
                  Unit: {formatCurrency(l.unitPrice)} <span className="mx-1">•</span> <span className="font-medium text-primary">{l.taxable ? "Gravada" : "Inafecta"}</span>
                </p>
              </div>
              <div className="flex items-center rounded-md border bg-card">
                <Button variant="ghost" size="icon-xs" aria-label={`Quitar uno de ${l.name}`} onClick={() => p.onQuantity(l.itemId, -1)}>
                  <Minus />
                </Button>
                <span className="w-6 text-center font-mono text-sm font-semibold tabular-nums" aria-live="polite">{l.quantity}</span>
                <Button variant="ghost" size="icon-xs" aria-label={`Agregar uno de ${l.name}`} onClick={() => p.onQuantity(l.itemId, 1)}>
                  <Plus />
                </Button>
              </div>
              <span className="w-20 text-right font-mono text-sm font-bold tabular-nums">{formatCurrency(lineTotal(l))}</span>
              <Button variant="ghost" size="icon-xs" aria-label={`Eliminar ${l.name}`} className="text-muted-foreground" onClick={() => p.onRemove(l.itemId)}>
                <X />
              </Button>
            </li>
          ))
        )}
      </ul>

      <div className="sticky bottom-0 border-t bg-card p-4 shadow-[0_-8px_16px_-12px_rgb(0_0_0/0.25)]">
        <section aria-label="Medio de pago" className="mb-3">
          <p className="mb-1.5 text-xs font-semibold tracking-wide uppercase">Medio de pago principal:</p>
          <div className="grid grid-cols-4 gap-1.5">
            {paymentMethods.map((m) => (
              <button
                key={m.value}
                type="button"
                onClick={() => p.onPaymentMethod(m.value)}
                aria-pressed={p.paymentMethod === m.value}
                className={cn(
                  "flex flex-col items-center gap-1.5 rounded-lg bg-accent px-1 py-3 text-[11px] font-semibold text-foreground/80 transition-colors hover:bg-accent/70 dark:bg-muted",
                  p.paymentMethod === m.value && "bg-card text-primary shadow-sm ring-1 ring-border dark:bg-card",
                )}
              >
                <m.icon className="size-4" strokeWidth={2} aria-hidden />
                {m.label}
              </button>
            ))}
          </div>
          {p.paymentMethod === "efectivo" ? (
            <div className="mt-2 flex items-center justify-between gap-3 rounded-lg bg-muted/60 p-3">
              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                Paga con S/:
                <Input
                  inputMode="decimal"
                  value={p.cashReceived}
                  onChange={(e) => p.onCashReceived(e.target.value)}
                  className="w-24 bg-card font-mono text-sm font-semibold tabular-nums"
                  placeholder="0.00"
                />
              </label>
              <div className="text-right">
                <p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Vuelto:</p>
                <p className="font-mono text-base font-bold text-primary tabular-nums">{formatCurrency(change)}</p>
              </div>
            </div>
          ) : null}
        </section>

        <dl className="flex flex-col gap-1 text-sm">
          <div className="flex justify-between text-muted-foreground">
            <dt>Op. Gravada (Base imponible)</dt>
            <dd className="font-mono tabular-nums">{formatCurrency(totals.taxableBase)}</dd>
          </div>
          <div className="flex justify-between text-muted-foreground">
            <dt>Op. Inafecta / Exonerada</dt>
            <dd className="font-mono tabular-nums">{formatCurrency(totals.exempt)}</dd>
          </div>
          <div className="flex justify-between text-muted-foreground">
            <dt>I.G.V. Oficial (18%)</dt>
            <dd className="font-mono tabular-nums">{formatCurrency(totals.igv)}</dd>
          </div>
          <div className="flex justify-between text-muted-foreground">
            <dt>Descuento aplicado</dt>
            <dd className="font-mono text-primary tabular-nums">{formatCurrency(0)}</dd>
          </div>
        </dl>
        <div className="mt-3 flex items-center justify-between gap-3 rounded-lg bg-muted/60 p-3">
          <div>
            <p className="text-[11px] font-bold tracking-wide uppercase">Total a liquidar</p>
            <p className="text-[11px] font-medium text-primary">Inc. 18% I.G.V.</p>
          </div>
          <span className="font-mono text-3xl font-bold text-primary tabular-nums" data-testid="cart-total">{formatCurrency(totals.total)}</span>
        </div>

        <div className="mt-3 flex flex-col gap-2">
          <Button size="lg" className="w-full text-base font-semibold" onClick={p.onEmit} disabled={p.lines.length === 0}>
            <CheckCircle2 data-icon="inline-start" />
            Emitir comprobante
            <kbd className="ml-auto rounded bg-primary-foreground/15 px-1.5 font-mono text-[10px]">F10</kbd>
          </Button>
          <div className="grid grid-cols-2 gap-2">
            <Button variant="secondary" className="bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted" onClick={p.onSaveProforma} disabled={p.lines.length === 0}>
              <FileText data-icon="inline-start" /> Guardar proforma
            </Button>
            <Button variant="secondary" className="bg-teal-100 font-semibold text-teal-900 hover:bg-teal-200 dark:bg-teal-900/50 dark:text-teal-100" onClick={() => p.onCashReceived(String(totals.total))} disabled={p.lines.length === 0}>
              <Zap data-icon="inline-start" /> Cobro rápido exacto
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
