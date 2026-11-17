import type { ReactNode } from "react";
import { cn } from "khipu-design-system/utils";
import { TARJETA } from "@/lib/estilos";

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

/**
 * Standard header of detail screens (cliente, mascota, comprobante, orden, tenant…).
 * @deprecated Temporary W5a adapter. The DS has no entity header (avatar, chips, meta rows, aside): this keeps the `TARJETA`
 * recipe and the DS type scale, because `CabeceraPagina` cannot host the chips and meta rows without invalid nesting (DS gap). Deleted in W8.
 */
export function EntityHeader({ avatar, title, chips, meta, aside, actions, className }: EntityHeaderProps) {
  return (
    <header className={cn(TARJETA, "flex flex-col gap-4 p-4 md:p-5 lg:flex-row lg:items-center lg:justify-between", className)}>
      <div className="flex min-w-0 items-start gap-4">
        {avatar}
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2"><h1 className="text-titulo-pagina font-semibold text-foreground">{title}</h1>{chips}</div>
          {meta ? <div className="mt-1.5 flex flex-col gap-1.5 text-etiqueta text-muted-foreground">{meta}</div> : null}
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
