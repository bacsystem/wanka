import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  status?: ReactNode;
  className?: string;
}

export function PageHeader({ eyebrow, title, description, actions, status, className }: PageHeaderProps) {
  return (
    <div className={cn("flex flex-wrap items-start justify-between gap-3", className)}>
      <div className="min-w-0">
        {eyebrow ? <p className="text-xs text-muted-foreground">{eyebrow}</p> : null}
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-xl font-bold tracking-tight md:text-2xl">{title}</h1>
          {status}
        </div>
        {description ? <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
