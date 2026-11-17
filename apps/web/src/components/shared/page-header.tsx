import type { ReactNode } from "react";
import { CabeceraPagina } from "khipu-design-system/patrones/cabecera-pagina";
import { ROTULO } from "@/lib/estilos";

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  status?: ReactNode;
  className?: string;
}

/**
 * @deprecated Temporary W5a adapter over `CabeceraPagina`. Use it directly (module PRs, W6.x); deleted in W8.
 * `CabeceraPagina` has no eyebrow or status slots (DS gap): the eyebrow goes in `volver` with the `ROTULO` recipe, and the status badge
 * goes first among the actions, so the `h1` keeps its exact accessible name.
 */
export function PageHeader({ eyebrow, title, description, actions, status, className }: PageHeaderProps) {
  return (
    <CabeceraPagina
      className={className}
      volver={eyebrow ? <p className={ROTULO}>{eyebrow}</p> : undefined}
      titulo={title}
      descripcion={description}
      acciones={
        status || actions ? (
          <>
            {status}
            {actions}
          </>
        ) : undefined
      }
    />
  );
}
