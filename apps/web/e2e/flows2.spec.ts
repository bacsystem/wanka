import { expect, test } from "@playwright/test";

test("agenda: new appointment drawer validates and schedules; reschedule dialog", async ({ page }, testInfo) => {
  await page.goto("/agenda");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: /Agendar nueva cita/ }).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet.getByText("María Castillo Quispe")).toBeVisible();
  await sheet.getByRole("button", { name: "10:30" }).click();
  await expect(sheet.getByText(/10:30–11:15/)).toBeVisible();
  await sheet.getByRole("button", { name: "Agendar cita" }).click();
  await expect(page.getByText(/Cita agendada/)).toBeVisible();
  if (testInfo.project.name === "desktop") {
    await page.getByRole("button", { name: /10:15 Mariana Quispe Flores/ }).click();
    await page.locator('aside[aria-label="Detalle de cita"]').getByRole("button", { name: "Reprogramar" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Reprogramar" }).click();
    await expect(page.getByText(/reprogramada al 19\/09\/2026/)).toBeVisible();
  }
});

test("purchase order editor: suggestions and totals; supplier file tabs", async ({ page }) => {
  await page.goto("/compras/ordenes/nueva");
  await expect(page.getByTestId("po-total")).toHaveText("S/ 4,407.30");
  await page.getByRole("button", { name: "+ Agregar", exact: true }).first().click();
  await expect(page.getByTestId("po-total")).toHaveText("S/ 4,832.10");
  await page.getByRole("button", { name: /Enviar orden al proveedor/ }).click();
  await expect(page.getByText(/OC-2026-0092 enviada/)).toBeVisible();
  await page.goto("/compras/proveedores/s1");
  await page.getByRole("tab", { name: /Comprobantes/ }).click();
  await expect(page.getByText("F002-011820").filter({ visible: true }).first()).toBeVisible();
});

test("GRE detail: confirm delivery updates the timeline; incident dialog", async ({ page }) => {
  await page.goto("/ventas/guias/T001-000145");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: "Reportar incidencia" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Registrar incidencia" }).click();
  await expect(page.getByText(/INC-2026-014 registrada/)).toBeVisible();
  await page.getByRole("button", { name: "Confirmar entrega" }).click();
  await expect(page.getByText(/Entrega de T001-000145 confirmada/)).toBeVisible();
  await expect(page.getByText("Confirmada por Dr. F. Zegarra · acta firmada")).toBeVisible();
});

test("invite collaborator validates email; new warehouse validates name", async ({ page }) => {
  await page.goto("/configuracion/usuarios");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: /Invitar nuevo colaborador/ }).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet).toBeVisible();
  await sheet.getByRole("button", { name: "Enviar invitación" }).click();
  await expect(page.getByText("Ingresa un correo corporativo válido")).toBeVisible();
  await sheet.getByPlaceholder("nombre@clinicasonrisa.pe").fill("rosa@clinicasonrisa.pe");
  await sheet.getByRole("button", { name: "Enviar invitación" }).click();
  await expect(page.getByText(/Invitación enviada a rosa@clinicasonrisa.pe/)).toBeVisible();
  await page.goto("/inventario/almacenes");
  await page.getByRole("button", { name: /Nuevo almacén \/ sede/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Guardar" }).click();
  await expect(page.getByText("Ingresa el nombre")).toBeVisible();
});

test("restaurant tenant: navigation changes and the floor map/KDS work", async ({ page }, testInfo) => {
  await page.goto("/seleccionar-empresa");
  await page.getByRole("button", { name: /Cevichería El Muelle/ }).click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
  await expect(page).toHaveURL(/\/$/);
  if (testInfo.project.name !== "mobile") await expect(page.getByRole("link", { name: "Salón y mesas" }).first()).toBeVisible();
  await page.goto("/salon");
  await page.getByRole("button", { name: /^M2 Ocupada/ }).click();
  const panel = testInfo.project.name === "desktop" ? page.locator('aside[aria-label="Detalle de mesa"]') : page.getByRole("dialog");
  await expect(panel.getByTestId("table-total")).toBeVisible();
  await panel.getByRole("button", { name: "Agregar plato" }).click();
  await panel.getByRole("button", { name: /Parihuela especial/ }).click();
  await expect(panel.getByTestId("table-total")).toHaveText("S/ 236.00");
  await page.goto("/salon/cocina");
  await page.getByRole("button", { name: "Iniciar preparación" }).first().click();
  await expect(page.getByRole("region", { name: "En preparación" }).getByText("#1042")).toBeVisible();
  // back to the dental tenant so later tests keep their navigation
  await page.goto("/seleccionar-empresa");
  await page.getByRole("button", { name: /Clínica Dental Sonrisa/ }).click();
  await page.getByRole("button", { name: /Continuar al panel principal/ }).click();
});

test("pet file: vaccine registration drawer and SOAP tab", async ({ page }) => {
  await page.goto("/mascotas/p1");
  await expect(page.getByText(/1 vacuna próxima a vencer/)).toBeVisible();
  await page.getByRole("button", { name: "Registrar vacuna" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Registrar" }).click();
  await expect(page.getByText(/carné actualizado/)).toBeVisible();
  await page.getByRole("tab", { name: /Historia clínica/ }).click();
  await expect(page.getByText("Nueva consulta (SOAP)")).toBeVisible();
});

test("liquidation report totals and profile password strength", async ({ page }) => {
  await page.goto("/reportes/liquidacion");
  await expect(page.getByTestId("kardex-total")).toHaveText("S/ 84,230.50");
  await page.goto("/perfil");
  const btn = page.getByRole("button", { name: "Actualizar contraseña" });
  await expect(btn).toBeDisabled();
  await page.getByLabel("Nueva contraseña", { exact: true }).fill("Sonr!sa2026Segura#");
  await expect(btn).toBeEnabled();
  await page.goto("/ruta-que-no-existe");
  await expect(page.getByRole("heading", { name: "No encontramos esta página" })).toBeVisible();
});
