import { Chip, PillEstado } from "khipu-design-system/patrones/pill-estado";
import type { IconoComponente } from "khipu-design-system/lib/iconos";

export type BadgeTone = "success" | "warning" | "danger" | "info" | "neutral";

/**
 * Class map still imported by `features/documents/components/sunat-status-badge.tsx` (W6.3 moves it to `PillEstado`).
 * @deprecated Removed with that module migration; do not use in new code.
 */
export const toneClass: Record<BadgeTone, string> = {
  success: "border-transparent bg-success text-success-foreground",
  warning: "border-transparent bg-warning text-warning-foreground",
  danger: "border-transparent bg-destructive-suave text-destructive-texto",
  info: "border-transparent bg-info text-info-foreground",
  neutral: "border-transparent bg-secondary text-muted-foreground",
};

const TONO = { success: "ok", warning: "aviso", danger: "error", info: "info", neutral: "neutro" } as const;

interface StatusBadgeProps {
  tone: BadgeTone;
  label: string;
  icon?: IconoComponente;
  /** @deprecated No effect: `PillEstado` always shows a dot when it has no icon. */
  dot?: boolean;
  className?: string;
}

/** @deprecated Temporary W5a adapter over `PillEstado`. Use it directly (`tono`); deleted in W8. */
export function StatusBadge({ tone, label, icon, className }: StatusBadgeProps) {
  return (
    <PillEstado tono={TONO[tone]} icon={icon} className={className}>
      {label}
    </PillEstado>
  );
}

/**
 * Compact label chip.
 * @deprecated Temporary W5a adapter over `Chip` (`tono`, `solido`). Use it directly; deleted in W8.
 */
export function Tag({ tone = "neutral", label, className, solid }: { tone?: BadgeTone | "primary"; label: string; className?: string; solid?: boolean }) {
  return <Chip tono={tone === "primary" ? "acento" : TONO[tone]} solido={solid} className={className}>{label}</Chip>;
}
