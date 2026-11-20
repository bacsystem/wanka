import { expect, test, waitForHydration } from "./fixtures";

const screens: { path: string; heading: RegExp }[] = [
  { path: "/admin", heading: /Panel maestro de operaciones/ },
  { path: "/admin/empresas", heading: /Directorio de empresas/ },
  { path: "/admin/empresas/TEN-104", heading: /OdontoSalud/ },
  { path: "/admin/planes", heading: /Catálogo de planes/ },
  { path: "/admin/facturacion", heading: /Facturación y cobranzas/ },
  { path: "/admin/monitoreo", heading: /Monitoreo SUNAT/ },
  { path: "/admin/usuarios", heading: /Personal interno/ },
  { path: "/admin/soporte", heading: /Bandeja de tickets/ },
  { path: "/admin/modulos", heading: /Feature flags/ },
  { path: "/admin/auditoria", heading: /Auditoría global/ },
  { path: "/admin/configuracion", heading: /Configuración de plataforma/ },
];

for (const s of screens) {
  // Consola /admin: pantalla mock inaccesible (404) hasta que su historia la habilite; entonces se reactiva.
  test.skip(`admin: renders ${s.path} without console errors or overflow`, async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(s.path);
    await expect(page.getByRole("heading", { level: 1, name: s.heading })).toBeVisible();
    await expect.poll(() => page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, window.innerWidth - window.visualViewport!.width)), { message: `horizontal overflow on ${s.path}`, timeout: 3000 }).toBeLessThanOrEqual(1);
    expect(errors).toEqual([]);
  });
}

// Consola /admin: pantalla mock inaccesible (404) hasta que su historia la habilite; entonces se reactiva.
test.skip("admin login: SSO → 2FA → dashboard; console is themed and separate from the tenant app", async ({ page }, testInfo) => {
  await page.goto("/admin/login");
  await expect(page.getByRole("heading", { name: "Ingreso a la consola" })).toBeVisible();
  await page.getByRole("button", { name: /Continuar con Google Workspace/ }).click();
  await page.getByLabel("Código TOTP").fill("123456");
  await page.getByRole("button", { name: /Verificar e ingresar/ }).click();
  await page.waitForURL(/\/admin$/);
  await expect(page.getByRole("heading", { level: 1, name: /Panel maestro de operaciones/ })).toBeVisible();
  if (testInfo.project.name === "desktop") await expect(page.getByRole("button", { name: /app de empresa/i }).first()).toBeVisible();
  // /admin/login and /admin are different root layouts (full page load): wait for hydration before clicking.
  await waitForHydration(page);
  await page.getByRole("button", { name: "Menú de usuario" }).click();
  await expect(page.getByRole("menuitem", { name: "Mi perfil" })).toBeVisible();
  await page.getByRole("menuitem", { name: "Mi perfil" }).click();
  await expect(page).toHaveURL(/\/admin\/perfil$/);
  await expect(page.getByRole("heading", { level: 1, name: /Mi perfil y seguridad/ })).toBeVisible();
});

// Consola /admin: pantalla mock inaccesible (404) hasta que su historia la habilite; entonces se reactiva.
test.skip("admin 404: unknown /admin/* URL shows the console 404, not the tenant one", async ({ page }) => {
  const response = await page.goto("/admin/no-existe");
  // The console has no loading.js, so the body is not streamed and Next answers a real 404
  // (the tenant app streams behind its loading.js and keeps answering 200; out of scope here).
  expect(response?.status()).toBe(404);
  await expect(page.getByRole("heading", { name: "No encontramos esta página de la consola" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
  await expect(page.locator("meta[name=robots][content*=noindex]")).toHaveCount(1);
  // admin shell (its own layout), not the tenant one
  await expect(page).toHaveTitle(/Consola de administración/);
  await expect(page.getByText("Ir al panel", { exact: true })).toHaveCount(0);
  await waitForHydration(page);
  await page.getByRole("link", { name: "Ir al dashboard de la consola" }).click();
  await page.waitForURL(/\/admin$/);
  await expect(page.getByRole("heading", { level: 1, name: /Panel maestro de operaciones/ })).toBeVisible();
});

test("tenant 404 is unchanged for unknown URLs outside /admin", async ({ page }) => {
  await page.goto("/no-existe");
  await expect(page.getByRole("heading", { name: "Página no encontrada" })).toBeVisible();
  await expect(page.getByRole("heading", { name: /de la consola/ })).toHaveCount(0);
  await expect(page.locator("html")).not.toHaveAttribute("data-paleta", "indigo");
  await expect(page.getByRole("link", { name: "Ir al inicio" })).toBeVisible();
});

// Consola /admin: pantalla mock inaccesible (404) hasta que su historia la habilite; entonces se reactiva.
test.skip("admin tenants: filters, suspend toggle, alta drawer and detail", async ({ page }) => {
  await page.goto("/admin/empresas");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: /Cuota CPE > 90%/ }).click();
  await expect(page.getByText(/PetMed Sur/).filter({ visible: true }).first()).toBeVisible();
  await expect(page.getByText("OdontoSalud")).toHaveCount(0);
  await page.getByRole("button", { name: "Limpiar filtros" }).click();
  await page.locator("main, [data-slot=sidebar-inset]").getByRole("button", { name: "Alta de empresa", exact: true }).last().click();
  await expect(page.getByRole("dialog").getByText("Alta de empresa (tenant)")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Cancelar" }).click();
  await page.getByRole("link", { name: /Clínica Dental OdontoSalud/ }).filter({ visible: true }).first().click();
  await expect(page).toHaveURL(/\/admin\/empresas\/TEN-104$/);
  await page.getByRole("button", { name: /Impersonar consola/ }).click();
  await expect(page.getByText(/Impersonando/)).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #52 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("data table: columns menu hides a column and reset restores it", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "mobile");
  await page.goto("/ventas/comprobantes");
  await page.waitForLoadState("networkidle");
  await expect(page.getByRole("columnheader", { name: "Cliente / adquirente" })).toBeVisible();
  await page.getByRole("button", { name: "Columnas" }).click();
  await page.getByRole("menuitemcheckbox", { name: "Cliente / adquirente" }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("columnheader", { name: "Cliente / adquirente" })).toHaveCount(0);
  await page.getByRole("button", { name: "Restablecer" }).click();
  await expect(page.getByRole("columnheader", { name: "Cliente / adquirente" })).toBeVisible();
});
