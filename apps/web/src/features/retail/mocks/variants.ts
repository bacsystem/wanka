export const sizes = ["S", "M", "L", "XL"] as const;
export const colors = [{ id: "B", name: "Blanco puro", code: "COL-BLA", hex: "#f5f5f4" }, { id: "N", name: "Negro humo", code: "COL-NEG", hex: "#1f2937" }, { id: "A", name: "Azul marino", code: "COL-AZU", hex: "#1e3a8a" }] as const;
export interface Variant { id: string; sku: string; color: string; size: string; ean: string; price: number; stockMain: number; stockShop: number }

export const productMock = { sku: "POL-001", name: "Polo básico cuello redondo 100% algodón pima", category: "Confección y moda", affectation: "10 Gravado IGV (18%)", avgCost: 18.5, replacementCost: 19, lastPurchase: "Factura F001-2849 (Textil San Cristóbal)", totalStock: 184, stockMain: 142, stockShop: 42 };

const base: Record<string, { price: number; stock: [number, number][] }> = {
  B: { price: 38, stock: [[10, 4], [12, 6], [5, 3], [2, 1]] },
  N: { price: 38, stock: [[16, 6], [24, 10], [14, 5], [6, 3]] },
  A: { price: 38, stock: [[11, 4], [20, 6], [12, 5], [4, 3]] },
};
export const variantsMock: Variant[] = colors.flatMap((c, ci) => sizes.map((s, si) => ({ id: `${c.id}-${s}`, sku: `POL-001-${c.id}-${s}`, color: c.id, size: s, ean: `77512345670${ci + 1}${si + 1}`, price: s === "XL" ? base[c.id].price + 2 : base[c.id].price, stockMain: base[c.id].stock[si][0], stockShop: base[c.id].stock[si][1] })));

export const priceLists = [
  { name: "Minorista", price: 38, margin: 51.3, note: "Precio de mostrador" },
  { name: "Mayorista (desde 12 und)", price: 32, margin: 42.2, note: "Descuento automático por cantidad" },
  { name: "Distribuidor", price: 27, margin: 31.5, note: "Requiere RUC y factura" },
  { name: "Promoción web", price: 34.9, margin: 47, note: "Vigente hasta 30/09/2026" },
];
export const costHistory = [{ date: "2026-09-02", cost: 19, doc: "F001-2849 · Textil San Cristóbal", qty: 120 }, { date: "2026-07-15", cost: 18.2, doc: "F001-2711 · Textil San Cristóbal", qty: 100 }, { date: "2026-05-10", cost: 17.8, doc: "F001-2530 · Textil San Cristóbal", qty: 150 }];
