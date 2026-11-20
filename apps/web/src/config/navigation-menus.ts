import type { ComandoPaleta } from "khipu-design-system/navegacion/paleta";
import type { GrupoNav, ItemNav } from "khipu-design-system/nav/sidebar-nav";
import type { Miga } from "khipu-design-system/nav/top-bar";
import {
  bottomBarFeatures,
  enabledFeatures,
  featureForPath,
  featureSectionLabel,
  type Feature,
  type FeatureSection,
} from "./navigation";

const SECTIONS: FeatureSection[] = ["core", "operaciones"];

function toItem(f: Feature): ItemNav {
  return { href: f.href, label: f.label, icon: f.icon, exacto: f.href === "/" };
}

/** Grupos del sidebar (y del cajón «Más»): una entrada por funcionalidad habilitada, sin grupos vacíos. */
export function sidebarGroups(features: readonly Feature[] = enabledFeatures): GrupoNav[] {
  return SECTIONS.map((section) => ({
    titulo: featureSectionLabel[section],
    items: features.filter((f) => (f.section ?? "core") === section).map(toItem),
  })).filter((g) => g.items.length > 0);
}

/** Entradas propias de la barra inferior móvil. */
export function bottomBarItems(features: readonly Feature[] = enabledFeatures): ItemNav[] {
  return bottomBarFeatures(features).map(toItem);
}

/** Comandos de la paleta Ctrl+K: navegar a cada funcionalidad habilitada. */
export function paletteCommands(features: readonly Feature[] = enabledFeatures): ComandoPaleta[] {
  return features.map((f) => ({
    id: f.code,
    label: f.label,
    href: f.href,
    icon: f.icon,
    grupo: "Ir a",
    detalle: f.href,
  }));
}

/** Miga de la top bar para `pathname`; vacía si la ruta no pertenece a una habilitada (p. ej. el 404). */
export function breadcrumbsFor(pathname: string, features: readonly Feature[] = enabledFeatures): Miga[] {
  const feature = featureForPath(pathname, features);
  return feature ? [{ prefijo: feature.href, seccion: "Wanka", pagina: feature.label }] : [];
}
