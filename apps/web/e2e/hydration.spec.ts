import { expect, test } from "./fixtures";

test("smoke: React still marks hydrated nodes with __reactProps$ (see e2e/fixtures.ts)", async ({ page }) => {
  await page.goto("/login");
  const found = await page.evaluate(() =>
    Array.from(document.querySelectorAll("button, input, a[href]")).some((el) =>
      Object.keys(el).some((k) => k.startsWith("__reactProps$")),
    ),
  );
  expect(found, "React hydration key not found — React internals changed; see e2e/fixtures.ts").toBe(true);
});
