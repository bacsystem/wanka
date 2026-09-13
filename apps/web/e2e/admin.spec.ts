import { expect, test } from "@playwright/test";

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
  test(`admin: renders ${s.path} without console errors or overflow`, async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(s.path);
    await expect(page.getByRole("heading", { level: 1, name: s.heading })).toBeVisible();
    await page.waitForTimeout(350);
    const overflow = await page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, window.innerWidth - window.visualViewport!.width));
    expect(overflow, `horizontal overflow on ${s.path}`).toBeLessThanOrEqual(1);
    expect(errors).toEqual([]);
  });
}

test("admin login: SSO → 2FA → dashboard; console is themed and separate from the tenant app", async ({ page }, testInfo) => {
  await page.goto("/admin/login");
  await expect(page.getByRole("heading", { name: "Ingreso a la consola" })).toBeVisible();
  await page.getByRole("button", { name: /Continuar con Google Workspace/ }).click();
  await page.getByLabel("Código TOTP").fill("123456");
  await page.getByRole("button", { name: /Verificar e ingresar/ }).click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole("heading", { level: 1, name: /Panel maestro de operaciones/ })).toBeVisible();
  if (testInfo.project.name === "desktop") await expect(page.getByRole("button", { name: /app de empresa/i }).first()).toBeVisible();
  await page.getByRole("button", { name: "Menú de usuario" }).click();
  await page.getByRole("menuitem", { name: "Mi perfil" }).click();
  await expect(page).toHaveURL(/\/admin\/perfil$/);
  await expect(page.getByRole("heading", { level: 1, name: /Mi perfil y seguridad/ })).toBeVisible();
});

test("admin tenants: filters, suspend toggle, alta drawer and detail", async ({ page }) => {
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

test("data table: columns menu hides a column and reset restores it", async ({ page }, testInfo) => {
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
