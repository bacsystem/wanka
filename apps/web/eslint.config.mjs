import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            { name: "cn", message: "Use `cn` from @/lib/utils (khipu-design-system, knows the DS type scale)." },
            { name: "clsx", message: "Use `cn` from @/lib/utils." },
            { name: "tailwind-merge", message: "Use `cn` from @/lib/utils." },
          ],
        },
      ],
    },
  },
  // E2E specs must take test/expect from ./fixtures (hydration gate), never from @playwright/test.
  // (Repeats the base paths: a later flat-config entry replaces the rule options.)
  {
    files: ["e2e/**/*.spec.ts"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            { name: "cn", message: "Use `cn` from @/lib/utils (khipu-design-system, knows the DS type scale)." },
            { name: "clsx", message: "Use `cn` from @/lib/utils." },
            { name: "tailwind-merge", message: "Use `cn` from @/lib/utils." },
            { name: "@playwright/test", message: "Import test/expect from ./fixtures (hydration gate)." },
          ],
        },
      ],
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
]);

export default eslintConfig;
