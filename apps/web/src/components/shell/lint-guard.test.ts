// @vitest-environment node
import path from "node:path";
import { ESLint } from "eslint";
import { describe, expect, it } from "vitest";

const root = path.resolve(__dirname, "../../..");
const eslint = new ESLint({ cwd: root });

async function messagesFor(code: string, file = "src/components/shell/probe.tsx") {
  const [result] = await eslint.lintText(code, { filePath: path.join(root, file) });
  return result.messages.filter((m) => m.ruleId === "no-restricted-imports").map((m) => m.message);
}

describe("el código nuevo no usa las capas locales", () => {
  it.each([
    ["components/ui", 'import { Button } from "@/components/ui/button";'],
    ["DataTable local", 'import { DataTable } from "@/components/shared/data-table";'],
    ["shell local", 'import { AppShell } from "@/components/layout/app-shell";'],
    ["shell local relativo", 'import { AppShell } from "../layout/app-shell";'],
  ])("rechaza importar %s", async (_name, code) => {
    expect(await messagesFor(code)).toHaveLength(1);
  });

  it("permite los componentes del DS y el propio shell nuevo", async () => {
    const code = 'import { Shell } from "khipu-design-system/nav/shell";\nimport { AppShell } from "@/components/shell/app-shell";';
    expect(await messagesFor(code)).toEqual([]);
  });
});
