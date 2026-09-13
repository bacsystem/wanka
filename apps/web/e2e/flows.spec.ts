import { expect, test } from "@playwright/test";

test("login → 2FA → company selection → dashboard with the chosen tenant", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Contraseña", { exact: true }).fill("secreto123");
  await page.getByRole("button", { name: /Ingresar a Wanka/ }).click();
  await expect(page.getByRole("heading", { name: /Verificación en dos pasos/ })).toBeVisible();
  for (let i = 1; i <= 6; i++) await page.getByLabel(`Dígito ${i}`).fill(String(i));
  await page.getByRole("button", { name: "Verificar y acceder" }).click();
  await expect(page).toHaveURL(/\/seleccionar-empresa$/);
  await page.getByRole("button", { name: /Veterinaria San Borja/ }).click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("button", { name: /Empresa actual: Veterinaria San Borja/ })).toBeVisible();
  // switch back from the topbar
  await page.getByRole("button", { name: /Empresa actual/ }).click();
  await page.getByRole("menuitem", { name: /Clínica Dental Sonrisa/ }).click();
  await expect(page.getByRole("button", { name: /Empresa actual: Clínica Dental Sonrisa/ })).toBeVisible();
});

test("login validates credentials and recover flow shows confirmation", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Contraseña", { exact: true }).fill("123");
  await page.getByRole("button", { name: /Ingresar a Wanka/ }).click();
  await expect(page.getByText(/contraseña de al menos 6/)).toBeVisible();
  await page.getByRole("button", { name: "¿Olvidaste tu contraseña?" }).click();
  await page.getByRole("button", { name: /Enviar enlace/ }).click();
  await expect(page.getByText("Enlace despachado con éxito")).toBeVisible();
});

test("onboarding: industry choice swaps the module list and steps advance", async ({ page }) => {
  await page.goto("/onboarding");
  await page.getByRole("button", { name: /Restaurante, café y bar/ }).click();
  await expect(page.getByText(/^Control de mesas y comandas/)).toBeVisible();
  await page.getByRole("button", { name: /Guardar y continuar al paso 3/ }).click();
  await expect(page.getByRole("heading", { name: /Certificado digital y Clave SOL/ })).toBeVisible();
});

test("customer file: tabs and link to the clinical record with an interactive odontogram", async ({ page }) => {
  await page.goto("/clientes/u2");
  await page.getByRole("tab", { name: /Cuenta corriente/ }).click();
  await expect(page.getByText("Pago POS Izipay")).toBeVisible();
  await page.getByRole("tab", { name: /Historia clínica/ }).click();
  await expect(page).toHaveURL(/\/clientes\/u2\/historia$/);
  await page.getByRole("button", { name: "Corona", exact: true }).click();
  await page.getByRole("button", { name: /^Pieza 11:/ }).click();
  await expect(page.getByRole("button", { name: "Pieza 11: Corona" })).toBeVisible();
  await expect(page.getByText(/Pieza seleccionada:/)).toContainText("11");
});

test("quote detail: editing a line updates totals and emitting marks it invoiced", async ({ page }) => {
  await page.goto("/ventas/proformas/COT-2026-00128");
  await expect(page.getByText("Importe total cotizado")).toBeVisible();
  await expect(page.locator("dd", { hasText: /S\/ 13,153\.46/ }).first()).toBeVisible();
  await page.getByRole("button", { name: "Quitar línea" }).first().click();
  await expect(page.locator("dd", { hasText: /S\/ 13,153\.46/ })).toHaveCount(0);
  await expect(page.locator("dd", { hasText: /S\/ 12,091\.46/ }).first()).toBeVisible();
  await page.getByRole("button", { name: /Aprobar y emitir factura F001/ }).click();
  await expect(page.getByText(/Facturada · F001-00000843/)).toBeVisible();
});

test("credit note wizard: partial devolution recomputes and emits", async ({ page }) => {
  await page.goto("/ventas/comprobantes/F001-00000842");
  await page.getByRole("button", { name: /Emitir nota de crédito/ }).click();
  await expect(page).toHaveURL(/nota-credito$/);
  await page.getByRole("button", { name: /07.*Devolución por ítem/ }).click();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.getByLabel("Cantidad a anular CIR-012").fill("0");
  await page.getByLabel("Cantidad a anular INS-088").fill("0");
  await expect(page.getByTestId("nc-total")).toHaveText("S/ 448.40");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.getByRole("button", { name: /Emitir y transmitir a SUNAT/ }).click();
  await expect(page.getByText(/Nota de crédito FC01-00000105 emitida/)).toBeVisible();
});

test("void dialog requires a motive", async ({ page }) => {
  await page.goto("/ventas/comprobantes/B001-00004291");
  await page.getByRole("button", { name: "Anular / comunicar baja", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Enviar resumen de bajas" })).toBeDisabled();
  await dialog.getByRole("textbox").fill("Error en la digitación del importe");
  await dialog.getByRole("button", { name: "Enviar resumen de bajas" }).click();
  await expect(page.getByText(/Resumen de bajas enviado/)).toBeVisible();
});

test("reception: received quantity drives missing count; physical count drives the adjustment", async ({ page }) => {
  await page.goto("/inventario/recepcion/T001-000428");
  await expect(page.getByTestId("missing")).toHaveText("1");
  await page.getByLabel("Recibido MAT-0104").filter({ visible: true }).fill("18");
  await expect(page.getByTestId("missing")).toHaveText("3");
  await page.goto("/inventario/toma-fisica");
  await expect(page.getByTestId("count-diff")).toHaveText("− S/ 184.00");
  await page.getByLabel("Conteo INSU-0055").filter({ visible: true }).fill("18");
  await expect(page.getByTestId("count-diff")).toHaveText("− S/ 120.00");
});

test("command palette searches and navigates; notifications open", async ({ page }, testInfo) => {
  await page.goto("/");
  await page.waitForTimeout(400);
  await page.keyboard.press("Control+k");
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("textbox", { name: "Buscar" })).toBeVisible();
  await dialog.getByRole("textbox", { name: "Buscar" }).fill("alpamayo");
  await dialog.getByRole("option", { name: /Factura F001-00000842/ }).click();
  await expect(page).toHaveURL(/comprobantes\/F001-00000842$/);
  if (testInfo.project.name !== "mobile") {
    await page.getByRole("button", { name: /Notificaciones/ }).click();
    await expect(page.getByText(/Nota de crédito FC01-00000104 observada/)).toBeVisible();
  }
});

test("agenda hands the patient and procedure to the POS", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop");
  await page.goto("/agenda");
  await page.waitForTimeout(300);
  await page.getByRole("button", { name: /08:30 Juan Carlos Pérez Huamán/ }).click();
  await page.locator('aside[aria-label="Detalle de cita"]').getByRole("button", { name: /Cobrar y emitir comprobante/ }).click();
  await expect(page).toHaveURL(/\/ventas\/nueva\?cliente=u1&items=c1$/);
  await expect(page.getByRole("complementary", { name: "Carrito" }).getByText("Juan Carlos Pérez Huamán")).toBeVisible();
  await expect(page.getByTestId("cart-total")).toHaveText("S/ 85.00");
});
