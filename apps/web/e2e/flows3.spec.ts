import { expect, test } from "./fixtures";

// Pantalla mock inaccesible hasta que la historia #53 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("new quote wizard: template, customer, items, live totals and send", async ({ page }) => {
  await page.goto("/ventas/proformas");
  await page.getByRole("button", { name: "Nueva cotización" }).click();
  await expect(page).toHaveURL(/\/ventas\/proformas\/nueva$/);
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByText("Selecciona un cliente")).toBeVisible();
  await page.getByRole("button", { name: /Blanqueamiento promo/ }).click();
  await page.getByLabel("Buscar cliente").fill("María");
  await page.getByRole("button", { name: /María Castillo Quispe/ }).click();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByRole("heading", { name: /Ítems · 2/ })).toBeVisible();
  await page.getByRole("button", { name: /Quitar/ }).first().click();
  await expect(page.getByRole("heading", { name: /Ítems · 1/ })).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByText("Enlace de aprobación en línea", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Crear y enviar cotización" }).click();
  await expect(page.getByText(/COT-2026-00129 creada/)).toBeVisible();
  await expect(page).toHaveURL(/\/ventas\/proformas$/);
});

// Pantalla mock inaccesible hasta que la historia #52 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("debit note: SUNAT catalogue 10 type, amount and issue", async ({ page }) => {
  await page.goto("/ventas/comprobantes/F001-00000842");
  await page.getByRole("button", { name: "Nota de débito" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /Emitir nota de débito/ })).toBeVisible();
  await page.getByRole("button", { name: /02 Aumento en el valor/ }).click();
  await page.getByLabel("Monto adicional (inc. IGV)").fill("118");
  await expect(page.getByText("Total nota de débito").locator("..").getByText("S/ 118.00")).toBeVisible();
  await page.getByRole("button", { name: /Emitir y transmitir a SUNAT/ }).click();
  await expect(page.getByText(/FD01-00000012 emitida/)).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #70 / #77 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("payables: select invoices and schedule a payment; integrations connect", async ({ page }) => {
  await page.goto("/finanzas/cuentas-por-pagar");
  await page.waitForLoadState("networkidle");
  const schedule = page.getByRole("button", { name: "Programar pago" });
  await expect(schedule).toBeDisabled();
  await page.getByRole("checkbox", { name: /Seleccionar F002-011820/ }).filter({ visible: true }).click();
  await expect(schedule).toBeEnabled();
  await schedule.click();
  await page.getByRole("dialog").getByRole("button", { name: /Programar/ }).click();
  await expect(page.getByText(/Pago programado/)).toBeVisible();
  await page.goto("/configuracion/integraciones");
  await page.getByRole("button", { name: "Conectar" }).first().click();
  await expect(page.getByText(/conectado$/).first()).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #68 / #72 / #79 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("shift history, audit log and management report tabs render data", async ({ page }) => {
  await page.goto("/finanzas/caja/historial");
  await expect(page.getByText("Turnos cerrados", { exact: true })).toBeVisible();
  await page.goto("/configuracion/auditoria");
  await expect(page.getByText("Registro de auditoría", { exact: true })).toBeVisible();
  await page.goto("/reportes/gestion");
  await page.getByRole("tab", { name: /Por producto/ }).click();
  await expect(page.getByText("Ventas por producto / servicio")).toBeVisible();
  await page.getByRole("tab", { name: /Cuentas por cobrar/ }).click();
  await expect(page.getByText("Antigüedad de cuentas por cobrar")).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #40 / #48 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("retail: variants matrix filters and POS barcode scan adds the item", async ({ page }) => {
  await page.goto("/catalogo/variantes/POL-001");
  await page.getByRole("tab", { name: /Stock < 10/ }).click();
  await expect(page.getByRole("tab", { name: /Stock < 10/ })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("Listas de precios", { exact: true })).toBeVisible();
  await page.goto("/ventas/nueva");
  await page.getByLabel("Escanear código").fill("7751234500001");
  await page.getByLabel("Escanear código").press("Enter");
  await expect(page.getByText(/Escaneado:/).first()).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #116 / #118 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("veterinary flows: hospitalization vitals, grooming board, new pet drawer", async ({ page }) => {
  await page.goto("/hospitalizacion");
  await page.getByRole("button", { name: /Registrar signos/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Guardar" }).click();
  await expect(page.getByText("Signos registrados")).toBeVisible();
  await page.goto("/grooming");
  await expect(page.getByText(/Groomer:/).first()).toBeVisible();
  await page.goto("/mascotas");
  await page.getByRole("button", { name: "Nueva mascota" }).click();
  await expect(page.getByRole("dialog").getByText("Nueva mascota")).toBeVisible();
});

// Pantalla mock inaccesible hasta que la historia #115 la rehaga sobre el DS, la conecte a la API y la habilite; entonces se reactiva.
test.skip("restaurant flows: delivery board and reservation drawer; industry dashboards", async ({ page }) => {
  await page.goto("/salon/delivery");
  await expect(page.getByText("Repartidores", { exact: true })).toBeVisible();
  await page.goto("/salon/reservas");
  await page.getByRole("button", { name: "Nueva reserva" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Guardar reserva" }).click();
  await expect(page.getByText(/Reserva creada/)).toBeVisible();
  // restaurant tenant → restaurant dashboard
  await page.goto("/seleccionar-empresa");
  await page.getByRole("button", { name: /Cevichería El Muelle/ }).click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
  await expect(page.getByText("Ventas por hora del día")).toBeVisible();
  // veterinary tenant → veterinary dashboard
  await page.goto("/seleccionar-empresa");
  await page.getByRole("button", { name: /Veterinaria/ }).first().click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
  await expect(page.getByText("Recordatorios de vacunas")).toBeVisible();
  // back to dental
  await page.goto("/seleccionar-empresa");
  await page.getByRole("button", { name: /Clínica Dental Sonrisa/ }).click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Inicio$/ })).toBeVisible();
});
