import type { ReactNode } from "react";
import { cn } from "khipu-design-system/utils";
import { TARJETA } from "@/lib/estilos";

interface FilterBarProps {
  children: ReactNode;
  className?: string;
  /** Stack rows (search row + chips row) instead of a single wrapping row. */
  stack?: boolean;
  "aria-label"?: string;
}

/**
 * Standalone filter container above a table/board: `TARJETA` recipe plus layout.
 * @deprecated Temporary W5a adapter. The DS has no filter-bar container (its `Toolbar` is an action pill group): DS gap. Deleted in W8.
 */
export function FilterBar({ children, className, stack = false, ...rest }: FilterBarProps) {
  return (
    <div className={cn(TARJETA, "p-3", stack ? "flex flex-col gap-3" : "flex flex-wrap items-center gap-2", className)} {...rest}>
      {children}
    </div>
  );
}
