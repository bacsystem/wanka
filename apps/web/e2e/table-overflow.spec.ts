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
    // Measure only once the layout has settled. On tablet the server HTML renders the sidebar expanded and
    // `SidebarAutoCollapse` collapses it after hydration (the container goes 718 -> 910 px a moment later), so a
    // single early measurement saw the narrow container and reported 16px of overflow (#144). Wait for fonts and
    // for the container width to stay unchanged across ~400ms of frames (longer than the sidebar transition).
    await table.evaluate(async (el) => {
      const box = el.parentElement as HTMLElement;
      await document.fonts.ready;
      let last = -1;
      let since = performance.now();
      await new Promise<void>((resolve) => {
        const tick = () => {
          if (box.clientWidth !== last) {
            last = box.clientWidth;
            since = performance.now();
          }
          if (performance.now() - since >= 400) resolve();
          else requestAnimationFrame(tick);
        };
        tick();
      });
    });
    const hidden = await table.evaluate((el) => {
      const box = el.parentElement as HTMLElement;
      return box.scrollWidth - box.clientWidth;
    });
    expect(hidden).toBeLessThanOrEqual(0);
  });
}
