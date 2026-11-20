import { expect, test as base, type Locator, type Page } from "@playwright/test";

/**
 * Hydration gate for E2E.
 *
 * React attaches an internal `__reactProps$<random>` property to a DOM node only once it has
 * hydrated it, so a control without it has no event handlers yet. Playwright's `click()` succeeds
 * on it and the click is silently lost (tabs not switching, buttons doing nothing). While a
 * Suspense boundary re-renders on the client the stale server copy can also coexist with the
 * client copy (two inputs with the same label, see issue #132); waiting until every control is
 * hydrated also waits that out.
 *
 * RISK: `__reactProps$` is a React internal, not public API. If a React upgrade renames it,
 * `waitForHydration` fails loudly with "React hydration key not found" (and `hydration.spec.ts`
 * smoke-checks it on a known page) instead of passing vacuously.
 *
 * Zero controls: a page with no interactive control has nothing to lose a click on, so
 * `waitForHydration` resolves immediately after the load event.
 *
 * Covers full page loads: `goto`, `reload`, `goBack`, `goForward`. After a navigation triggered
 * by a click that crosses root layouts (e.g. /admin/login -> /admin), call
 * `await page.waitForURL(...)` then `await waitForHydration(page)`.
 */
const CONTROLS = "button, a[href], input, select, textarea, [role=tab]";

export async function waitForHydration(page: Page, timeout = 15_000) {
  try {
    await page.waitForFunction(
      (selector) =>
        Array.from(document.querySelectorAll(selector)).every((el) =>
          Object.keys(el).some((k) => k.startsWith("__reactProps$")),
        ),
      CONTROLS,
      { timeout },
    );
  } catch (cause) {
    const info = await page.evaluate((selector) => {
      const has = (el: Element) => Object.keys(el).some((k) => k.startsWith("__reactProps$"));
      return {
        anyHydrated: Array.from(document.querySelectorAll("*")).some(has),
        unhydrated: Array.from(document.querySelectorAll(selector))
          .filter((el) => !has(el))
          .slice(0, 3)
          .map((el) => el.outerHTML.slice(0, 200)),
      };
    }, CONTROLS);
    const where = `at ${page.url()}`;
    if (!info.anyHydrated) {
      throw new Error(`React hydration key not found — React internals changed; see e2e/fixtures.ts (${where})`, { cause });
    }
    throw new Error(
      `Page not hydrated after ${timeout} ms ${where}. First unhydrated controls:\n${info.unhydrated.join("\n")}`,
      { cause },
    );
  }
}

type Nav = "goto" | "reload" | "goBack" | "goForward";

/** Same `test`, but every full page load resolves only after the page is hydrated. */
export const test = base.extend({
  page: async ({ page }, provide) => {
    for (const name of ["goto", "reload", "goBack", "goForward"] as Nav[]) {
      const original = (page[name] as (...a: unknown[]) => Promise<unknown>).bind(page);
      (page as unknown as Record<Nav, unknown>)[name] = async (...args: unknown[]) => {
        const res = await original(...args);
        await waitForHydration(page);
        return res;
      };
    }
    await provide(page);
  },
});

/**
 * Abre el menú de usuario del shell. En móvil (≤767 px) no hay sidebar: el menú vive en el cajón de «Más» de la barra inferior.
 */
export async function openUserMenu(page: Page) {
  const more = page.getByRole("button", { name: "Más" });
  if (await more.isVisible()) await more.click();
  await page.getByRole("button", { name: "Menú de usuario" }).filter({ visible: true }).click();
}

export { expect };
export type { Locator, Page };
