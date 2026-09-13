import { describe, expect, it } from "vitest";
import { isActivePath } from "./navigation";

describe("isActivePath", () => {
  it("matches home only exactly", () => {
    expect(isActivePath("/", "/")).toBe(true);
    expect(isActivePath("/ventas", "/")).toBe(false);
  });
  it("matches a section and its children", () => {
    expect(isActivePath("/ventas", "/ventas")).toBe(true);
    expect(isActivePath("/ventas/comprobantes", "/ventas")).toBe(true);
    expect(isActivePath("/ventasx", "/ventas")).toBe(false);
  });
});
