import { expect, test } from "./fixtures";

// Todas estas rutas son pantallas mock inaccesibles (404): se reactivan cuando cada historia (#113 y #115 salón, #118 hospitalización y grooming,
// #40 variantes, #72 reportes, #70 cuentas por pagar, #68 turnos, #79 auditoría, #77 integraciones, #53 cotizaciones, #52/#59 notas) habilita la suya.
// Dark-theme overflow / console audit for the batch-3 routes (light theme is covered by screens.spec.ts).
const routes = ["/salon/delivery", "/salon/reservas", "/hospitalizacion", "/grooming", "/catalogo/variantes/POL-001", "/reportes/gestion", "/finanzas/cuentas-por-pagar", "/finanzas/caja/historial", "/configuracion/auditoria", "/configuracion/integraciones", "/ventas/proformas/nueva", "/ventas/comprobantes/F001-00000842/nota-debito"];

test.use({ colorScheme: "dark" });
for (const r of routes) {
  test.skip(`dark: ${r} has no overflow or console errors`, async ({ page }) => {
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
