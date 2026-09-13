import type { LucideIcon } from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";

export interface SectionNavItem {
  href: string;
  label: string;
  icon?: LucideIcon;
  /** Small mono counter shown after the label. */
  badge?: string;
  /** Extra classes for the inactive badge (tone). */
  badgeClassName?: string;
}

interface SectionNavProps {
  items: SectionNavItem[];
  /** href of the current section. */
  active: string;
  "aria-label": string;
  className?: string;
}

/** Route-based pill navigation between sibling screens (compras, inventario…). Uses Link so each tab is a real page. */
export function SectionNav({ items, active, className, ...rest }: SectionNavProps) {
  return (
    <nav className={cn("no-scrollbar flex max-w-full gap-1 overflow-x-auto rounded-xl border bg-card p-1.5 shadow-xs", className)} {...rest}>
      {items.map((t) => {
        const isActive = t.href === active;
        return (
          <Link key={t.href} href={t.href} aria-current={isActive ? "page" : undefined} className={cn("flex h-10 shrink-0 items-center gap-2 rounded-lg px-4 text-sm font-semibold transition-colors", isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-foreground/80 hover:bg-muted")}>
            {t.icon ? <t.icon className="size-4" /> : null} {t.label}
            {t.badge ? <span className={cn("rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold", isActive ? "bg-primary-foreground/20" : t.badgeClassName || "bg-accent text-primary dark:bg-muted")}>{t.badge}</span> : null}
          </Link>
        );
      })}
    </nav>
  );
}
