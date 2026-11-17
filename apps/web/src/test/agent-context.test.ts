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

/**
 * Mirrors Gemini CLI's `findImports` (packages/core/src/utils/memoryImportProcessor.ts): an `@` at the
 * start of the text or right after whitespace, followed by a path up to the next whitespace whose first
 * character is `.`, `/` or an ASCII letter, outside code regions (any backtick run, inline or fenced).
 */
export function geminiImportPaths(text: string): string[] {
  const code: Array<[number, number]> = [];
  for (const m of text.matchAll(/(`+)([\s\S]*?)\1/g)) code.push([m.index, m.index + m[0].length]);
  const inCode = (i: number) => code.some(([a, b]) => i >= a && i < b);
  const paths: string[] = [];
  for (let i = text.indexOf("@"); i !== -1; i = text.indexOf("@", i + 1)) {
    if (i > 0 && !/\s/.test(text[i - 1])) continue;
    if (inCode(i)) continue;
    const path = text.slice(i + 1).match(/^\S+/)?.[0] ?? "";
    if (path && (path[0] === "." || path[0] === "/" || /[A-Za-z]/.test(path[0]))) paths.push(path);
  }
  return paths;
}

function geminiImports(file: string) {
  return geminiImportPaths(readFileSync(file, "utf8")).map((p) => resolve(dirname(file), p));
}

/** Every file Gemini CLI reaches from `entry` by following imports recursively (max depth 5, like Gemini). */
function geminiImportGraph(entry: string, depth = 0, out = new Map<string, string[]>()) {
  if (out.has(entry) || depth > 5 || !existsSync(entry)) return out;
  const targets = geminiImports(entry);
  out.set(entry, targets);
  for (const t of targets) geminiImportGraph(t, depth + 1, out);
  return out;
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

  it("every Gemini CLI import reachable from the context files (CLAUDE.md included) resolves to an existing file", () => {
    for (const entry of [rootGemini, resolve(REPO_ROOT, "apps/web/GEMINI.md")]) {
      const graph = geminiImportGraph(entry);
      if (entry === rootGemini) expect([...graph.keys()]).toContain(resolve(REPO_ROOT, "CLAUDE.md"));
      for (const [file, targets] of graph)
        for (const target of targets) expect(existsSync(target), `unresolved Gemini import in ${file}: ${target}`).toBe(true);
    }
  });
});
