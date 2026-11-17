"use client";

import { CLAVE_PALETA, PALETA_POR_DEFECTO, PALETAS, valorValido } from "khipu-design-system/apariencia-opciones";
import { useLayoutEffect } from "react";
import { lockPalette } from "@/components/layout/palette-lock";

/** Palette of the admin console: the design system's "indigo" (lavender accent). */
export const ADMIN_PALETTE = "indigo";

/** Palette the user chose for the tenant app (localStorage), or the default. Never the admin one forced by this component. */
function savedPalette(): string {
  try {
    return valorValido(localStorage.getItem(CLAVE_PALETA), PALETAS, PALETA_POR_DEFECTO);
  } catch {
    return PALETA_POR_DEFECTO;
  }
}

/**
 * Puts the console on the design system's `indigo` palette (`<html data-paleta="indigo">`) for as long as an admin
 * page is mounted, and gives the user's saved palette back when leaving. It does not write localStorage, so the
 * choice made in the tenant app is not touched. It re-applies the palette if something else changes it. It renders no
 * script element: React 19 reports a script created while rendering on the client (client-side navigation), so a full
 * load of /admin shows the tenant palette until hydration (layout effect, before the next paint after that).
 */
export function AdminPalette() {
  useLayoutEffect(() => {
    const root = document.documentElement;
    const force = () => {
      if (root.dataset.paleta !== ADMIN_PALETTE) root.dataset.paleta = ADMIN_PALETTE;
    };
    const unlock = lockPalette();
    force();
    const observer = new MutationObserver(force);
    observer.observe(root, { attributes: true, attributeFilter: ["data-paleta"] });
    return () => {
      observer.disconnect();
      unlock();
      root.dataset.paleta = savedPalette();
    };
  }, []);

  return null;
}
