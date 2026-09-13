import { expect, test, type Page } from "@playwright/test";

/** Wait for hydration before the first interaction (client handlers are not attached on first paint). */
async function settle(page: Page) {
  await page.waitForLoadState("networkidle");
}

test("comprobantes: search, type filter and bulk selection", async ({ page }) => {
  await page.goto("/ventas/comprobantes");
  await settle(page);
  const search = page.getByRole("searchbox", { name: /Buscar por serie/ });
  await search.fill("alpamayo");
  await expect(page.getByText("Inversiones Alpamayo S.A.C.").filter({ visible: true }).first()).toBeVisible();
  await expect(page.getByText("Juan Carlos Pérez Huamán")).toHaveCount(0);
  await search.fill("");
  await page.getByRole("combobox", { name: "Tipo de comprobante" }).click();
  await page.getByRole("option", { name: "Nota de crédito (07)" }).click();
  await expect(page.getByText("FC01-00000104").filter({ visible: true }).first()).toBeVisible();
  await expect(page.getByText("B001-00004291")).toHaveCount(0);
  await page.getByRole("button", { name: "Limpiar" }).click();
});

test("comprobantes: bulk actions bar appears when rows are selected (desktop)", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "mobile");
  await page.goto("/ventas/comprobantes");
  await settle(page);
  await page.getByRole("checkbox", { name: "Seleccionar todos" }).click();
  await expect(page.getByText(/10 comprobantes seleccionados/)).toBeVisible();
  await page.getByRole("button", { name: /Reenviar a SUNAT/ }).click();
  await expect(page.getByText("10 comprobantes reenviados a SUNAT")).toBeVisible();
});

test("comprobante detail opens from the list and shows CDR", async ({ page }) => {
  await page.goto("/ventas/comprobantes");
  await settle(page);
  await page.getByRole("link", { name: "F001-00000842" }).first().click();
  await expect(page).toHaveURL(/F001-00000842$/);
  await expect(page.getByText("La Factura número F001-842 ha sido aceptada")).toBeVisible();
  await expect(page.getByText("Total a pagar:")).toBeVisible();
});

