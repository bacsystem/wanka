import { expect, test as base, type Page } from "@playwright/test";

/**
 * React attaches `__reactProps$…` to a DOM node only once it has hydrated it, so a control
 * without that key has no event handlers yet. Playwright's `click()` succeeds on it and the
 * click is silently lost (tabs not switching, navigation buttons doing nothing). While a
 * Suspense boundary is still re-rendering on the client, the server HTML and the client copy
 * can also coexist briefly (two inputs with the same label), so we wait until every control
 * is hydrated, which also means the stale server copy is gone.
 */
export async function waitForHydration(page: Page) {
  await page.waitForFunction(
    () => {
      const controls = document.querySelectorAll("button, a[href], input, select, textarea, [role=tab]");
      if (controls.length === 0) return false;
      return Array.from(controls).every((el) => Object.keys(el).some((k) => k.startsWith("__reactProps$")));
    },
    undefined,
    { timeout: 15_000 },
  );
}

/** Same `test`, but every full page load (goto/reload) resolves only after the page is hydrated. */
export const test = base.extend({
  page: async ({ page }, provide) => {
    const goto = page.goto.bind(page);
    const reload = page.reload.bind(page);
    page.goto = (async (...args: Parameters<Page["goto"]>) => {
      const res = await goto(...args);
      await waitForHydration(page);
      return res;
    }) as Page["goto"];
    page.reload = (async (...args: Parameters<Page["reload"]>) => {
      const res = await reload(...args);
      await waitForHydration(page);
      return res;
    }) as Page["reload"];
    await provide(page);
  },
});

export { expect };
export type { Page };
