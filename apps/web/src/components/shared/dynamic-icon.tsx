"use client";

import {
  CheckCircleIcon,
  ClockIcon,
  LockIcon as PhosphorLockIcon,
  LockOpenIcon,
  type IconProps,
  type Icon,
  WarningIcon,
  WifiHighIcon,
  WifiSlashIcon,
  XCircleIcon,
  ArrowsClockwiseIcon,
} from "@phosphor-icons/react";
import { PESO_ICONO } from "khipu-design-system/lib/iconos";
import { Spinner } from "khipu-design-system/feedback/spinner";
import { cn } from "khipu-design-system/utils";

/**
 * Phosphor icons that change with state: the glyph swaps per status and gets a subtle motion cue.
 * @deprecated Temporary W5a adapter; module PRs use the DS icons directly. Deleted in W8.
 */
export type IconState = "ok" | "warning" | "error" | "pending" | "syncing" | "offline";

const map: Record<IconState, { icon: Icon; className: string }> = {
  ok: { icon: CheckCircleIcon, className: "text-success-solid" },
  warning: { icon: WarningIcon, className: "text-warning-solid motion-safe:animate-pulse" },
  error: { icon: XCircleIcon, className: "text-destructive-texto motion-safe:animate-pulse" },
  pending: { icon: ClockIcon, className: "text-muted-foreground" },
  syncing: { icon: ArrowsClockwiseIcon, className: "text-primary motion-safe:animate-spin" },
  offline: { icon: WifiSlashIcon, className: "text-destructive-texto" },
};

export function StateIcon({ state, className, ...props }: { state: IconState } & IconProps) {
  const { icon: Glyph, className: tone } = map[state];
  return <Glyph weight={PESO_ICONO} className={cn("size-4 transition-colors", tone, className)} aria-hidden {...props} />;
}

/** Connection icon: wifi ↔ wifi-slash. */
export function ConnectionIcon({ online, className }: { online: boolean; className?: string }) {
  const Glyph = online ? WifiHighIcon : WifiSlashIcon;
  return <Glyph weight={PESO_ICONO} className={cn("size-4", online ? "text-success-solid" : "text-destructive-texto motion-safe:animate-pulse", className)} aria-hidden />;
}

/** Lock icon: lock ↔ lock-open. */
export function LockIcon({ locked, className }: { locked: boolean; className?: string }) {
  const Glyph = locked ? PhosphorLockIcon : LockOpenIcon;
  return <Glyph weight={PESO_ICONO} className={cn("size-4 transition-transform", !locked && "-rotate-12", className)} aria-hidden />;
}

/** Spinner for buttons while an action runs (decorative: the button label says what is happening). */
export function ButtonSpinner({ className }: { className?: string }) {
  return <Spinner role={undefined} aria-label={undefined} aria-hidden className={className} />;
}
