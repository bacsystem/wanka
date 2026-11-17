"use client";

import { CLAVE_PALETA, PALETA_POR_DEFECTO, PALETAS, valorValido } from "khipu-design-system/apariencia-opciones";
import { useEffect } from "react";

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
 * choice made in the tenant app is not touched. The inline script applies it before the first paint on a full load;
 * the effect covers client-side navigation and re-applies it if something else (the appearance menu) changes it.
 */
export function AdminPalette() {
  useEffect(() => {
    const root = document.documentElement;
    const force = () => {
      if (root.dataset.paleta !== ADMIN_PALETTE) root.dataset.paleta = ADMIN_PALETTE;
    };
    force();
    const observer = new MutationObserver(force);
    observer.observe(root, { attributes: true, attributeFilter: ["data-paleta"] });
    return () => {
      observer.disconnect();
      root.dataset.paleta = savedPalette();
    };
  }, []);

  return <script dangerouslySetInnerHTML={{ __html: `document.documentElement.dataset.paleta=${JSON.stringify(ADMIN_PALETTE)}` }} />;
}
