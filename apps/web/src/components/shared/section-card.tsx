import type { ReactNode } from "react";
import { CabeceraSeccion } from "khipu-design-system/patrones/cabecera-seccion";
import { Card, CardDescription, CardHeader, CardTitle } from "khipu-design-system/ui/card";
import type { IconoComponente } from "khipu-design-system/lib/iconos";
import { cn } from "khipu-design-system/utils";
import { TARJETA } from "@/lib/estilos";
import type { StatTone } from "./stat-card";

interface SectionCardProps {
  title: ReactNode;
  /** Subtitle under the title. */
  description?: ReactNode;
  icon?: IconoComponente;
  /** @deprecated No effect: the DS section icon has no box or tint (ADR 0010). Removed with the adapter in W8. */
  iconClassName?: string;
  /** @deprecated No effect: the DS section icon has no box or tint (ADR 0010). Removed with the adapter in W8. */
  iconTone?: StatTone;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
}

/**
 * @deprecated Temporary W5a adapter over the DS section card. Use `Card` + `CabeceraSeccion` directly (module PRs, W6.x); deleted in W8.
 * With an icon: `TARJETA` recipe + `CabeceraSeccion`. Without one (`CabeceraSeccion` requires an icon, DS gap): DS `Card` + `CardHeader`.
 */
export function SectionCard({ title, description, icon, action, children, className, contentClassName }: SectionCardProps) {
  const actions = action ? <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2">{action}</div> : undefined;
  if (icon) {
    return (
      <div className={cn(TARJETA, "min-w-0 overflow-hidden", className)}>
        <CabeceraSeccion icon={icon} titulo={title} subtitulo={description} derecha={actions} />
        <div className={contentClassName}>{children}</div>
      </div>
    );
  }
  return (
    <Card className={cn("min-w-0", className)}>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <div className="min-w-0">
            <CardTitle>
              <h2>{title}</h2>
            </CardTitle>
            {description ? <CardDescription>{description}</CardDescription> : null}
          </div>
          {actions}
        </div>
      </CardHeader>
      <div className={contentClassName}>{children}</div>
    </Card>
  );
}
