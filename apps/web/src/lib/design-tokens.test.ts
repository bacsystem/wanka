import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(__dirname, "..");
const read = (rel: string) => readFileSync(resolve(SRC, rel), "utf8");
// The tokens and the Tailwind theme come from the pinned khipu-design-system package, not from this repo.
const DS = resolve(SRC, "../node_modules/khipu-design-system/dist/styles");
const tokens = readFileSync(resolve(DS, "tokens.css"), "utf8");
const theme = readFileSync(resolve(DS, "theme.css"), "utf8");
const globals = read("app/globals.css");

/** Body of the `@theme inline { ... }` blocks (Tailwind-only mapping, where `--x: var(--x)` is legitimate). */
function themeBlocks(css: string): string {
  return [...css.matchAll(/@theme inline\s*\{([\s\S]*?)\n\}/g)].map((m) => m[1]).join("\n");
}
const withoutThemeBlocks = (css: string) => css.replace(/@theme inline\s*\{[\s\S]*?\n\}/g, "");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const plainCss = stripComments([tokens, withoutThemeBlocks(theme), withoutThemeBlocks(globals)].join("\n"));
const themeCss = stripComments([themeBlocks(theme), themeBlocks(globals)].join("\n"));

const declarations = (css: string) =>
  [...css.matchAll(/(--[a-z0-9-]+)\s*:\s*([^;]+);/g)].map((m) => ({ name: m[1], value: m[2] }));

// Set by next/font in app/layout.tsx or by html[data-font]; not declared as plain tokens.
const RUNTIME_VARS = new Set(["--i", "--font-sans", "--font-mono", "--fuente-jakarta", "--fuente-manrope", "--fuente-dmsans", "--fuente-figtree"]);
const declared = new Set([...declarations(plainCss).map((d) => d.name), ...RUNTIME_VARS]);

function collectTsx(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return collectTsx(full);
    return name.endsWith(".tsx") && !name.endsWith(".test.tsx") ? [full] : [];
  });
}

function missingRefs(css: string): string[] {
  return declarations(css).flatMap((d) =>
    [...d.value.matchAll(/var\((--[a-z0-9-]+)/g)]
      .map((m) => m[1])
      .filter((v) => !declared.has(v))
      .map((v) => `${d.name} -> ${v}`),
  );
}

describe("design tokens resolve", () => {
  it("no custom property references itself outside @theme", () => {
    const selfRefs = declarations(plainCss).filter((d) => new RegExp(`var\\(${d.name}[,)]`).test(d.value));
    expect(selfRefs.map((d) => `${d.name}: ${d.value}`)).toEqual([]);
  });

  it("every var() in the @theme mapping points to a declared variable", () => {
    expect(missingRefs(themeCss)).toEqual([]);
  });

  it("every var() used in plain CSS points to a declared variable", () => {
    expect(missingRefs(plainCss)).toEqual([]);
  });

  it("every khipu color/shadow utility used in TSX has a --color-* / --shadow-* token", () => {
    const colors = new Set(
      declarations(themeCss)
        .filter((d) => d.name.startsWith("--color-"))
        .map((d) => d.name.slice(8)),
    );
    const shadows = new Set(
      declarations(themeCss)
        .filter((d) => d.name.startsWith("--shadow-"))
        .map((d) => d.name.slice(9)),
    );
    const builtinShadows = new Set(["none", "inner", "2xs", "xl", "2xl"]);
    const FAMILY =
      /^(accent|acento|sidebar|success|warning|destructive|info|chart|brand|code|card|popover|primary|secondary|muted|background|foreground|input|ring|overlay|texto-terciario|borde-tarjeta|codigo)(-|$)/;
    const UTILITY = /(?<![\w-])(bg|text|border|ring|fill|stroke|from|to|via|outline|divide|decoration|placeholder|caret|shadow)-([a-z][a-z0-9-]*)/g;

    const bad = new Set<string>();
    for (const file of collectTsx(SRC)) {
      for (const m of readFileSync(file, "utf8").matchAll(UTILITY)) {
        const [, prefix, rawName] = m;
        if (prefix === "shadow") {
          if (!shadows.has(rawName) && !builtinShadows.has(rawName)) bad.add(`${relative(SRC, file)}: shadow-${rawName}`);
          continue;
        }
        const name = prefix === "border" || prefix === "divide" ? rawName.replace(/^[tblrxyse]-/, "") : rawName;
        if (FAMILY.test(name) && !colors.has(name)) bad.add(`${relative(SRC, file)}: ${prefix}-${rawName}`);
      }
    }
    expect([...bad]).toEqual([]);
  });
});
