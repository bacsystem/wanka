import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { toneBox, type StatTone } from "./stat-card";

interface SectionCardProps {
  title: ReactNode;
  /** Subtitle under the title (Stitch cards almost always have one). */
  description?: ReactNode;
  icon?: LucideIcon;
  iconClassName?: string;
  /** Color of the icon box; icons render inside a tinted rounded square. */
  iconTone?: StatTone;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
}

export function SectionCard({ title, description, icon: Icon, iconClassName, iconTone = "default", action, children, className, contentClassName }: SectionCardProps) {
  return (
    <Card className={cn("min-w-0 gap-0 overflow-hidden rounded-xl py-0 shadow-xs", className)}>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b px-4 py-3">
        <div className="flex min-w-0 items-center gap-2.5">
          {Icon ? (
            <span className={cn("flex size-7 shrink-0 items-center justify-center rounded-lg", toneBox[iconTone], iconClassName)}>
              <Icon className="size-4" strokeWidth={2} aria-hidden />
            </span>
          ) : null}
          <div className="min-w-0">
            <h2 className="min-w-0 text-sm font-semibold">{title}</h2>
            {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
          </div>
        </div>
        {action ? <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2">{action}</div> : null}
      </div>
      <CardContent className={cn("p-0", contentClassName)}>{children}</CardContent>
    </Card>
  );
}
