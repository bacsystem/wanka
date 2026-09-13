import { expect, test } from "@playwright/test";

// Dark-theme overflow / console audit for the batch-3 routes (light theme is covered by screens.spec.ts).
const routes = ["/salon/delivery", "/salon/reservas", "/hospitalizacion", "/grooming", "/catalogo/variantes/POL-001", "/reportes/gestion", "/finanzas/cuentas-por-pagar", "/finanzas/caja/historial", "/configuracion/auditoria", "/configuracion/integraciones", "/ventas/proformas/nueva", "/ventas/comprobantes/F001-00000842/nota-debito"];

test.use({ colorScheme: "dark" });
for (const r of routes) {
  test(`dark: ${r} has no overflow or console errors`, async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    page.on("pageerror", (e) => errors.push(e.message));
    await page.addInitScript(() => localStorage.setItem("theme", "dark"));
    await page.goto(r);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator("html")).toHaveClass(/dark/);
    await expect.poll(() => page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, window.innerWidth - window.visualViewport!.width)), { message: `horizontal overflow on ${r}`, timeout: 3000 }).toBeLessThanOrEqual(1);
    expect(errors).toEqual([]);
  });
}
