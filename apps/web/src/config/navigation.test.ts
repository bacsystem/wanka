import { describe, expect, it } from "vitest";
import { isActivePath, navigation } from "./navigation";
import { groupLandings } from "./redirects";

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

describe("groupLandings", () => {
  const groups = navigation.filter((n) => n.children?.length);
  // Groups that render a page at their own href (no redirect needed): /compras and /reportes list
  // themselves as first child; /catalogo is served by the optional catch-all route.
  const selfServed = ["/catalogo"];
  it("gives every group without its own page a landing module", () => {
    const needing = groups.filter((g) => !selfServed.includes(g.href) && !g.children!.some((c) => c.href === g.href)).map((g) => g.href);
    expect(Object.keys(groupLandings).sort()).toEqual(needing.sort());
  });
  it("points each group to one of its own children", () => {
    for (const [group, landing] of Object.entries(groupLandings)) {
      const g = groups.find((x) => x.href === group);
      expect(g, `${group} is a navigation group`).toBeDefined();
      expect(g!.children!.map((c) => c.href)).toContain(landing);
    }
  });
});
