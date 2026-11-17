import type { Metadata } from "next";
import { ScriptDiseno } from "khipu-design-system/script-diseno";
import type { ReactNode } from "react";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { fonts } from "@/app/fonts";
import "@/app/globals.css";

export const rootMetadata: Metadata = {
  title: { default: "Wanka", template: "%s · Wanka" },
  description: "Gestión empresarial multirubro con facturación electrónica SUNAT",
};

/**
 * The `<html>` document shared by every root layout (tenant app, admin console, admin login): fonts, anti-flash
 * appearance script, theme and appearance providers, tooltips and toasts.
 *
 * `palette` forces a design system palette for the whole document (the admin console uses "indigo"). It is rendered
 * as `data-paleta` in the server HTML and passed as `inicial` to the script and the provider, so the first paint
 * already has it and nothing is written to localStorage (the user's saved palette for the tenant app is untouched).
 */
export function AppDocument({ children, palette }: { children: ReactNode; palette?: string }) {
  const initial = palette ? { paleta: palette } : undefined;
  return (
    <html lang="es-PE" suppressHydrationWarning data-paleta={palette} className={`${fonts} h-full antialiased`}>
      <head>
        <ScriptDiseno inicial={initial} />
      </head>
      <body className="flex min-h-full flex-col">
        <ThemeProvider initialAppearance={initial}>
          <TooltipProvider>{children}</TooltipProvider>
          <Toaster position="top-right" richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
