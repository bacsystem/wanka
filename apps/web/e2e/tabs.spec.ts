import { expect, test } from "./fixtures";
import type { Page } from "@playwright/test";

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
  await expect(page.getByRole("tablist")).toBeVisible();
  await expect(todas).toHaveAttribute("aria-selected", "true");
  await todas.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: /Aprobada/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("tab", { name: /Aprobada/ })).toHaveAttribute("aria-selected", "true");
  await expect(todas).toHaveAttribute("aria-selected", "false");
  await expectNoDanglingAriaControls(page);
});

test("customer detail: line strip, tabs by role and keyboard", async ({ page }) => {
  await page.goto("/clientes/u2");
  const resumen = page.getByRole("tab", { name: /Resumen/ });
  await expect(resumen).toHaveAttribute("aria-selected", "true");
  await resumen.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: /Comprobantes/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("tab", { name: /Comprobantes/ })).toHaveAttribute("aria-selected", "true");
  await expectNoDanglingAriaControls(page);
});

test("kardex: counters stay in the tab name and ArrowLeft wraps", async ({ page }) => {
  await page.goto("/inventario/kardex");
  const todos = page.getByRole("tab", { name: /^Todos/ });
  await todos.focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByRole("tab", { name: /Mermas/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByText("ACTA-MERM-10")).toBeVisible();
  await expectNoDanglingAriaControls(page);
});
