"use client";

import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CommandPalette, useCommandPalette } from "./command-palette";

export function TopbarSearch() {
  const { open, setOpen } = useCommandPalette();
  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className="relative hidden h-10 w-64 items-center rounded-lg border border-input bg-muted/50 pl-9 text-left text-xs text-muted-foreground transition-colors hover:bg-card lg:flex xl:w-72" aria-label="Búsqueda global (Ctrl+K)">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
        <span className="truncate">Buscar comprobante, cliente, producto…</span>
        <kbd className="pointer-events-none absolute top-1/2 right-2 flex -translate-y-1/2 items-center gap-0.5 rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] font-semibold text-muted-foreground">⌘K</kbd>
      </button>
      <Button variant="ghost" size="icon" aria-label="Buscar" className="lg:hidden" onClick={() => setOpen(true)}><Search /></Button>
      <CommandPalette open={open} onOpenChange={setOpen} />
    </>
  );
}
