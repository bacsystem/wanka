"use client";

import { UserPlus, X } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { SearchInput } from "@/components/shared/toolbar";
import { initials } from "@/lib/tenant";
import type { Customer } from "@/types/domain";

interface CustomerPickerProps {
  customers: Customer[];
  value: Customer;
  generic: Customer;
  onChange: (c: Customer) => void;
}

export function CustomerPicker({ customers, value, generic, onChange }: CustomerPickerProps) {
  const [query, setQuery] = React.useState("");
  const [open, setOpen] = React.useState(false);
  const q = query.trim().toLowerCase();
  const results = q
    ? customers.filter((c) => c.name.toLowerCase().includes(q) || c.documentNumber.includes(q)).slice(0, 5)
    : [];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3 rounded-lg bg-muted/60 p-3">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-primary">
          {initials(value.name)}
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold">{value.name}</p>
          <p className="text-xs text-muted-foreground tabular-nums">
            {value.documentType} {value.documentNumber}
            {value.clinicalRecord ? <> · <span className="font-medium text-primary">{value.clinicalRecord}</span></> : ""}
          </p>
        </div>
        {value.id !== generic.id ? (
          <Button variant="ghost" size="icon-sm" aria-label="Quitar cliente" onClick={() => onChange(generic)}>
            <X />
          </Button>
        ) : null}
      </div>
      <div className="relative">
        <SearchInput
          value={query}
          onChange={(v) => { setQuery(v); setOpen(true); }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 120)}
          placeholder="Buscar por DNI, RUC o nombre…"
          aria-label="Buscar cliente"
          className="sm:max-w-none"
        />
        {open && results.length > 0 ? (
          <ul className="absolute z-20 mt-1 w-full overflow-hidden rounded-md border bg-popover shadow-md" role="listbox">
            {results.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  role="option"
                  aria-selected={c.id === value.id}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    onChange(c);
                    setQuery("");
                    setOpen(false);
                  }}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                >
                  <span className="truncate">{c.name}</span>
                  <span className="ml-auto shrink-0 font-mono text-xs text-muted-foreground tabular-nums">
                    {c.documentType} {c.documentNumber}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <Button variant="ghost" size="sm" className="w-fit px-1 text-primary">
        <UserPlus data-icon="inline-start" /> Nuevo cliente
      </Button>
    </div>
  );
}
