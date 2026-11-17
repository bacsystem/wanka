import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Agent context files (CLAUDE.md "Agent context files"): GEMINI.md only imports CLAUDE.md.
 * Antigravity inlines `@[label](path)` and truncates a rule file above 24 000 bytes after
 * expansion (https://antigravity.google/docs/rules), silently dropping the tail of CLAUDE.md.
 * This test keeps a margin: the expanded root GEMINI.md must stay at or below 22 000 bytes.
 */
const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../../../..");
export const ANTIGRAVITY_BUDGET_BYTES = 22_000;

const INCLUDE = /@\[[^\]]*\]\(([^)\s]+)\)/g; // Antigravity inline include
const GEMINI_IMPORT = /^@(\.{1,2}\/\S+|\/\S+)\s*$/gm; // Gemini CLI import on its own line

function stripFrontmatter(text: string) {
  return text.startsWith("---\n") ? text.replace(/^---\n[\s\S]*?\n---\n/, "") : text;
}

/** Expands Antigravity `@[label](path)` includes recursively, relative to each file. */
export function expandAntigravity(file: string, seen = new Set<string>()): string {
  if (seen.has(file)) throw new Error(`circular include: ${file}`);
  seen.add(file);
  const text = stripFrontmatter(readFileSync(file, "utf8"));
  return text.replace(INCLUDE, (_m, rel: string) => {
    const target = resolve(dirname(file), rel);
    if (!existsSync(target)) throw new Error(`missing include ${rel} in ${file}`);
    return expandAntigravity(target, new Set(seen));
  });
}

function geminiImports(file: string) {
  return [...readFileSync(file, "utf8").matchAll(GEMINI_IMPORT)].map((m) => resolve(dirname(file), m[1]));
}

describe("agent context files", () => {
  const rootGemini = resolve(REPO_ROOT, "GEMINI.md");

  it("root GEMINI.md expands under the Antigravity budget", () => {
    const bytes = Buffer.byteLength(expandAntigravity(rootGemini), "utf8");
    console.info(`[agent-context] expanded root GEMINI.md: ${bytes} bytes (budget ${ANTIGRAVITY_BUDGET_BYTES}, Antigravity cap 24000)`);
    expect(bytes).toBeLessThanOrEqual(ANTIGRAVITY_BUDGET_BYTES);
  });

  it("root GEMINI.md imports CLAUDE.md for both tools", () => {
    expect(geminiImports(rootGemini)).toEqual([resolve(REPO_ROOT, "CLAUDE.md")]);
    expect(expandAntigravity(rootGemini)).toContain(readFileSync(resolve(REPO_ROOT, "CLAUDE.md"), "utf8"));
  });

  it("CLAUDE.md contains no Antigravity include syntax (it would be expanded inside the context)", () => {
    expect(readFileSync(resolve(REPO_ROOT, "CLAUDE.md"), "utf8").match(INCLUDE)).toBeNull();
  });

  it("every Gemini CLI import in the context files points to an existing file", () => {
    for (const file of [rootGemini, resolve(REPO_ROOT, "apps/web/GEMINI.md")]) {
      for (const target of geminiImports(file)) expect(existsSync(target), `${file} -> ${target}`).toBe(true);
    }
  });
});
