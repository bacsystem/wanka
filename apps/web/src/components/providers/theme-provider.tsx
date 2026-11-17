"use client";

import { IconContext, type IconProps } from "@phosphor-icons/react";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ComponentProps } from "react";
import { AparienciaProvider } from "@/components/apariencia";
import { DisenoProvider } from "@/components/diseno";
import { ProveedorMovimiento } from "@/components/movimiento";
import { PESO_ICONO } from "@/lib/iconos";

const ICONOS: IconProps = { weight: PESO_ICONO };

export function ThemeProvider({ children, ...props }: ComponentProps<typeof NextThemesProvider>) {
  return (
    <NextThemesProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange {...props}>
      <DisenoProvider>
        <AparienciaProvider>
          <IconContext.Provider value={ICONOS}>
            <ProveedorMovimiento>{children}</ProveedorMovimiento>
          </IconContext.Provider>
        </AparienciaProvider>
      </DisenoProvider>
    </NextThemesProvider>
  );
}
