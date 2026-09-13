import { expect, test } from "@playwright/test";

test("dashboard renders its core sections", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Inicio" })).toBeVisible();
  await expect(page.getByText("Ventas de hoy")).toBeVisible();
  await expect(page.getByText("Ventas últimos 30 días")).toBeVisible();
  await expect(page.getByText("Últimos comprobantes emitidos")).toBeVisible();
  await expect(page.getByText("Alertas de stock")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Estado de envíos SUNAT" })).toBeVisible();
});

test("navigation adapts to the viewport", async ({ page }, testInfo) => {
  await page.goto("/");
  const sidebar = page.locator('[data-slot="sidebar"]');
  const bottomNav = page.getByRole("navigation", { name: "Navegación principal" });

  if (testInfo.project.name === "mobile") {
    await expect(bottomNav).toBeVisible();
    await expect(page.locator('[data-slot="sidebar-container"]')).toBeHidden();
    await expect(page.getByLabel("Nueva venta")).toBeVisible();
  } else {
    await expect(sidebar.first()).toBeVisible();
    await expect(bottomNav).toBeHidden();
    await expect(sidebar.getByRole("link", { name: "Inicio", exact: true })).toBeVisible();
  }

  await page.screenshot({
    path: testInfo.outputPath(`dashboard-${testInfo.project.name}.png`),
    fullPage: true,
  });
});

test("tablet starts with the sidebar collapsed to icons", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "tablet");
  await page.goto("/");
  await expect(page.locator('[data-slot="sidebar"][data-state="collapsed"]')).toBeVisible();
});
