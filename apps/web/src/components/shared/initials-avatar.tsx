import { cn } from "@/lib/utils";

/** Standard initials avatar (circle by default) used in tables, headers and menus. */
export function InitialsAvatar({ initials, size = "md", tone = "accent", shape = "circle", className }: { initials: string; size?: "sm" | "md" | "lg" | "xl"; tone?: "accent" | "primary" | "muted"; shape?: "circle" | "square"; className?: string }) {
  const sz = { sm: "size-7 text-[11px]", md: "size-8 text-[11px]", lg: "size-9 text-xs", xl: "size-12 text-sm" }[size];
  const tn = { accent: "bg-accent text-accent-foreground", primary: "bg-primary text-primary-foreground", muted: "bg-muted text-muted-foreground" }[tone];
  return <span className={cn("flex shrink-0 items-center justify-center font-bold", shape === "circle" ? "rounded-full" : "rounded-lg", sz, tn, className)}>{initials}</span>;
}
