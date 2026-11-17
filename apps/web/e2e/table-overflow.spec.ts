import { expect, test } from "./fixtures";

/**
 * The DS `ui/table` cells are wider than the old local ones (px-3, nowrap). Line tables of a document must
 * show every column (notably the totals) without horizontal scrolling on desktop and tablet.
 * Measures the table's scroll container: scrollWidth - clientWidth must be 0.
 */
const cases = [
  { name: "comprobante detail lines", path: "/ventas/comprobantes/F001-00000842", header: "Total" },
  { name: "purchase order lines", path: "/compras/ordenes/nueva", header: "Costo unit." },
];

for (const c of cases) {
  test(`${c.name} fit their container without horizontal scroll`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name === "mobile", "mobile uses cards / may scroll by design");
    await page.goto(c.path);
    const table = page.getByRole("table").filter({ has: page.getByRole("columnheader", { name: c.header }) }).first();
    await expect(table).toBeVisible();
    const hidden = await table.evaluate((el) => {
      const box = el.parentElement as HTMLElement;
      return box.scrollWidth - box.clientWidth;
    });
    expect(hidden).toBeLessThanOrEqual(0);
  });
}
