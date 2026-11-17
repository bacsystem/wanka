import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const HARDCODED_PALETTE = /\b(?:bg|text|border)-(?:emerald|amber|rose|blue|indigo|teal)-\d+/;

function collectTsx(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === "mocks" ? [] : collectTsx(full);
    return name.endsWith(".tsx") && !name.endsWith(".test.tsx") ? [full] : [];
  });
}

describe("Design System khipu 0.5 guardrails", () => {
  it("ningún componente o página TSX usa colores de paleta hardcodeados", () => {
    const root = resolve(__dirname, "..");
    const violations = collectTsx(root).filter((file) =>
      HARDCODED_PALETTE.test(readFileSync(file, "utf8")),
    );
    expect(violations.map((f) => relative(root, f))).toEqual([]);
  });

  it("tokens.css, theme.css y shadcn.css están importados en globals.css", () => {
    const globals = readFileSync(resolve(__dirname, "../app/globals.css"), "utf8");
    expect(globals).toContain("../styles/tokens.css");
    expect(globals).toContain("../styles/theme.css");
    expect(globals).toContain("../styles/shadcn.css");
  });
});
