import { TabsRuta, type TonoContadorTab } from "khipu-design-system/navegacion/tabs-ruta";

export interface SectionNavItem {
  href: string;
  label: string;
  /** Short counter shown after the label ("12", "SUNAT 13.1"). */
  badge?: string;
  /** Colour of the counter. */
  badgeTone?: TonoContadorTab;
}

interface SectionNavProps {
  items: SectionNavItem[];
  "aria-label": string;
}

/**
 * @deprecated W8: use `TabsRuta` from `khipu-design-system/navegacion/tabs-ruta` directly.
 * Route-based tabs between sibling screens (compras, inventario…). The DS marks the current one from `usePathname`;
 * every item matches its exact route (the first tab of a section is the section root).
 */
export function SectionNav({ items, "aria-label": etiqueta }: SectionNavProps) {
  return <TabsRuta etiqueta={etiqueta} items={items.map((t) => ({ href: t.href, etiqueta: t.label, contador: t.badge, tonoContador: t.badgeTone, exacto: true }))} />;
}
