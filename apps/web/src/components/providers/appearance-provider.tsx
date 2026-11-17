"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";
import {
  DEFAULT_FONT,
  DEFAULT_PALETTE,
  FONT_KEY,
  FONTS,
  type FontOption,
  PALETTE_KEY,
  PALETTES,
  type PaletteOption,
  validValue,
} from "@/config/appearance";

interface AppearanceContextValue {
  palette: string;
  setPalette: (value: string) => void;
  palettes: PaletteOption[];
  font: string;
  setFont: (value: string) => void;
  fonts: FontOption[];
}

const AppearanceContext = createContext<AppearanceContextValue>({
  palette: DEFAULT_PALETTE,
  setPalette: () => {},
  palettes: [],
  font: DEFAULT_FONT,
  setFont: () => {},
  fonts: [],
});

function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Ignore storage errors (private browsing / restricted storage)
  }
}

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [palette, setPaletteState] = useState(DEFAULT_PALETTE);
  const [font, setFontState] = useState(DEFAULT_FONT);

  useEffect(() => {
    const d = document.documentElement.dataset;
    queueMicrotask(() => {
      setPaletteState(validValue(d.palette ?? readStorage(PALETTE_KEY), PALETTES, DEFAULT_PALETTE));
      setFontState(validValue(d.font ?? readStorage(FONT_KEY), FONTS, DEFAULT_FONT));
    });
  }, []);

  const setPalette = useCallback((next: string) => {
    const valid = validValue(next, PALETTES, DEFAULT_PALETTE);
    setPaletteState(valid);
    document.documentElement.dataset.palette = valid;
    writeStorage(PALETTE_KEY, valid);
  }, []);

  const setFont = useCallback((next: string) => {
    const valid = validValue(next, FONTS, DEFAULT_FONT);
    setFontState(valid);
    document.documentElement.dataset.font = valid;
    writeStorage(FONT_KEY, valid);
  }, []);

  const value = useMemo<AppearanceContextValue>(
    () => ({ palette, setPalette, palettes: PALETTES, font, setFont, fonts: FONTS }),
    [palette, setPalette, font, setFont],
  );

  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance() {
  return useContext(AppearanceContext);
}