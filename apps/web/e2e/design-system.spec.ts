import { expect, test, type Page } from "@playwright/test";

// khipu-design-system v0.5.23 palette values (dist/styles/tokens.css): default Lima in :root, Indigo and Oceano in html[data-paleta].
const LIMA = "#d4f06b";
const INDIGO = "#c9c8ff";
const OCEANO = "#9fe3dc";

const token = (page: Page, name: string) =>
  page.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);

test.describe("StatGrid mobile scroller", () => {
  for (const path of ["/inventario/stock", "/admin"]) {
    test(`${path}: KPI cards keep 176 px and the container scrolls`, async ({ page }, testInfo) => {
      test.skip(testInfo.project.name !== "mobile", "mobile contract only");
      await page.goto(path);
      const grid = page.getByRole("region", { name: "Indicadores" }).first();
      await expect(grid).toBeVisible();
      const m = await grid.evaluate((el) => ({
        scrollWidth: el.scrollWidth,
        clientWidth: el.clientWidth,
        widths: [...el.children].map((c) => c.getBoundingClientRect().width),
      }));
      expect(m.scrollWidth).toBeGreaterThan(m.clientWidth);
      expect(m.widths.length).toBeGreaterThan(1);
      for (const w of m.widths) expect(w).toBeGreaterThanOrEqual(176);
    });
  }
});

test.describe("palette", () => {
  test("the tenant app wears the default palette", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    expect(await token(page, "--acento")).toBe(LIMA);
  });

  test("the tenant app wears the palette the user saved", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("khipu-paleta", "oceano"));
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    expect(await token(page, "--acento")).toBe(OCEANO);
  });

  test("the admin console uses the design system indigo palette and the derived tokens follow it", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("khipu-paleta", "oceano"));
    await page.goto("/admin");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
    expect(await token(page, "--acento")).toBe(INDIGO);
    // The derived tokens come from the same palette, not from the default one.
    expect(await token(page, "--sidebar-primary")).toBe(INDIGO);
    // The user's saved choice is untouched.
    expect(await page.evaluate(() => localStorage.getItem("khipu-paleta"))).toBe("oceano");
  });

  test("leaving the console gives the user's palette back (client-side navigation)", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name === "mobile", "the link to the tenant app is hidden on mobile");
    await page.addInitScript(() => localStorage.setItem("khipu-paleta", "oceano"));
    await page.goto("/admin");
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
    await page.locator('a[href="/"]:visible').first().click();
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "oceano");
    expect(await token(page, "--acento")).toBe(OCEANO);
  });
});

test.describe("dark tokens", () => {
  test.use({ colorScheme: "dark" });

  test("--acento-borde and --accent-border resolve to a color (no self-reference)", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("theme", "dark"));
    await page.goto("/");
    await expect(page.locator("html")).toHaveClass(/dark/);
    expect(await token(page, "--acento-borde")).not.toBe("");
    expect(await token(page, "--accent-border")).not.toBe("");
  });
});

test("the mobile 'Nueva venta' FAB has its elevation (shadow-lg)", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "mobile only");
  await page.goto("/");
  const fab = page.locator('a[aria-label="Nueva venta"]');
  await expect(fab).toBeVisible();
  expect(await fab.evaluate((el) => getComputedStyle(el).boxShadow)).not.toBe("none");
});

test.describe("admin console palette", () => {
  test("client-side navigation between the tenant app and the console raises no console error (no <script> in the React tree)", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name === "mobile", "the link to the tenant app is hidden on mobile");
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto("/admin");
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
    await page.locator('a[href="/"]:visible').first().click();
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "lima");
    await page.goBack();
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
    expect(errors).toEqual([]);
  });

  test("the appearance menu offers the palette in the tenant app but not in the console", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Cambiar tema y apariencia" }).click();
    await expect(page.getByRole("menuitemradio", { name: "Océano" })).toBeVisible();
    await page.keyboard.press("Escape");

    await page.goto("/admin");
    await expect(page.locator("html")).toHaveAttribute("data-paleta", "indigo");
    await page.getByRole("button", { name: "Cambiar tema y apariencia" }).click();
    await expect(page.getByRole("menuitemradio", { name: "Inter" })).toBeVisible();
    await expect(page.getByRole("menuitemradio", { name: "Océano" })).toHaveCount(0);
  });
});
