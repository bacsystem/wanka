import { expect, test, type Locator } from "./fixtures";

type Rgba = [number, number, number, number];

/** Text color and effective background (alpha-composited up through transparent ancestors), as 0-255 channels. */
async function colorsOf(locator: Locator) {
  return locator.evaluate((el) => {
    const parse = (value: string): number[] => {
      const m = value.match(/^color\(srgb ([\d.e-]+) ([\d.e-]+) ([\d.e-]+)(?: \/ ([\d.%e-]+))?\)$/);
      if (m) {
        const a = m[4] === undefined ? 1 : m[4].endsWith("%") ? parseFloat(m[4]) / 100 : parseFloat(m[4]);
        return [parseFloat(m[1]) * 255, parseFloat(m[2]) * 255, parseFloat(m[3]) * 255, a];
      }
      const n = (value.match(/[\d.]+/g) ?? []).map(Number);
      return [n[0], n[1], n[2], n[3] ?? 1];
    };
    const layers: number[][] = [];
    for (let node: Element | null = el; node; node = node.parentElement) {
      const bg = parse(getComputedStyle(node).backgroundColor);
      if (bg[3] > 0) layers.push(bg);
      if (bg[3] === 1) break;
    }
    let base = [255, 255, 255];
    for (const [r, g, b, a] of layers.reverse()) {
      base = [r * a + base[0] * (1 - a), g * a + base[1] * (1 - a), b * a + base[2] * (1 - a)];
    }
    const fg = parse(getComputedStyle(el).color);
    const text = [0, 1, 2].map((i) => fg[i] * fg[3] + base[i] * (1 - fg[3]));
    return { text: [...text, 1] as Rgba, background: [...base, 1] as Rgba };
  });
}

function luminance([r, g, b]: Rgba) {
  const lin = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: Rgba, b: Rgba) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const BUTTONS = [
  { route: "/ventas/nueva", name: /^Agregar / },
  { route: "/ventas/proformas", name: /Facturar 1-clic/ },
  { route: "/salon", name: /Abrir mesa/, tenant: "t_004" },
  { route: "/admin", name: /Ver incidentes/ },
];

for (const theme of ["light", "dark"] as const) {
  for (const { route, name, tenant } of BUTTONS) {
    test(`${theme}: ${name} on ${route} has WCAG contrast >= 4.5`, async ({ page, context }, testInfo) => {
      test.skip(testInfo.project.name !== "desktop", "desktop only");
      await page.emulateMedia({ colorScheme: theme });
      await page.addInitScript((t) => localStorage.setItem("theme", t), theme);
      if (tenant) {
        await context.addCookies([{ name: "tenant_id", value: tenant, url: String(testInfo.project.use.baseURL) }]);
      }
      await page.goto(route);
      await expect(page.locator("html")).toHaveClass(theme === "dark" ? /dark/ : /^(?!.*dark)/);
      const button = page.getByRole("button", { name }).or(page.getByRole("link", { name })).first();
      await expect(button).toBeVisible();
      const { text, background } = await colorsOf(button);
      expect(contrast(text, background)).toBeGreaterThanOrEqual(4.5);
    });
  }
}

const DOT_STATUSES = ["libre", "ocupada", "por_cobrar", "reservada", "sucia"];
const MODES = [
  { name: "light", theme: "light", oscuro: "" },
  { name: "dark", theme: "dark", oscuro: "" },
  { name: "dark grafito", theme: "dark", oscuro: "grafito" },
  { name: "dark tintado", theme: "dark", oscuro: "tintado" },
] as const;

for (const mode of MODES) {
  test(`${mode.name}: every /salon table status dot has WCAG contrast >= 4.5`, async ({ page, context }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop", "desktop only");
    await page.emulateMedia({ colorScheme: mode.theme });
    await page.addInitScript((t) => localStorage.setItem("theme", t), mode.theme);
    await context.addCookies([{ name: "tenant_id", value: "t_004", url: String(testInfo.project.use.baseURL) }]);
    await page.goto("/salon");
    await expect(page.locator("html")).toHaveClass(mode.theme === "dark" ? /dark/ : /^(?!.*dark)/);
    if (mode.oscuro) await page.evaluate((o) => document.documentElement.setAttribute("data-oscuro", o), mode.oscuro);
    for (const status of DOT_STATUSES) {
      const dot = page.locator(`[data-table-dot="${status}"]`).first();
      await expect(dot, `dot ${status}`).toBeVisible();
      const { text, background } = await colorsOf(dot);
      expect(contrast(text, background), `dot ${status}`).toBeGreaterThanOrEqual(4.5);
    }
  });
}
