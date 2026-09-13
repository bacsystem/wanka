"use client";

import * as React from "react";
import { Input } from "@/components/ui/input";
import { parseNumber } from "@/lib/format";

interface NumberInputProps extends Omit<React.ComponentProps<typeof Input>, "value" | "onChange" | "type" | "max"> {
  value: number;
  onValueChange: (value: number) => void;
  /** Upper bound applied on every change. */
  max?: number;
  /** Whole units only (pieces, banknotes): numeric keyboard and decimals are truncated. */
  integer?: boolean;
}

/**
 * Numeric field for quantities, prices and discounts. Keeps what the user typed as text (so "12." or "1,5"
 * survive the re-render and clearing the field shows empty, not "0"), commits the parsed number on every
 * change and normalizes the text on blur.
 */
export function NumberInput({ value, onValueChange, max, integer = false, onBlur, inputMode, ref, ...rest }: NumberInputProps) {
  const [text, setText] = React.useState(String(value));
  const committed = React.useRef(value);
  const inputRef = React.useRef<HTMLInputElement>(null);
  React.useImperativeHandle(ref, () => inputRef.current!, []);

  // Parent changed the value (reset, recalculation): reflect it unless it is the one we just committed.
  // A reset to the same number while the field shows something else ("abc" → 0) is only applied when the
  // field is not focused, so it never fights the user's typing.
  React.useEffect(() => {
    const external = value !== committed.current;
    const stale = String(value) !== text && document.activeElement !== inputRef.current;
    if (external || stale) {
      committed.current = value;
      setText(String(value));
    }
  }, [value, text]);

  return (
    <Input
      ref={inputRef}
      type="text"
      inputMode={inputMode ?? (integer ? "numeric" : "decimal")}
      {...rest}
      value={text}
      onChange={(e) => {
        const raw = e.target.value;
        setText(raw);
        let n = parseNumber(raw);
        if (integer) n = Math.trunc(n);
        if (max !== undefined) n = Math.min(max, n);
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
