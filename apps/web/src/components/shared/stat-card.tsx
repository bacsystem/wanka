import type { ReactNode } from "react";
import { Kpi } from "khipu-design-system/datos/kpi";
import type { IconoComponente } from "khipu-design-system/lib/iconos";
import { FilaMetricas, Metrica, type TonoMetrica } from "khipu-design-system/patrones/metricas";

export type StatTone = "default" | "success" | "warning" | "danger" | "info";

interface StatCardProps {
  label: string;
  value: string;
  suffix?: string;
  icon: IconoComponente;
  deltaPercent?: number | null;
  deltaLabel?: string;
  hint?: ReactNode;
  footer?: ReactNode;
  tone?: StatTone;
  emphasizeValue?: boolean;
  highlight?: boolean;
}

export const toneBox: Record<StatTone, string> = {
  default: "bg-acento-suave text-acento-superficie-texto",
  success: "bg-success text-success-foreground",
  info: "bg-info text-info-foreground",
  warning: "bg-warning text-warning-foreground",
  danger: "bg-destructive-suave text-destructive-texto",
};

export const toneText: Record<StatTone, string> = {
  default: "text-acento-superficie-texto",
  success: "text-success-foreground",
  info: "text-info-foreground",
  warning: "text-warning-foreground",
  danger: "text-destructive-texto",
};

const dsTone: Record<StatTone, TonoMetrica> = { default: "neutro", success: "ok", warning: "aviso", danger: "error", info: "acento" };

/**
 * @deprecated W8: use `Metrica` (or `Kpi` when there is a delta) from `khipu-design-system/patrones/metricas` and `datos/kpi` directly.
 * Adapter over the DS: a card with `deltaPercent` is a `Kpi`, anything else a `Metrica` (docs/frontend-migration-khipu.md §3.1).
 */
export function StatCard({ label, value, suffix, icon, deltaPercent, deltaLabel, hint, footer, tone = "default", emphasizeValue, highlight }: StatCardProps) {
  // Emphasis on a neutral card needs a colour to paint: the accent (DS-F review).
  const tono: TonoMetrica = emphasizeValue && tone === "default" ? "acento" : dsTone[tone];
  const valor = <span data-testid="kpi-value">{value}</span>;
  if (deltaPercent !== undefined && deltaPercent !== null) {
    return (
      <Kpi etiqueta={label} valor={valor} sufijo={suffix} icono={icon} tono={tono} valorConTono={emphasizeValue} variacion={deltaPercent} periodo={deltaLabel} ayuda={hint} pie={footer} destacada={highlight} />
    );
  }
  return (
    <Metrica etiqueta={label} sufijo={suffix} icono={icon} tono={tono} valorConTono={emphasizeValue} ayuda={hint ?? deltaLabel} pie={footer} destacada={highlight}>
      {valor}
    </Metrica>
  );
}

export function StatChip({ children }: { children: ReactNode }) {
  return <span className="rounded-full bg-secondary px-2.5 py-0.5 font-semibold text-foreground/80">{children}</span>;
}

/** @deprecated W8: use `FilaMetricas desplazable` from `khipu-design-system/patrones/metricas`. */
export function StatGrid({ children, columns = 4, label = "Indicadores" }: { children: ReactNode; columns?: 3 | 4 | 5; label?: string }) {
  return (
    <FilaMetricas desplazable columnas={columns} etiqueta={label}>
      {children}
    </FilaMetricas>
  );
}

interface StatCompactProps {
  label: string;
  value: string;
  icon: IconoComponente;
  tone?: StatTone;
  emphasizeValue?: boolean;
}

/** @deprecated W8: use `Metrica` from `khipu-design-system/patrones/metricas`. */
export function StatCompact({ label, value, icon, tone = "default", emphasizeValue }: StatCompactProps) {
  const tono: TonoMetrica = emphasizeValue && tone === "default" ? "acento" : dsTone[tone];
  return (
    <Metrica etiqueta={label} icono={icon} tono={tono} valorConTono={emphasizeValue}>
      {value}
    </Metrica>
  );
}
