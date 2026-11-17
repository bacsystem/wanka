"use client";

import { useSyncExternalStore } from "react";

/**
 * While a page forces its own palette (the admin console), the appearance menu must not offer the palette choice:
 * picking one would silently change the palette the user saved for the tenant app. Module-level store so the
 * component that forces the palette and the menu need no common parent.
 */
let locks = 0;
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

/** Marks the palette as forced; returns the function that releases the lock. */
export function lockPalette(): () => void {
  locks += 1;
  emit();
  return () => {
    locks -= 1;
    emit();
  };
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function usePaletteLocked(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => locks > 0,
    () => false,
  );
}
