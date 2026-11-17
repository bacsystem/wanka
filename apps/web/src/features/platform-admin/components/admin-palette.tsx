"use client";

import { useEffect } from "react";
import { lockPalette } from "@/components/layout/palette-lock";

/**
 * The console document already wears the design system's `indigo` palette (see `AppDocument` `palette`): this only
 * takes the palette choice out of the appearance menu while the console is mounted, so picking one cannot change
 * the palette the user saved for the tenant app.
 */
export function AdminPalette() {
  useEffect(() => lockPalette(), []);
  return null;
}
