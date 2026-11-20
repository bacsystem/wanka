import { expect, test } from "./fixtures";

// Rutas que NO están en la lista de habilitadas de config/navigation.ts: escritas a mano, no derivadas de la lista.
const DISABLED = ["/ventas", "/ventas/nueva", "/clientes", "/clientes/u2", "/inventario/stock", "/configuracion/usuarios", "/reportes", "/perfil", "/no-existe",
  // Con punto: los segmentos dinámicos los aceptan y no deben saltarse la lista.
  "/inventario/recepcion/g.1", "/catalogo/variantes/s.k", "/clientes/abc.def", "/ventas.txt", "/api/x"];

for (const path of DISABLED) {
  test(`${path} answers 404 with the not-found page, not the mock or «Módulo en construcción»`, async ({ page }) => {
    const response = await page.goto(path);
    expect(response?.status()).toBe(404);
    await expect(page.getByRole("heading", { name: "Página no encontrada" })).toBeVisible();
    await expect(page.getByText("Módulo en construcción")).toHaveCount(0);
    await expect(page.getByText("aún no está implementado")).toHaveCount(0);
  });
}

test("a disabled route keeps the URL the user typed", async ({ page }) => {
  await page.goto("/ventas");
  await expect(page).toHaveURL(/\/ventas$/);
});

test("Ctrl+K lists only the enabled features", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "mobile", "the search shortcut is for keyboards");
  await page.goto("/");
  await page.keyboard.press("Control+k");
  const list = page.getByRole("listbox");
  await expect(list.getByRole("option")).toHaveCount(1);
  await expect(list.getByRole("option", { name: /Inicio/ })).toBeVisible();
  await page.keyboard.type("venta");
  await expect(page.getByText("Sin resultados")).toBeVisible();
});

test("the sidebar and the «Más» drawer list only the enabled features", async ({ page }, testInfo) => {
  await page.goto("/");
  if (testInfo.project.name === "mobile") {
    await page.getByRole("button", { name: "Más" }).click();
    const menu = page.getByRole("dialog").getByRole("navigation", { name: "Menú principal" });
    await expect(menu.getByRole("link")).toHaveText(["Inicio"]);
    await expect(menu.getByText(/Ventas|Clientes|Inventario|Configuración/)).toHaveCount(0);
  } else {
    const sidebar = page.getByRole("navigation", { name: "Menú principal" });
    await expect(sidebar.getByRole("link")).toHaveText(["Inicio"]);
    await expect(sidebar.getByText(/Ventas|Clientes|Inventario|Configuración/)).toHaveCount(0);
  }
});