test("clientes: new customer drawer validates document length", async ({ page }) => {
  await page.goto("/clientes");
  await settle(page);
  await page.getByRole("button", { name: /Nuevo cliente/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Registrar nuevo cliente / paciente")).toBeVisible();
  await dialog.getByPlaceholder("72109843").fill("123");
  await dialog.getByRole("button", { name: "Consultar" }).click();
  await expect(page.getByText("El DNI debe tener 8 dígitos")).toBeVisible();
  await dialog.getByPlaceholder("72109843").fill("72109843");
  await dialog.getByRole("button", { name: "Consultar" }).click();
  await expect(page.getByText("RENIEC: datos encontrados")).toBeVisible();
  await dialog.getByRole("button", { name: "Cancelar" }).click();
  await expect(dialog).toBeHidden();
});

test("cotizaciones: tabs filter and 1-click invoicing dialog computes detraction", async ({ page }) => {
  await page.goto("/ventas/proformas");
  await settle(page);
  await page.getByRole("tab", { name: /Vencida/ }).click();
  await expect(page.getByRole("link", { name: "COT-2026-00125" })).toBeVisible();
  await expect(page.getByText("COT-2026-00128")).toHaveCount(0);
  await page.getByRole("tab", { name: /Aprobada/ }).click();
  await page.getByRole("button", { name: "Facturar 1-clic" }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("S/ 13,153.46").first()).toBeVisible();
  await dialog.getByRole("switch", { name: "Aplicar detracción" }).click();
  await expect(dialog.getByText("S/ 1,578.42")).toBeVisible();
  await dialog.getByRole("button", { name: /Emitir y transmitir/ }).click();
  await expect(page.getByText(/Factura F001-00000843 emitida/)).toBeVisible();
});

test("stock: warehouse and state filters narrow the table", async ({ page }) => {
  await page.goto("/inventario/stock");
  await settle(page);
  await page.getByRole("combobox", { name: "Estado de stock" }).click();
  await page.getByRole("option", { name: "Crítico / agotado" }).click();
  await expect(page.getByText("Resina fotocurable Filtek Z350 XT").filter({ visible: true }).first()).toBeVisible();
  await expect(page.getByText("Ibuprofeno 400 mg")).toHaveCount(0);
  await page.getByRole("button", { name: /Nueva transferencia/ }).click();
  await expect(page.getByRole("dialog").getByText("Nueva transferencia interna")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Despachar transferencia" }).click();
  await expect(page.getByText(/Transferencia despachada/)).toBeVisible();
});

test("caja: cash count drives the difference and gates the closing", async ({ page }) => {
  await page.goto("/finanzas/caja");
  await settle(page);
  const close = page.getByRole("button", { name: /Z-Report/ });
  await expect(close).toBeEnabled();
  const input = page.getByRole("textbox", { name: "Cantidad de S/ 200" });
  await input.fill("3");
  await expect(page.getByTestId("cash-counted")).toHaveText("S/ 1,620.00");
  await expect(page.getByText(/\(sobrante\)/)).toBeVisible();
  await expect(close).toBeDisabled();
  await input.fill("2");
  await expect(close).toBeEnabled();
});

test("cuentas por cobrar: register payment dialog", async ({ page }) => {
  await page.goto("/finanzas/cuentas-por-cobrar");
  await settle(page);
  await page.getByRole("button", { name: "Abonar" }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText(/F001-000835/)).toBeVisible();
  await dialog.getByRole("button", { name: "Confirmar abono" }).click();
  await expect(page.getByText(/Abono de S\/ 2,800.00 registrado/)).toBeVisible();
});

test("usuarios: permission matrix toggles and resets", async ({ page }) => {
  await page.goto("/configuracion/usuarios");
  await settle(page);
  await page.getByRole("tab", { name: /Roles y matriz/ }).click();
  const cell = page.getByRole("checkbox", { name: "Caja y arqueo: Ver" });
  await expect(cell).not.toBeChecked();
  await cell.click();
  await expect(cell).toBeChecked();
  await page.getByRole("button", { name: "Restablecer" }).click();
  await expect(cell).not.toBeChecked();
  await page.getByRole("button", { name: "Super Admin" }).click();
  await expect(page.getByRole("checkbox", { name: "Caja y arqueo: Ver" })).toBeDisabled();
});

test("agenda: selecting an appointment shows its detail", async ({ page }, testInfo) => {
  await page.goto("/agenda");
  await settle(page);
  if (testInfo.project.name !== "desktop") {
    // Below xl the detail opens in a bottom sheet (mobile defaults to the list view).
    await page.getByRole("button", { name: /Mariana Quispe Flores/ }).click();
    await expect(page.getByRole("dialog").getByText("Cita #OD-8496")).toBeVisible();
  } else {
    await page.getByRole("button", { name: /10:15 Mariana Quispe Flores/ }).click();
    await expect(page.locator('aside[aria-label="Detalle de cita"]').getByText("Cita #OD-8496")).toBeVisible();
    await page.getByRole("button", { name: "Hacer pasar" }).click();
    await expect(page.getByText("Mariana Quispe Flores pasó a sillón")).toBeVisible();
  }
});

test("theme toggle switches to dark and back", async ({ page }) => {
  await page.goto("/");
  await settle(page);
  await page.getByRole("button", { name: "Cambiar tema" }).click();
  await page.getByRole("menuitemradio", { name: "Oscuro" }).click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.getByRole("button", { name: "Cambiar tema" }).click();
  await page.getByRole("menuitemradio", { name: "Claro" }).click();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
});

test("mobile: 'Más' sheet lists remaining modules", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile");
  await page.goto("/");
  await settle(page);
  await page.getByRole("button", { name: "Más" }).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet.getByRole("link", { name: "Tributarios SUNAT" })).toBeVisible();
  await sheet.getByRole("link", { name: "Caja y arqueo" }).click();
  await expect(page).toHaveURL(/\/finanzas\/caja$/);
});

test("catálogo: tabs route to sub-paths and the 3-step drawer computes IGV", async ({ page }) => {
  await page.goto("/catalogo");
  await settle(page);
  await page.getByRole("tab", { name: /Productos \(kardex\)/ }).click();
  await expect(page).toHaveURL(/\/catalogo\/productos$/);
  await expect(page.getByText("SRV-0012")).toHaveCount(0);
  await page.getByRole("button", { name: /Nuevo servicio \/ producto/ }).click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("button", { name: "Siguiente" }).click();
  await expect(sheet.getByText("S/ 72.03")).toBeVisible();
  await sheet.getByRole("button", { name: "Siguiente" }).click();
  await sheet.getByRole("button", { name: "Guardar en catálogo" }).click();
  await expect(page.getByText("Ítem guardado en el catálogo")).toBeVisible();
});

test("movimientos: SUNAT operation filter and movement drawer", async ({ page }) => {
  await page.goto("/inventario/movimientos");
  await settle(page);
  await page.getByRole("combobox", { name: "Operación SUNAT" }).click();
  await page.getByRole("option", { name: /11 – Merma/ }).click();
  await expect(page.getByText("MOV-2026-001287").filter({ visible: true }).first()).toBeVisible();
  await expect(page.getByText("MOV-2026-001290")).toHaveCount(0);
  await page.getByRole("button", { name: "Registrar movimiento" }).click();
  await expect(page.getByRole("dialog").getByText("S/ 785.00")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Guardar y aplicar al kardex" }).click();
  await expect(page.getByText(/aplicado al kardex/)).toBeVisible();
});

test("almacenes: site filter narrows the warehouse cards", async ({ page }) => {
  await page.goto("/inventario/almacenes");
  await settle(page);
  await page.getByRole("combobox", { name: "Sede" }).click();
  await page.getByRole("option", { name: "Sede San Isidro" }).click();
  await expect(page.getByRole("heading", { name: /Almacén San Isidro/ })).toBeVisible();
  await expect(page.getByRole("heading", { name: /Almacén Central de Farmacia/ })).toHaveCount(0);
});

test("compras: tabs navigate to proveedores and órdenes; new supplier drawer opens", async ({ page }) => {
  await page.goto("/compras");
  await settle(page);
  await page.getByRole("navigation", { name: "Secciones de compras" }).getByRole("link", { name: /Proveedores/ }).click();
  await expect(page).toHaveURL(/\/compras\/proveedores$/);
  await page.getByRole("button", { name: "Nuevo proveedor" }).click();
  await expect(page.getByRole("dialog").getByText("Validación automática del RUC")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Cancelar" }).click();
  await page.getByRole("navigation", { name: "Secciones de compras" }).getByRole("link", { name: /Órdenes de compra/ }).click();
  await expect(page).toHaveURL(/\/compras\/ordenes$/);
  await expect(page.getByText("OC-2026-0091").filter({ visible: true }).first()).toBeVisible();
});

test("kardex: filters rows and switches to the physical-only (12.1) layout", async ({ page }) => {
  await page.goto("/inventario/kardex");
  await settle(page);
  await expect(page.getByText("F001-08492")).toBeVisible();
  await page.getByRole("tab", { name: /Mermas/ }).click();
  await expect(page.getByText("ACTA-MERM-10")).toBeVisible();
  await expect(page.getByText("F001-08492")).toHaveCount(0);
  await page.getByRole("button", { name: "Físico (12.1)" }).click();
  await expect(page.getByRole("columnheader", { name: "C. ponderado" })).toHaveCount(0);
});

test("user menu opens with profile and logout", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Menú de usuario" }).click();
  await expect(page.getByRole("menuitem", { name: /Cerrar sesión/ })).toBeVisible();
});
