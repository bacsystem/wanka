import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface FilterBarProps {
  children: ReactNode;
  className?: string;
  /** Stack rows (search row + chips row) instead of a single wrapping row. */
  stack?: boolean;
  "aria-label"?: string;
}

/** Standalone filter container above a table/board: card surface, 12 px padding, 40 px controls inside. */
export function FilterBar({ children, className, stack = false, ...rest }: FilterBarProps) {
  return (
    <div className={cn("rounded-xl border bg-card p-3 shadow-xs", stack ? "flex flex-col gap-3" : "flex flex-wrap items-center gap-2", className)} {...rest}>
      {children}
    </div>
  );
}
