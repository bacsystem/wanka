"use client";

import { IconContext, type IconProps } from "@phosphor-icons/react";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ComponentProps } from "react";
import { AppearanceProvider } from "@/components/providers/appearance-provider";
import { MotionProvider } from "@/components/providers/motion-provider";
import { ICON_WEIGHT } from "@/lib/icons";

const ICONS: IconProps = { weight: ICON_WEIGHT };

export function ThemeProvider({ children, ...props }: ComponentProps<typeof NextThemesProvider>) {
  return (
    <NextThemesProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange {...props}>
      <AppearanceProvider>
        <IconContext.Provider value={ICONS}>
          <MotionProvider>{children}</MotionProvider>
        </IconContext.Provider>
      </AppearanceProvider>
    </NextThemesProvider>
  );
}