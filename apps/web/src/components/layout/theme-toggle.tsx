"use client";

import { Monitor, Moon, Palette, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useAppearance } from "@/components/providers/appearance-provider";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function ThemeToggle() {
  const { theme = "system", setTheme } = useTheme();
  const { palette, setPalette, palettes, font, setFont, fonts } = useAppearance();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label="Cambiar tema y apariencia" />}>
        <Sun className="dark:hidden" />
        <Moon className="hidden dark:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuGroup>
          <DropdownMenuLabel>Tema</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={theme} onValueChange={(v) => setTheme(String(v))}>
            <DropdownMenuRadioItem value="light" closeOnClick>
              <Sun /> Claro
            </DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="dark" closeOnClick>
              <Moon /> Oscuro
            </DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="system" closeOnClick>
              <Monitor /> Sistema
            </DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        {palettes.length > 0 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuLabel className="flex items-center gap-1.5">
                <Palette className="size-3.5" /> Paleta
              </DropdownMenuLabel>
              <DropdownMenuRadioGroup value={palette} onValueChange={(v) => setPalette(String(v))}>
                {palettes.map((p) => (
                  <DropdownMenuRadioItem key={p.value} value={p.value}>
                    <span className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: p.swatch[0] }} />
                    {p.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </>
        ) : null}
        {fonts.length > 0 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuLabel>Tipografía</DropdownMenuLabel>
              <DropdownMenuRadioGroup value={font} onValueChange={(v) => setFont(String(v))}>
                {fonts.map((f) => (
                  <DropdownMenuRadioItem key={f.value} value={f.value}>
                    {f.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}