"use client";

import { AlertTriangle, CheckCircle2, Clock, Loader2, Lock, LockOpen, RefreshCw, Wifi, WifiOff, XCircle, type LucideProps } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Lucide icons that change with state (Stitch "iconos dinámicos"):
 * the glyph swaps per status and gets a subtle motion cue.
 */
export type IconState = "ok" | "warning" | "error" | "pending" | "syncing" | "offline";

const map: Record<IconState, { icon: React.ComponentType<LucideProps>; className: string }> = {
  ok: { icon: CheckCircle2, className: "text-success-solid" },
  warning: { icon: AlertTriangle, className: "text-warning-solid motion-safe:animate-pulse" },
  error: { icon: XCircle, className: "text-destructive-text motion-safe:animate-pulse" },
  pending: { icon: Clock, className: "text-muted-foreground" },
  syncing: { icon: RefreshCw, className: "text-primary motion-safe:animate-spin [animation-duration:1.6s]" },
  offline: { icon: WifiOff, className: "text-destructive-text" },
};

export function StateIcon({ state, className, ...props }: { state: IconState } & LucideProps) {
  const { icon: Icon, className: tone } = map[state];
  return <Icon className={cn("size-4 transition-colors", tone, className)} aria-hidden {...props} />;
}

/** Connection icon: wifi ↔ wifi-off. */
export function ConnectionIcon({ online, className }: { online: boolean; className?: string }) {
  const Icon = online ? Wifi : WifiOff;
  return <Icon className={cn("size-4", online ? "text-success-solid" : "text-destructive-text motion-safe:animate-pulse", className)} aria-hidden />;
}

/** Lock icon: lock ↔ lock-open. */
export function LockIcon({ locked, className }: { locked: boolean; className?: string }) {
  const Icon = locked ? Lock : LockOpen;
  return <Icon className={cn("size-4 transition-transform", !locked && "-rotate-12", className)} aria-hidden />;
}

/** Spinner for buttons while an action runs. */
export function ButtonSpinner({ className }: { className?: string }) {
  return <Loader2 className={cn("size-4 animate-spin", className)} aria-hidden />;
}
