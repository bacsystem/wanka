"use client";

import type { ReactNode } from "react";
import * as React from "react";
import { Buscador } from "khipu-design-system/formularios/buscador";
import { cn } from "khipu-design-system/utils";

/**
 * Row of filters above a table.
 * @deprecated Temporary W5a adapter: the DS `Toolbar` is a pill-shaped action group, not a filter row, so this stays a layout row. Deleted in W8.
 */
export function Toolbar({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex flex-wrap items-center gap-2 border-b p-3", className)}>{children}</div>;
}

type InputProps = Omit<React.ComponentProps<"input">, "onChange" | "value" | "className" | "placeholder" | "type" | "ref">;

interface SearchInputProps extends InputProps {
  placeholder: string;
  value?: string;
  onChange?: (value: string) => void;
  /** Wrapper classes (width). */
  className?: string;
  ref?: React.Ref<HTMLInputElement>;
}

/**
 * @deprecated Temporary W5a adapter over `Buscador`. Use `Buscador` directly (`valor`/`onCambio`); deleted in W8.
 * `Buscador` does not forward `ref` (DS gap), so the input is looked up from the wrapper.
 */
export function SearchInput({ placeholder, value, onChange, className, ref, ...rest }: SearchInputProps) {
  const [internal, setInternal] = React.useState("");
  const current = value ?? internal;
  const wrapper = React.useCallback(
    (el: HTMLDivElement | null) => {
      const input = el?.querySelector("input") ?? null;
      if (typeof ref === "function") ref(input);
      else if (ref) ref.current = input;
    },
    [ref],
  );
  return (
    <div ref={wrapper} className={cn("min-w-0 flex-1 sm:max-w-xs", className)}>
      <Buscador
        placeholder={placeholder}
        valor={current}
        onCambio={(v) => {
          setInternal(v);
          onChange?.(v);
        }}
        {...rest}
      />
    </div>
  );
}
