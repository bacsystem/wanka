import { expect, test } from "@playwright/test";

test("POS 'Agregar' buttons are legible (text color differs from background)", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "desktop only");
  await page.goto("/ventas/nueva");
  const button = page.getByRole("button", { name: /^Agregar / }).first();
  await expect(button).toBeVisible();
  const colors = await button.evaluate((el) => {
    const s = getComputedStyle(el);
    return { color: s.color, background: s.backgroundColor };
  });
  expect(colors.color).not.toBe(colors.background);
});
