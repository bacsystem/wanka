"use client";

import * as React from "react";
import { EntradaDecimal } from "khipu-design-system/formularios/entrada-decimal";

interface NumberInputProps {
  id?: string;
  name?: string;
  value: number;
  onValueChange: (value: number) => void;
  /** Upper bound applied on every change. */
  max?: number;
  /** Whole units only (pieces, banknotes): numeric keyboard and decimals are truncated. */
  integer?: boolean;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
  "aria-label"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  /** Ignored: every DS control is `h-control`. Kept so existing call sites compile until their module migrates. */
  size?: "default" | "sm" | "lg";
}

/**
 * @deprecated Use `EntradaDecimal` (or `EntradaMonto`) from `khipu-design-system/formularios/*` directly; this adapter
 * goes away in W8. Keeps Wanka's `number` contract (an emptied field commits `0`) over the DS decimal input.
 */
export function NumberInput({ id, value, onValueChange, max, integer = false, "aria-invalid": invalid, size, ...rest }: NumberInputProps) {
  void size;
  const autoId = React.useId();
  return (
    <EntradaDecimal
      id={id ?? autoId}
      valor={value}
      onCambio={(v) => onValueChange(v ?? 0)}
      max={max}
      decimales={integer ? 0 : 2}
      invalido={invalid}
      {...rest}
    />
  );
}
