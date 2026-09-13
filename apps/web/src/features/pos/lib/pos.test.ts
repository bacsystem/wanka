import { describe, expect, it } from "vitest";
import { addToCart, cartTotals, changeQuantity, IGV_RATE, removeFromCart } from "./pos";
import type { CartLine, CatalogItem } from "@/types/domain";

const item: CatalogItem = {
  id: "i1", sku: "SRV-0012", name: "Curación", kind: "servicio", category: "Tratamientos", price: 85, taxable: true,
};
const line = (over: Partial<CartLine> = {}): CartLine => ({
  itemId: "i1", sku: "SRV-0012", name: "Curación", unitPrice: 85, quantity: 1, discount: 0, taxable: true, ...over,
});

describe("cartTotals", () => {
  it("splits an IGV-inclusive price into base and tax", () => {
    const t = cartTotals([line()]);
    expect(IGV_RATE).toBe(0.18);
    expect(t.total).toBeCloseTo(85, 2);
    expect(t.taxableBase).toBeCloseTo(72.03, 2);
    expect(t.igv).toBeCloseTo(12.97, 2);
    expect(t.exempt).toBe(0);
  });
  it("applies quantity and line discounts", () => {
    const t = cartTotals([line({ quantity: 2, discount: 10 })]);
    expect(t.total).toBeCloseTo(160, 2);
  });
  it("keeps non-taxable lines out of the IGV base", () => {
    const t = cartTotals([line(), line({ itemId: "i2", unitPrice: 50, taxable: false })]);
    expect(t.exempt).toBeCloseTo(50, 2);
    expect(t.total).toBeCloseTo(135, 2);
    expect(t.igv).toBeCloseTo(12.97, 2);
  });
  it("counts items", () => {
    expect(cartTotals([line({ quantity: 3 }), line({ itemId: "i2" })]).itemCount).toBe(4);
  });
});

describe("cart mutations", () => {
  it("adds a new line or increments an existing one", () => {
    const once = addToCart([], item);
    expect(once).toHaveLength(1);
    const twice = addToCart(once, item);
    expect(twice).toHaveLength(1);
    expect(twice[0].quantity).toBe(2);
  });
  it("changes quantity and removes at zero", () => {
    const cart = addToCart([], item);
    expect(changeQuantity(cart, "i1", 1)[0].quantity).toBe(2);
    expect(changeQuantity(cart, "i1", -1)).toHaveLength(0);
    expect(removeFromCart(addToCart(cart, item), "i1")).toHaveLength(0);
  });
});
