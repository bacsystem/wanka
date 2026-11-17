import type { LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type BadgeTone = "success" | "warning" | "danger" | "info" | "neutral";

/** Pastillas pastel del design system khipu 0.5 (sin borde en reposo, contraste WCAG AA). */
export const toneClass: Record<BadgeTone, string> = {
  success: "border-transparent bg-success text-success-foreground",
  warning: "border-transparent bg-warning text-warning-foreground",
  danger: "border-transparent bg-destructive-suave text-destructive-texto",
  info: "border-transparent bg-info text-info-foreground",
  neutral: "border-transparent bg-secondary text-muted-foreground",
};

interface StatusBadgeProps {
  tone: BadgeTone;
  label: string;
  icon?: LucideIcon;
  dot?: boolean;
  className?: string;
}

export function StatusBadge({ tone, label, icon: Icon, dot, className }: StatusBadgeProps) {
  return (
    <Badge variant="outline" className={cn("gap-1 rounded-full font-semibold", toneClass[tone], className)} data-tone={tone}>
      {Icon ? <Icon strokeWidth={2} aria-hidden /> : null}
      {dot ? <span className="size-1.5 rounded-full bg-current" aria-hidden /> : null}
      {label}
    </Badge>
  );
}

/** Chip compacto en mayúsculas ("PRINCIPAL", "ENTERPRISE", "OFICIAL"). */
export function Tag({ tone = "neutral", label, className, solid }: { tone?: BadgeTone | "primary"; label: string; className?: string; solid?: boolean }) {
  const cls =
    tone === "primary"
      ? solid
        ? "bg-primary text-primary-foreground"
        : "bg-acento-suave text-acento-superficie-texto"
      : solid
        ? {
            success: "bg-success-solid text-[#0a0b0e]",
            warning: "bg-warning-solid text-[#0a0b0e]",
            danger: "bg-destructive text-destructive-foreground",
            info: "bg-acento-2-texto text-white",
            neutral: "bg-foreground text-background",
          }[tone]
        : toneClass[tone];
  return <span className={cn("inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase", cls, className)}>{label}</span>;
}
