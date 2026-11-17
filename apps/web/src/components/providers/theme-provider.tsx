"use client";

import { IconContext, type IconProps } from "@phosphor-icons/react";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ComponentProps } from "react";
import { AparienciaProvider } from "khipu-design-system/apariencia";
import { DisenoProvider } from "khipu-design-system/diseno";
import { ProveedorMovimiento } from "khipu-design-system/movimiento";
import { PESO_ICONO } from "@/lib/iconos";

const ICONS: IconProps = { weight: PESO_ICONO };

type ThemeProviderProps = ComponentProps<typeof NextThemesProvider> & {
  /** Appearance preferences that win over the saved ones (the admin console forces its palette). */
  initialAppearance?: ComponentProps<typeof AparienciaProvider>["inicial"];
};

export function ThemeProvider({ children, initialAppearance, ...props }: ThemeProviderProps) {
  return (
    <NextThemesProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange {...props}>
      <DisenoProvider>
        <AparienciaProvider inicial={initialAppearance}>
          <IconContext.Provider value={ICONS}>
            <ProveedorMovimiento>{children}</ProveedorMovimiento>
          </IconContext.Provider>
        </AparienciaProvider>
      </DisenoProvider>
    </NextThemesProvider>
  );
}
