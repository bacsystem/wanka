"use client";

import * as React from "react";
import { useSidebar } from "@/components/ui/sidebar";

const COLLAPSE_QUERY = "(max-width: 1279px)";

/**
 * Tablet range (768–1279px) starts with the sidebar collapsed to icons,
 * unless the user already chose a state (persisted in the sidebar cookie).
 */
export function SidebarAutoCollapse({ hasStoredState }: { hasStoredState: boolean }) {
  const { setOpen } = useSidebar();

  React.useEffect(() => {
    if (hasStoredState) return;
    const mql = window.matchMedia(COLLAPSE_QUERY);
    const apply = () => setOpen(!mql.matches);
    apply();
    mql.addEventListener("change", apply);
    return () => mql.removeEventListener("change", apply);
  }, [hasStoredState, setOpen]);

  return null;
}
