import { expect, test } from "./fixtures";

test("home is blank inside the DS shell", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Inicio" })).toBeVisible();
  // Sin KPIs, tarjetas ni datos de mentira: el contenido está vacío.
  await expect(page.getByRole("main")).toBeEmpty();
  await expect(page.getByRole("region", { name: "Indicadores" })).toHaveCount(0);
});

test("navigation adapts to the viewport", async ({ page }, testInfo) => {
  await page.goto("/");
  const sidebar = page.getByRole("navigation", { name: "Menú principal" });
  const bottomNav = page.getByRole("navigation", { name: "Navegación principal" });

  if (testInfo.project.name === "mobile") {
    await expect(bottomNav).toBeVisible();
    await expect(sidebar).toBeHidden();
    await expect(bottomNav.getByRole("link")).toHaveText(["Inicio"]);
    await expect(bottomNav.getByRole("button", { name: "Más" })).toBeVisible();
  } else {
    await expect(sidebar).toBeVisible();
    await expect(bottomNav).toBeHidden();
    await expect(sidebar.getByRole("link")).toHaveText(["Inicio"]);
  }

  await page.screenshot({
    path: testInfo.outputPath(`home-${testInfo.project.name}.png`),
    fullPage: true,
  });
});

test("the sidebar is 260 px wide on desktop and a 76 px rail on tablet", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "mobile");
  await page.goto("/");
  const aside = page.locator("aside").first();
  await expect(aside).toBeVisible();
  const width = (await aside.boundingBox())?.width ?? 0;
  if (testInfo.project.name === "tablet") expect(width).toBeLessThan(100);
  else expect(width).toBeGreaterThan(240);
});
