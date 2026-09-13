import { expect, test } from "@playwright/test";

test("POS adds items, computes IGV totals and emits a comprobante", async ({ page }, testInfo) => {
  await page.goto("/ventas/nueva");
  await expect(page.getByRole("heading", { level: 1, name: "Nueva venta" })).toBeVisible();
  // Tablet auto-collapses the sidebar right after hydration; wait so the layout is stable before clicking.
  if (testInfo.project.name === "tablet") {
    await expect(page.locator('[data-slot="sidebar"][data-state="collapsed"]')).toBeVisible();
  }

  // Below xl (1280px) the cart lives in a bottom sheet.
  const mobile = testInfo.project.name !== "desktop";
  const total = page.getByTestId(mobile ? "cart-total-mobile" : "cart-total");
  const add = (name: string) => page.getByRole("button", { name: `Agregar ${name}` }).click();

  await add("Curación con resina fotocurable");
  await expect(total).toHaveText("S/ 85.00");
  await add("Curación con resina fotocurable");
  await expect(total).toHaveText("S/ 170.00");
  await add("Radiografía periapical digital");
  await expect(total).toHaveText("S/ 200.00");

  if (mobile) {
    await page.getByRole("button", { name: /Ver carrito/ }).click();
  }
  const cart = mobile ? page.getByRole("dialog", { name: "Carrito" }) : page.getByRole("complementary", { name: "Carrito" });
  await expect(cart.getByTestId("cart-total")).toHaveText("S/ 200.00");
  await expect(cart.getByText("I.G.V. Oficial (18%)").locator("xpath=following-sibling::dd")).toHaveText("S/ 30.51");

  await cart.getByRole("button", { name: /Emitir comprobante/ }).click();
  await expect(page.getByText(/Comprobante B001-00012904 emitido/)).toBeVisible();
  await expect(page.getByTestId(mobile ? "cart-total-mobile" : "cart-total")).toHaveText("S/ 0.00");
});

test("POS blocks a factura without RUC", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop");
  await page.goto("/ventas/nueva");
  await page.getByRole("button", { name: "Agregar Consulta odontológica general" }).click();
  const cart = page.getByRole("complementary", { name: "Carrito" });
  await cart.getByRole("button", { name: "Factura" }).click();
  await cart.getByRole("button", { name: /Emitir comprobante/ }).click();
  await expect(page.getByText("Una factura requiere un cliente con RUC.")).toBeVisible();
});
