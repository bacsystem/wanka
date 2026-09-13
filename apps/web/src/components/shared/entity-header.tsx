import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface EntityHeaderProps {
  /** Avatar / icon box on the left (already styled). */
  avatar?: ReactNode;
  title: ReactNode;
  /** Chips next to the title (status, type, code). */
  chips?: ReactNode;
  /** Meta rows under the title (RUC, address, plan…). */
  meta?: ReactNode;
  /** Right column: alerts above the action buttons. */
  aside?: ReactNode;
  actions?: ReactNode;
  className?: string;
}

/** Standard header of detail screens (cliente, mascota, comprobante, orden, tenant…). */
export function EntityHeader({ avatar, title, chips, meta, aside, actions, className }: EntityHeaderProps) {
  return (
    <header className={cn("flex flex-col gap-4 rounded-xl border bg-card p-4 shadow-xs md:p-5 lg:flex-row lg:items-center lg:justify-between", className)}>
      <div className="flex min-w-0 items-start gap-4">
        {avatar}
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2"><h1 className="text-xl font-bold tracking-tight md:text-2xl">{title}</h1>{chips}</div>
          {meta ? <div className="mt-1.5 flex flex-col gap-1.5 text-xs text-muted-foreground">{meta}</div> : null}
        </div>
      </div>
      {aside || actions ? (
        <div className="flex min-w-0 flex-col gap-3 lg:items-end">
          {aside}
          {actions ? <div className="flex flex-wrap gap-2 lg:justify-end">{actions}</div> : null}
        </div>
      ) : null}
    </header>
  );
}
