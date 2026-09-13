"use client";

import { getNavigation, isActivePath, navSectionLabel, type NavSection } from "@/config/navigation";
import { SidebarSections } from "./shell-parts";
import { useTenant } from "./tenant-context";

export function SidebarNav() {
  const navigation = getNavigation(useTenant().industry);
  const sections = (["core", "operaciones"] as NavSection[]).map((id) => ({ id, label: navSectionLabel[id], items: navigation.filter((i) => (i.section ?? "core") === id) }));
  return <SidebarSections sections={sections} isActive={isActivePath} />;
}
