import { expect, test } from "./fixtures";

// El login real (#24) reemplazó al mock; el 2FA (#25) y la selección de empresa (#27) lo reactivan sobre la API.
test.skip("login → 2FA → company selection → shell with the chosen tenant", async ({ page }) => {
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
  // En móvil el selector de empresa vive en el cajón de «Más».
  const more = page.getByRole("button", { name: "Más" });
  if (await more.isVisible()) await more.click();
  const selector = page.getByRole("combobox", { name: "Cambiar de empresa" }).filter({ visible: true });
  await expect(selector).toContainText("Veterinaria San Borja");
  // switch back from the sidebar selector
  await selector.click();
  await page.getByRole("option", { name: /Clínica Dental Sonrisa/ }).click();
  await expect(selector).toContainText("Clínica Dental Sonrisa");
});

// La validación del login real está en e2e/login.spec.ts; la recuperación de contraseña vuelve con #26.
test.skip("login validates credentials and recover flow shows confirmation", async ({ page }) => {
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

// Pantalla mock inaccesible hasta que la historia #34 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("customer file: tabs and link to the clinical record with an interactive odontogram", async ({ page }) => {
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

// Pantalla mock inaccesible hasta que la historia #53 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("quote detail: editing a line updates totals and emitting marks it invoiced", async ({ page }) => {
  await page.goto("/ventas/proformas/COT-2026-00128");
  await expect(page.getByText("Importe total cotizado")).toBeVisible();
  await expect(page.locator("dd", { hasText: /S\/ 13,153\.46/ }).first()).toBeVisible();
  await page.getByRole("button", { name: "Quitar línea" }).first().click();
  await expect(page.locator("dd", { hasText: /S\/ 13,153\.46/ })).toHaveCount(0);
  await expect(page.locator("dd", { hasText: /S\/ 12,091\.46/ }).first()).toBeVisible();
  await page.getByRole("button", { name: /Aprobar y emitir factura F001/ }).click();
  await expect(page.getByText(/Facturada · F001-00000843/)).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #52 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("credit note wizard: partial devolution recomputes and emits", async ({ page }) => {
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

// Pantalla mock inaccesible hasta que la historia #52 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("void dialog requires a motive", async ({ page }) => {
  await page.goto("/ventas/comprobantes/B001-00004291");
  await page.getByRole("button", { name: "Anular / comunicar baja", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Enviar resumen de bajas" })).toBeDisabled();
  await dialog.getByRole("textbox").fill("Error en la digitación del importe");
  await dialog.getByRole("button", { name: "Enviar resumen de bajas" }).click();
  await expect(page.getByText(/Resumen de bajas enviado/)).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #46 / #65 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("reception: received quantity drives missing count; physical count drives the adjustment", async ({ page }) => {
  await page.goto("/inventario/recepcion/T001-000428");
  await expect(page.getByTestId("missing")).toHaveText("1");
  await page.getByLabel("Recibido MAT-0104").filter({ visible: true }).fill("18");
  await expect(page.getByTestId("missing")).toHaveText("3");
  await page.goto("/inventario/toma-fisica");
  await expect(page.getByTestId("count-diff")).toHaveText("− S/ 184.00");
  await page.getByLabel("Conteo INSU-0055").filter({ visible: true }).fill("18");
  await expect(page.getByTestId("count-diff")).toHaveText("− S/ 120.00");
});

test("command palette searches the enabled features and navigates", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "mobile", "the search shortcut is for keyboards");
  await page.goto("/");
  await page.keyboard.press("Control+k");
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("textbox", { name: "Ir a una funcionalidad…" }).fill("inicio");
  await dialog.getByRole("option", { name: /Inicio/ }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(dialog).toBeHidden();
});

// Pantalla mock inaccesible hasta que la historia #75 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("agenda hands the patient and procedure to the POS", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop");
  await page.goto("/agenda");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: /08:30 Juan Carlos Pérez Huamán/ }).click();
  await page.locator('aside[aria-label="Detalle de cita"]').getByRole("button", { name: /Cobrar y emitir comprobante/ }).click();
  await expect(page).toHaveURL(/\/ventas\/nueva\?cliente=u1&items=c1$/);
  await expect(page.getByRole("complementary", { name: "Carrito" }).getByText("Juan Carlos Pérez Huamán")).toBeVisible();
  await expect(page.getByTestId("cart-total")).toHaveText("S/ 85.00");
});
