import { mkdirSync } from "node:fs";
import path from "node:path";
import { test } from "./fixtures";

/**
 * Before/after visual evidence for the khipu migration waves (W2, W4, W5a...). NOT an assertion suite.
 *
 * Opt-in: without CAPTURES=1 every test here is skipped (zero cost in the default `pnpm test:e2e`).
 * Why no `toHaveScreenshot` baselines: the migration intentionally changes pixels, so a baseline would
 * be red by design; and font/AA rendering differs between macOS (devs) and ubuntu (CI), so committed
 * PNGs would be flaky on one of them and bloat the repo (13 screens x 3 viewports x 2 themes).
 * Instead the PNGs are written to a git-ignored folder and attached to the PR body / a CI artifact.
 *
 * Usage (production build, deterministic; run BEFORE and AFTER on the same machine):
 *   pnpm build && CAPTURES=1 CAPTURES_LABEL=before E2E_PROD=1 pnpm --filter web exec playwright test ds-captures
 *   ...apply the wave, `pnpm build`...
 *   CAPTURES=1 CAPTURES_LABEL=after  E2E_PROD=1 pnpm --filter web exec playwright test ds-captures
 * Output: apps/web/captures/<label>/<project>/<theme>/<screen>.png (override the root with CAPTURES_DIR).
 * Compare (ImageMagick): magick compare -metric AE captures/before/desktop/light/home.png captures/after/desktop/light/home.png diff.png
 * Narrow the run: `--project=mobile`, `-g "dark"`, `-g "comprobantes"`.
 */
const enabled = process.env.CAPTURES === "1";
const label = process.env.CAPTURES_LABEL ?? "current";
const root = path.resolve(process.env.CAPTURES_DIR ?? "captures", label);

// Pinned "now" for anything the client renders from the clock (Monday 2026-06-15 10:00 Lima).
const FIXED_NOW = new Date("2026-06-15T15:00:00Z");

const screens: { name: string; path: string }[] = [
  { name: "home", path: "/" },
  { name: "pos", path: "/ventas/nueva" },
  { name: "comprobantes", path: "/ventas/comprobantes" },
  { name: "comprobante-detalle", path: "/ventas/comprobantes/F001-00000842" },
  { name: "clientes", path: "/clientes" },
  { name: "cliente-detalle", path: "/clientes/u2" },
  { name: "inventario-stock", path: "/inventario/stock" },
  { name: "catalogo-productos", path: "/catalogo/productos" },
  { name: "caja", path: "/finanzas/caja" },
  { name: "usuarios", path: "/configuracion/usuarios" },
  { name: "login", path: "/login" },
  { name: "admin", path: "/admin" },
  { name: "admin-empresas", path: "/admin/empresas" },
];

test.describe("ds captures", () => {
  test.skip(!enabled, "opt-in: set CAPTURES=1 (see header of e2e/ds-captures.spec.ts)");
  test.use({ reducedMotion: "reduce" });

  for (const theme of ["light", "dark"] as const) {
    test.describe(theme, () => {
      test.use({ colorScheme: theme });
      for (const s of screens) {
        test(`${s.name}`, async ({ page }, info) => {
          await page.clock.setFixedTime(FIXED_NOW);
          await page.addInitScript((t) => localStorage.setItem("theme", t), theme);
          await page.goto(s.path);
          await page.evaluate(() => document.fonts.ready);
          await page.addStyleTag({
            content:
              "*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}" +
              "::-webkit-scrollbar{display:none!important}html{scrollbar-width:none!important}",
          });
          await page.waitForLoadState("networkidle");
          // Two frames so charts (Recharts) and layout effects settle before the shot.
          await page.evaluate(() => new Promise<void>((r) => requestAnimationFrame(() => requestAnimationFrame(() => r()))));
          const dir = path.join(root, info.project.name, theme);
          mkdirSync(dir, { recursive: true });
          await page.screenshot({ path: path.join(dir, `${s.name}.png`), fullPage: true, animations: "disabled", caret: "hide" });
        });
      }
    });
  }
});
