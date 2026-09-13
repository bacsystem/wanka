import { Search } from "lucide-react";
import type { ReactNode } from "react";
import * as React from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function Toolbar({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex flex-wrap items-center gap-2 border-b p-3", className)}>{children}</div>;
}

interface SearchInputProps extends Omit<React.ComponentProps<typeof Input>, "onChange" | "value" | "className"> {
  placeholder: string;
  value?: string;
  onChange?: (value: string) => void;
  /** Wrapper classes (width). */
  className?: string;
  /** Extra classes for the input itself. */
  inputClassName?: string;
  /** Muted background (Stitch filter bars). Default true. */
  muted?: boolean;
  /** Icon shown instead of the search glass (e.g. ScanLine for barcode fields). */
  icon?: React.ComponentType<{ className?: string }>;
  ref?: React.Ref<HTMLInputElement>;
}

/** The only search field: 40 px, icon at left, optional muted background. */
export function SearchInput({ placeholder, value, onChange, className, inputClassName, muted = true, icon: Icon = Search, ref, ...rest }: SearchInputProps) {
  return (
    <div className={cn("relative min-w-0 flex-1 sm:max-w-xs", className)}>
      <Icon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        ref={ref}
        type="search"
        placeholder={placeholder}
        aria-label={rest["aria-label"] ?? placeholder}
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        {...rest}
        className={cn("pl-9 text-sm", muted && "bg-muted/50", inputClassName)}
      />
    </div>
  );
}
