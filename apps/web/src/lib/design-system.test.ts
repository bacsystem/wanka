import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const ALL_PALETTES =
  "slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose";
const HARDCODED_PALETTE = new RegExp(`\\b(?:bg|text|border|ring|fill|from|to|via|stroke)-(?:${ALL_PALETTES})-\\d+`);

function collectTsx(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === "mocks" ? [] : collectTsx(full);
    return name.endsWith(".tsx") && !name.endsWith(".test.tsx") ? [full] : [];
  });
}

describe("Design System khipu 0.5 guardrails", () => {
  it("ningún componente o página TSX usa colores de paleta Tailwind hardcodeados", () => {
    const root = resolve(__dirname, "..");
    const violations = collectTsx(root).filter((file) => HARDCODED_PALETTE.test(readFileSync(file, "utf8")));
    expect(violations.map((f) => relative(root, f))).toEqual([]);
  });

  it("globals.css importa shadcn/tailwind.css, tokens.css y theme.css y redeclara derivados en .theme-admin", () => {
    const globals = readFileSync(resolve(__dirname, "../app/globals.css"), "utf8");
    expect(globals).toContain('@import "shadcn/tailwind.css";');
    expect(globals).toContain("../styles/tokens.css");
    expect(globals).toContain("../styles/theme.css");
    expect(globals).toMatch(/\.theme-admin[\s\S]*--sidebar-primary:/);
    expect(globals).toMatch(/\.dark \.theme-admin[\s\S]*--accent-soft:/);
  });

  it("todos los controles base usan h-control (36 px) y su variante sm es menor (h-7, 28 px)", () => {
    const theme = readFileSync(resolve(__dirname, "../styles/theme.css"), "utf8");
    expect(theme).toContain("--spacing-control: 2.25rem;");

    const btn = readFileSync(resolve(__dirname, "../components/ui/button.tsx"), "utf8");
    const inp = readFileSync(resolve(__dirname, "../components/ui/input.tsx"), "utf8");
    const sel = readFileSync(resolve(__dirname, "../components/ui/select.tsx"), "utf8");
    expect(btn).toContain("h-control");
    expect(btn).toContain("sm: \"h-7");
    expect(inp).toContain("h-control");
    expect(inp).toContain('"h-7"');
    expect(sel).toContain("data-[size=default]:h-control");
    expect(sel).toContain("data-[size=sm]:h-7");
  });
});