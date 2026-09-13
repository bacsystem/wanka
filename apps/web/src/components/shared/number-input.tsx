"use client";

import * as React from "react";
import { Input } from "@/components/ui/input";
import { parseNumber } from "@/lib/format";

interface NumberInputProps extends Omit<React.ComponentProps<typeof Input>, "value" | "onChange" | "type" | "max"> {
  value: number;
  onValueChange: (value: number) => void;
  /** Upper bound applied on every change. */
  max?: number;
}

/**
 * Numeric field for quantities, prices and discounts. Keeps what the user typed as text (so "12." or "1,5"
 * survive the re-render and clearing the field shows empty, not "0"), commits the parsed number on every
 * change and normalizes the text on blur.
 */
export function NumberInput({ value, onValueChange, max, onBlur, inputMode = "decimal", ...rest }: NumberInputProps) {
  const [text, setText] = React.useState(String(value));
  const committed = React.useRef(value);

  // Parent changed the value (reset, recalculation): reflect it unless it is the one we just committed.
  React.useEffect(() => {
    if (value !== committed.current) {
      committed.current = value;
      setText(String(value));
    }
  }, [value]);

  return (
    <Input
      type="text"
      inputMode={inputMode}
      {...rest}
      value={text}
      onChange={(e) => {
        const raw = e.target.value;
        setText(raw);
        const n = max === undefined ? parseNumber(raw) : Math.min(max, parseNumber(raw));
        committed.current = n;
        onValueChange(n);
      }}
      onBlur={(e) => {
        setText(String(value));
        onBlur?.(e);
      }}
    />
  );
}
