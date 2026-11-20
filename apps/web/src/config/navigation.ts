import { SquaresFourIcon } from "@phosphor-icons/react/ssr";
import type { IconoComponente } from "@/lib/iconos";

/** Una funcionalidad habilitada: terminada de punta a punta contra la API real. */
export interface Feature {
  /** Código estable de la funcionalidad (no se muestra). */
  code: string;
  label: string;
  /** Ruta base. `/` solo coincide exacta; el resto, también con sus subrutas (`/clientes` → `/clientes/123`). */
  href: string;
  icon: IconoComponente;
  /** Grupo del sidebar; por defecto, `core`. */
  section?: FeatureSection;
}
export type FeatureSection = "core" | "operaciones";
export const featureSectionLabel: Record<FeatureSection, string> = { core: "Módulos core", operaciones: "Operaciones" };

/**
 * Lista de funcionalidades habilitadas: ÚNICA fuente del sidebar, la barra inferior, la paleta Ctrl+K y las rutas.
 * Lo que no está aquí no se muestra y su URL responde 404 (`proxy.ts`).
 *
 * Para habilitar una funcionalidad, la historia que la entrega (pantalla sobre el design system, conectada a la API
 * real) agrega su entrada a esta lista. Nunca se comenta código para ocultar algo.
 * Las pantallas mock anteriores siguen en `features/*` como referencia y su navegación en `legacy-navigation.ts`.
 */
export const enabledFeatures: readonly Feature[] = [
  { code: "home", label: "Inicio", href: "/", icon: SquaresFourIcon },
];

/** Si `pathname` pertenece a alguna funcionalidad de `features`. */
export function isEnabledPath(pathname: string, features: readonly Feature[] = enabledFeatures): boolean {
  return features.some((f) => (f.href === "/" ? pathname === "/" : pathname === f.href || pathname.startsWith(`${f.href}/`)));
}

/** La funcionalidad más específica (prefijo más largo) que contiene `pathname`. */
export function featureForPath(pathname: string, features: readonly Feature[] = enabledFeatures): Feature | undefined {
  return features
    .filter((f) => isEnabledPath(pathname, [f]))
    .sort((a, b) => b.href.length - a.href.length)[0];
}

/** Máximo de entradas propias de la barra inferior (sin «Más»), el tope de `BarraInferior`. */
export const BOTTOM_BAR_LIMIT = 4;

/** Entradas de la barra inferior: las primeras habilitadas; el resto queda en «Más». */
export function bottomBarFeatures(features: readonly Feature[] = enabledFeatures): Feature[] {
  return features.slice(0, BOTTOM_BAR_LIMIT);
}
