import type { LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type BadgeTone = "success" | "warning" | "danger" | "info" | "neutral";

/** Stitch pills: tinted background + matching border. */
export const toneClass: Record<BadgeTone, string> = {
  success: "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-300",
  warning: "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  danger: "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/60 dark:text-rose-300",
  info: "border-teal-200 bg-teal-50 text-teal-800 dark:border-teal-900 dark:bg-teal-950/60 dark:text-teal-300",
  neutral: "border-border bg-muted text-muted-foreground",
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
    <Badge variant="outline" className={cn("gap-1 font-medium", toneClass[tone], className)} data-tone={tone}>
      {Icon ? <Icon strokeWidth={2} aria-hidden /> : null}
      {dot ? <span className="size-1.5 rounded-full bg-current" aria-hidden /> : null}
      {label}
    </Badge>
  );
}

/** Small uppercase tag ("PRINCIPAL", "ENTERPRISE", "OFICIAL"): the only ad-hoc chip allowed. */
export function Tag({ tone = "neutral", label, className, solid }: { tone?: BadgeTone | "primary"; label: string; className?: string; solid?: boolean }) {
  const cls = tone === "primary" ? (solid ? "bg-primary text-primary-foreground" : "border border-primary/20 bg-accent text-accent-foreground") : solid ? { success: "bg-emerald-600 text-white", warning: "bg-amber-500 text-white", danger: "bg-rose-600 text-white", info: "bg-blue-600 text-white", neutral: "bg-foreground text-background" }[tone] : toneClass[tone] + " border";
  return <span className={cn("inline-flex shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-bold tracking-wide uppercase", cls, className)}>{label}</span>;
}
