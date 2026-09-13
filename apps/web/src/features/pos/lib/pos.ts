import type { CartLine, CatalogItem } from "@/types/domain";

export const IGV_RATE = 0.18;

export interface CartTotals {
  itemCount: number;
  taxableBase: number;
  exempt: number;
  igv: number;
  total: number;
}

function round2(n: number) {
  return Math.round(n * 100) / 100;
}

export function lineTotal(line: CartLine) {
  return Math.max(0, line.unitPrice * line.quantity - line.discount);
}

/** Prices are IGV-inclusive (retail convention in Peru); totals split base and tax for the comprobante. */
export function cartTotals(lines: CartLine[]): CartTotals {
  let taxableGross = 0;
  let exempt = 0;
  let itemCount = 0;
  for (const l of lines) {
    itemCount += l.quantity;
    if (l.taxable) taxableGross += lineTotal(l);
    else exempt += lineTotal(l);
  }
  const taxableBase = taxableGross / (1 + IGV_RATE);
  const igv = taxableGross - taxableBase;
  return {
    itemCount,
    taxableBase: round2(taxableBase),
    exempt: round2(exempt),
    igv: round2(igv),
    total: round2(taxableGross + exempt),
  };
}

export function addToCart(lines: CartLine[], item: CatalogItem): CartLine[] {
  const existing = lines.find((l) => l.itemId === item.id);
  if (existing) return lines.map((l) => (l.itemId === item.id ? { ...l, quantity: l.quantity + 1 } : l));
  return [
    ...lines,
    { itemId: item.id, sku: item.sku, name: item.name, unitPrice: item.price, quantity: 1, discount: 0, taxable: item.taxable },
  ];
}

export function changeQuantity(lines: CartLine[], itemId: string, delta: number): CartLine[] {
  return lines
    .map((l) => (l.itemId === itemId ? { ...l, quantity: l.quantity + delta } : l))
    .filter((l) => l.quantity > 0);
}

export function removeFromCart(lines: CartLine[], itemId: string): CartLine[] {
  return lines.filter((l) => l.itemId !== itemId);
}
