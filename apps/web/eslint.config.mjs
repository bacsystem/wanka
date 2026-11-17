import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const DS = "khipu-design-system";

/** Restricted imports by key, so a folder can opt out of its own key only. */
const RESTRICTED_PATHS = {
  cn: [
    { name: "cn", message: "Use `cn` from @/lib/utils (khipu-design-system, knows the DS type scale). / Usa `cn` de @/lib/utils." },
    { name: "clsx", message: "Use `cn` from @/lib/utils. / Usa `cn` de @/lib/utils." },
    { name: "tailwind-merge", message: "Use `cn` from @/lib/utils. / Usa `cn` de @/lib/utils." },
    { name: "class-variance-authority", message: `No local variants: use the ${DS} component or recipe. / Sin variantes locales: usa el componente o receta de ${DS}.` },
  ],
  icons: [
    { name: "lucide-react", message: `Use @phosphor-icons/react with PESO_ICONO* from ${DS}/lib/iconos. / Usa @phosphor-icons/react con PESO_ICONO* de ${DS}/lib/iconos.` },
  ],
  toast: [
    { name: "sonner", message: `Use \`useToast()\` from ${DS}/feedback/toast; the \`toast\` object arrives with DS-D. / Usa \`useToast()\` de ${DS}/feedback/toast; el objeto \`toast\` llega con DS-D.` },
  ],
  charts: [
    { name: "recharts", message: `Use ${DS}/datos/grafico. / Usa ${DS}/datos/grafico.` },
  ],
  themes: [
    { name: "next-themes", message: "next-themes is only allowed in components/providers. / next-themes solo se permite en components/providers." },
  ],
};

const RESTRICTED_PATTERNS = {
  // Subpaths of the banned packages (e.g. lucide-react/dynamic) are banned with the package itself.
  icons: [{ group: ["lucide-react/*"], message: RESTRICTED_PATHS.icons[0].message }],
  toast: [{ group: ["sonner/*"], message: RESTRICTED_PATHS.toast[0].message }],
  charts: [{ group: ["recharts/*"], message: RESTRICTED_PATHS.charts[0].message }],
  themes: [{ group: ["next-themes/*"], message: RESTRICTED_PATHS.themes[0].message }],
  baseUi: [
    { group: ["@base-ui/*"], message: `Import the ${DS} component, never Base UI directly. / Importa el componente de ${DS}, no Base UI directamente.` },
  ],
  // Alias and relative spellings (`@/components/ui/x`, `../../components/ui/x`).
  ui: [
    { regex: "^(@/components/|(\\.\\./)+(components/)?)ui(/|$)", message: `Local components/ui is being replaced: import from ${DS}/ui/*. / components/ui local se reemplaza: importa de ${DS}/ui/*.` },
  ],
  shared: [
    { regex: "^(@/components/|(\\.\\./)+(components/)?)shared(/|$)", message: `components/shared is a temporary adapter layer: use the ${DS} component. / components/shared es una capa temporal: usa el componente de ${DS}.` },
  ],
};

/**
 * Builds a flat-config entry with the full ban list minus the keys in `skip`.
 * `extraPaths` adds paths (e.g. the e2e fixtures rule) without redefining the base list.
 */
function restrictedImports({ files, skip = [], extraPaths = [] } = {}) {
  const pick = (table) => Object.entries(table).filter(([key]) => !skip.includes(key)).flatMap(([, v]) => v);
  return {
    ...(files ? { files } : {}),
    rules: {
      "no-restricted-imports": [
        "error",
        { paths: [...pick(RESTRICTED_PATHS), ...extraPaths], patterns: pick(RESTRICTED_PATTERNS) },
      ],
    },
  };
}

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Khipu migration guard rails (docs/frontend-migration-khipu.md section 4, ADR 0010).
  // The restricted lists are defined ONCE here; every block below derives from them, because a later
  // flat-config entry replaces the rule options instead of merging them.
  // Existing violations live in eslint-suppressions.json (`pnpm lint` passes --pass-on-unpruned-suppressions).
  // The baseline only shrinks: after each merge the tech-lead runs the serial chore
  // `eslint . --prune-suppressions` (plan section 1).
  // Not covered: dynamic `import()` and `require()` (the rule only sees static imports).
  restrictedImports(),
  // Local UI folders may import each other; the other bans still apply (baselined).
  restrictedImports({ files: ["src/components/ui/**"], skip: ["ui"] }),
  restrictedImports({ files: ["src/components/shared/**"], skip: ["shared"] }),
  // next-themes belongs to the providers only.
  restrictedImports({ files: ["src/components/providers/**"], skip: ["themes"] }),
  // E2E specs must take test/expect from ./fixtures (hydration gate), never from @playwright/test.
  restrictedImports({
    files: ["e2e/**/*.spec.ts"],
    extraPaths: [{ name: "@playwright/test", message: "Import test/expect from ./fixtures (hydration gate)." }],
  }),
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Playwright output (CI failure artifacts / local runs)
    "playwright-report/**",
    "test-results/**",
    "blob-report/**",
  ]),
]);

export default eslintConfig;
