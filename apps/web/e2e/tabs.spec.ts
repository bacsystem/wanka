import { expect, test, type Page } from "./fixtures";

/**
 * Tab strips (khipu `Tabs`, strip-only mode): found by role, arrow keys move selection,
 * and no tab points `aria-controls` at a missing element.
 */
async function expectNoDanglingAriaControls(page: Page) {
  const dangling = await page.evaluate(() =>
    Array.from(document.querySelectorAll('[role="tab"][aria-controls]'))
      .map((t) => t.getAttribute("aria-controls") ?? "")
      .filter((id) => !document.getElementById(id)),
  );
  expect(dangling).toEqual([]);
}

test("cotizaciones: segmented strip, arrow keys select and filter", async ({ page }) => {
  await page.goto("/ventas/proformas");
  const todas = page.getByRole("tab", { name: /^Todas/ });
  await expect(page.getByRole("link", { name: "COT-2026-00125" })).toBeVisible();
  await expect(page.getByRole("tablist")).toBeVisible();
  await expect(todas).toHaveAttribute("aria-selected", "true");
  await todas.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: /Aprobada/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("tab", { name: /Aprobada/ })).toHaveAttribute("aria-selected", "true");
  await expect(todas).toHaveAttribute("aria-selected", "false");
  // the strip filters the table: an expired quote is gone under "Aprobada"
  await expect(page.getByRole("link", { name: "COT-2026-00125" })).toHaveCount(0);
  await expectNoDanglingAriaControls(page);
});

test("customer detail: line strip, tabs by role and keyboard", async ({ page }) => {
  await page.goto("/clientes/u2");
  // sections switch through panels: exactly one tabpanel, labelled by its tab
  await expect(page.getByRole("tabpanel")).toHaveCount(1);
  const resumen = page.getByRole("tab", { name: /Resumen/ });
  await expect(resumen).toHaveAttribute("aria-selected", "true");
  await resumen.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: /Comprobantes/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("tab", { name: /Comprobantes/ })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: /Comprobantes/ })).toBeVisible();
  await expect(page.getByRole("tabpanel")).toHaveCount(1);
  await expectNoDanglingAriaControls(page);
});

test("kardex: counters stay in the tab name and ArrowLeft wraps", async ({ page }) => {
  await page.goto("/inventario/kardex");
  const todos = page.getByRole("tab", { name: /^Todos/ });
  await expect(page.getByText("F001-08492")).toBeVisible();
  await todos.focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByRole("tab", { name: /Mermas/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByText("ACTA-MERM-10")).toBeVisible();
  // the strip filters the table: an entry row is gone under "Mermas"
  await expect(page.getByText("F001-08492")).toHaveCount(0);
  await expectNoDanglingAriaControls(page);
});

test("billing: counters keep the es-PE thousands separator in the tab name", async ({ page }) => {
  await page.goto("/admin/facturacion");
  await expect(page.getByRole("tab", { name: "Todos 1,482" })).toBeVisible();
});

test("customer detail: keyboard focus ring on a line tab is inset (not clipped by the scroller)", async ({ page }) => {
  await page.goto("/clientes/u2");
  await page.getByRole("tab", { name: /Resumen/ }).focus();
  await page.keyboard.press("ArrowRight");
  const comprobantes = page.getByRole("tab", { name: /Comprobantes/ });
  await expect(comprobantes).toBeFocused();
  await expect.poll(() => comprobantes.evaluate((el) => getComputedStyle(el).boxShadow)).toContain("inset");
});

test("segmented strips stay on one row at phone width", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "phone width only");
  for (const path of ["/admin/facturacion", "/ventas/proformas", "/finanzas/cuentas-por-cobrar", "/catalogo", "/inventario/kardex"]) {
    await page.goto(path);
    const first = page.getByRole("tab").first();
    const strip = page.getByRole("tablist").first();
    await expect(first).toBeVisible();
    const [stripH, tabH] = await Promise.all([strip.evaluate((el) => el.getBoundingClientRect().height), first.evaluate((el) => el.getBoundingClientRect().height)]);
    expect(stripH, path).toBeLessThanOrEqual(tabH + 8);
  }
});
