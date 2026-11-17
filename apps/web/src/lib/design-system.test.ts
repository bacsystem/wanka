import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(__dirname, "..");
const PKG = resolve(SRC, "../node_modules/khipu-design-system/dist");
const ALL_PALETTES =
  "slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose";
const HARDCODED_PALETTE = new RegExp(`\\b(?:bg|text|border|ring|fill|from|to|via|stroke)-(?:${ALL_PALETTES})-\\d+`);

const ARBITRARY_COLOR =
  /\b(?:bg|text|border|ring|fill|stroke|from|to|via|outline|decoration|shadow)-\[(?:#|rgba?\(|hsla?\(|hwb\(|lab\(|lch\(|oklab\(|oklch\(|color\()/;
const HEX_LITERAL = /#[0-9a-fA-F]{3,8}\b/;
const WHITE_BLACK = /\b(?:bg|text|border|ring|fill|stroke|from|to|via)-(?:white|black)\b/;

// Pre-existing literals, out of scope for the khipu adoption PR: migración W-n (docs/frontend-migration-khipu.md).
// Any NEW file with a hex literal or bg/text-white|black fails the guard.
const HEX_ALLOWLIST = ["features/platform-admin/components/monitoring-screen.tsx", "components/ui/chart.tsx"];
const WHITE_BLACK_ALLOWLIST = [
  "features/settings/components/sunat-settings.tsx",
  "components/ui/sheet.tsx",
  "components/ui/dialog.tsx",
  "components/shared/status-badge.tsx",
];

function collectTsx(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === "mocks" ? [] : collectTsx(full);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [full] : [];
  });
}

describe("khipu design system adoption", () => {
  it("no TSX component or page hardcodes a Tailwind palette color", () => {
    const violations = collectTsx(SRC).filter((file) => HARDCODED_PALETTE.test(readFileSync(file, "utf8")));
    expect(violations.map((f) => relative(SRC, f))).toEqual([]);
  });

  it("no TSX component or page uses an arbitrary color value (use DS tokens)", () => {
    const violations = collectTsx(SRC).filter((file) => ARBITRARY_COLOR.test(readFileSync(file, "utf8")));
    expect(violations.map((f) => relative(SRC, f))).toEqual([]);
  });

  it("no source file adds a hex color literal or bg/text-white|black (allow-list = pending migration)", () => {
    const rel = (f: string) => relative(SRC, f);
    const read = (f: string) => readFileSync(f, "utf8");
    const files = collectTsx(SRC);
    expect(files.filter((f) => HEX_LITERAL.test(read(f)) && !HEX_ALLOWLIST.includes(rel(f))).map(rel)).toEqual([]);
    expect(files.filter((f) => WHITE_BLACK.test(read(f)) && !WHITE_BLACK_ALLOWLIST.includes(rel(f))).map(rel)).toEqual([]);
  });

  it("globals.css imports the package styles and does not redefine its tokens", () => {
    const globals = readFileSync(resolve(SRC, "app/globals.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
    expect(globals).toContain('@import "khipu-design-system/styles.css";');
    // A bare :root/.dark override would beat the palette the user chose (same specificity, last one wins).
    const declaredInRoot = [...globals.matchAll(/(?:^|\n)\s*(?:html)?:root\s*\{([^}]*)\}/g)].flatMap((m) =>
      [...m[1].matchAll(/(--[a-z0-9-]+)\s*:/g)].map((d) => d[1]),
    );
    expect(declaredInRoot).toEqual(["--brand-panel"]);
    expect(globals).not.toMatch(/(^|\n)\s*(html)?\.dark\s*\{/);
    expect(globals).not.toContain("theme-admin");
  });

  it("the vendored tokens.css / theme.css copies are gone", () => {
    expect(() => readFileSync(resolve(SRC, "styles/tokens.css"))).toThrow();
    expect(() => readFileSync(resolve(SRC, "styles/theme.css"))).toThrow();
  });

  it("controls are h-control and the package defines it as 32 px", () => {
    expect(readFileSync(resolve(PKG, "styles/theme.css"), "utf8")).toContain("--spacing-control: 2rem;");
    for (const file of ["button", "input", "select"]) {
      const source = readFileSync(resolve(SRC, `components/ui/${file}.tsx`), "utf8");
      expect(source).toContain("h-control");
      expect(source).not.toMatch(/(?<![\w-])h-(?:7|8|9|10|11)(?![\w-])/);
    }
  });
});

describe("admin console palette", () => {
  const options = readFileSync(resolve(PKG, "components/apariencia-opciones.js"), "utf8");
  const tokens = readFileSync(resolve(PKG, "styles/tokens.css"), "utf8");
  const read = (rel: string) => readFileSync(resolve(SRC, rel), "utf8");
  const id = read("features/platform-admin/components/admin-palette-id.ts");

  it("is the design system's indigo palette, which defines the five accents", () => {
    expect(id).toContain('ADMIN_PALETTE = "indigo"');
    expect(options).toContain('valor: "indigo"');
    const block = tokens.match(/html:root\[data-paleta="indigo"\]\s*\{([^}]*)\}/)?.[1] ?? "";
    for (const token of ["--acento", "--acento-texto", "--acento-2", "--acento-2-texto", "--acento-3"]) {
      expect(block).toContain(`${token}:`);
    }
  });

  it("the console and its login are their own root layouts with the palette in the server HTML", () => {
    for (const file of ["app/(admin)/layout.tsx", "app/(admin-auth)/layout.tsx"]) {
      const layout = read(file);
      expect(layout, file).toContain("<AppDocument palette={ADMIN_PALETTE}>");
      expect(layout, file).toContain("<AdminPalette />");
    }
    expect(read("components/providers/app-document.tsx")).toContain("data-paleta={palette}");
    // The tenant app and its auth pages have another root layout; there is no top-level app/layout.tsx.
    expect(read("app/(tenant)/layout.tsx")).toContain("<AppDocument>");
    expect(() => readFileSync(resolve(SRC, "app/layout.tsx"))).toThrow();
  });

  it("forcing the palette never writes the user's saved one", () => {
    for (const file of ["features/platform-admin/components/admin-palette.tsx", "components/providers/app-document.tsx"]) {
      expect(read(file), file).not.toContain("setItem");
    }
  });

  it("renders no <script>: React 19 reports one created during client-side navigation", () => {
    for (const file of collectTsx(resolve(SRC, "features/platform-admin")).concat(collectTsx(resolve(SRC, "app/(admin)")), collectTsx(resolve(SRC, "app/(admin-auth)")))) {
      expect(readFileSync(file, "utf8"), relative(SRC, file)).not.toMatch(/<script[\s>]/);
    }
  });

  it("the appearance menu hides the palette group while the palette is forced", () => {
    const toggle = readFileSync(resolve(SRC, "components/layout/theme-toggle.tsx"), "utf8");
    expect(toggle).toContain("usePaletteLocked()");
    expect(toggle).toContain("!paletteLocked");
  });

  it("the local .theme-admin overrides no longer exist", () => {
    const offenders = collectTsx(SRC).filter((f) => readFileSync(f, "utf8").includes("theme-admin"));
    expect(offenders.map((f) => relative(SRC, f))).toEqual([]);
  });
});
