export interface PaletteOption {
  value: string;
  label: string;
  swatch: [string, string, string];
}

export interface FontOption {
  value: string;
  label: string;
}

export const PALETTES: PaletteOption[] = [
  { value: "lima", label: "Lima", swatch: ["#d4f06b", "#cfe2fb", "#f8ec84"] },
  { value: "indigo", label: "Índigo", swatch: ["#ddd4fb", "#cfe2fb", "#f5ccde"] },
  { value: "oceano", label: "Océano", swatch: ["#a4e2ea", "#e4e4f8", "#fbe3b0"] },
  { value: "coral", label: "Coral", swatch: ["#fbc4ba", "#cfe2fb", "#f8ec84"] },
  { value: "violeta", label: "Violeta", swatch: ["#eecaf7", "#a4e2ea", "#f8ec84"] },
];

export const FONTS: FontOption[] = [
  { value: "inter", label: "Inter" },
  { value: "jakarta", label: "Plus Jakarta" },
  { value: "manrope", label: "Manrope" },
  { value: "dmsans", label: "DM Sans" },
  { value: "figtree", label: "Figtree" },
];

export const DEFAULT_PALETTE = "lima";
export const DEFAULT_FONT = "inter";

export const PALETTE_KEY = "wanka-palette";
export const FONT_KEY = "wanka-font";

export function validValue<T extends { value: string }>(
  candidate: string | undefined | null,
  options: readonly T[],
  fallback: string,
): string {
  return candidate && options.some((o) => o.value === candidate) ? candidate : fallback;
}