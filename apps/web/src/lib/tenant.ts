import type { Industry } from "@/types/domain";

export const industryLabel: Record<Industry, string> = {
  restaurante: "Restaurante",
  veterinaria: "Veterinaria",
  odontologia: "Odontología",
  retail: "Venta de productos",
};

export function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join("");
}
