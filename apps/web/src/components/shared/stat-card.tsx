import type { LucideIcon } from "lucide-react";
import { TrendingDown, TrendingUp } from "lucide-react";
import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { formatPercent } from "@/lib/format";
import { cn } from "@/lib/utils";

export type StatTone = "default" | "success" | "warning" | "danger" | "info";

interface StatCardProps {
  label: string;
  value: string;
  /** Small unit text next to the value, e.g. "documentos". */
  suffix?: string;
  icon: LucideIcon;
  deltaPercent?: number | null;
  /** Text after the delta ("vs. ayer"). */
  deltaLabel?: string;
  hint?: ReactNode;
  /** Extra row under a divider: chips, sparkline, link. */
  footer?: ReactNode;
  tone?: StatTone;
  /** Paint the value with the tone color (Stitch "Stock bajo"). */
  emphasizeValue?: boolean;
  /** Solid primary card (Stitch "Saldo teórico esperado"). */
  highlight?: boolean;
  className?: string;
}

/** Icon box colors per tone (Stitch: teal / blue / amber / rose). */
export const toneBox: Record<StatTone, string> = {
  default: "bg-accent text-primary",
  success: "bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400",
  info: "bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400",
  warning: "bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400",
  danger: "bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400",
};
export const toneText: Record<StatTone, string> = {
  default: "text-primary",
  success: "text-emerald-600 dark:text-emerald-400",
  info: "text-blue-600 dark:text-blue-400",
  warning: "text-amber-600 dark:text-amber-400",
  danger: "text-rose-600 dark:text-rose-400",
};

export function StatCard({ label, value, suffix, icon: Icon, deltaPercent, deltaLabel, hint, footer, tone = "default", emphasizeValue, highlight, className }: StatCardProps) {
  const hasDelta = deltaPercent !== undefined && deltaPercent !== null;
  const positive = hasDelta && deltaPercent >= 0;
  return (
    <Card className={cn("gap-0 rounded-xl py-0 shadow-xs transition-colors hover:border-ring/30", highlight && "border-primary bg-primary text-primary-foreground [&_.text-muted-foreground]:text-primary-foreground/80 [&_.border-t]:border-primary-foreground/20", className)}>
      <CardContent className="flex h-full flex-col p-4">
        <div className="flex items-center justify-between gap-2 text-muted-foreground">
          <span className="text-xs font-medium">{label}</span>
          <span className={cn("flex size-7 shrink-0 items-center justify-center rounded-lg", highlight ? "bg-primary-foreground/20 text-primary-foreground" : toneBox[tone])}>
            <Icon className="size-4" strokeWidth={2} aria-hidden />
          </span>
        </div>
        <div className="mt-2 mb-1">
          <div className={cn("text-2xl font-bold tracking-tight tabular-nums", emphasizeValue && toneText[tone])} data-testid="kpi-value">
            {value}
            {suffix ? <span className="ml-1.5 font-sans text-xs font-normal text-muted-foreground">{suffix}</span> : null}
          </div>
          {hasDelta || hint ? (
            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs">
              {hasDelta ? (
                <span className={cn("inline-flex items-center gap-0.5 font-semibold", positive ? "text-emerald-600 dark:text-emerald-400" : "text-rose-600 dark:text-rose-400")}>
                  {positive ? <TrendingUp className="size-3.5" /> : <TrendingDown className="size-3.5" />}
                  {formatPercent(deltaPercent)}
                </span>
              ) : null}
              {deltaLabel ? <span className="text-muted-foreground">{deltaLabel}</span> : null}
              {hint ? <span className={cn("text-muted-foreground", emphasizeValue && cn("font-medium", toneText[tone]))}>{hint}</span> : null}
            </div>
          ) : null}
        </div>
        {footer ? <div className="mt-auto flex items-center gap-2 border-t pt-2 text-[11px] font-medium text-muted-foreground">{footer}</div> : null}
      </CardContent>
    </Card>
  );
}

/** Small neutral chip used in StatCard footers ("34 Boletas"). */
export function StatChip({ children }: { children: ReactNode }) {
  return <span className="rounded bg-muted px-2 py-0.5 font-semibold text-foreground/80">{children}</span>;
}

/** KPI row: horizontal scroller on mobile, grid from sm up. */
export function StatGrid({ children, columns = 4 }: { children: React.ReactNode; columns?: 3 | 4 | 5 }) {
  const cols = { 3: "xl:grid-cols-3", 4: "xl:grid-cols-4", 5: "xl:grid-cols-5" }[columns];
  return (
    <section aria-label="Indicadores" className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
      <div
        className={cn(
          "grid min-w-max auto-cols-[minmax(11rem,1fr)] grid-flow-col gap-3 sm:min-w-0 sm:grid-flow-row sm:grid-cols-2 md:gap-4",
          cols,
        )}
      >
        {children}
      </div>
    </section>
  );
}

/** Compact KPI (Stitch agenda/inventory variant): icon box left, big value + label. */
export function StatCompact({ label, value, icon: Icon, tone = "default", className, emphasizeValue }: { label: string; value: string; icon: LucideIcon; tone?: StatTone; className?: string; emphasizeValue?: boolean }) {
  return (
    <Card className={cn("gap-0 rounded-xl py-0 shadow-xs", className)}>
      <CardContent className="flex items-center gap-4 p-4">
        <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-lg", toneBox[tone])}><Icon className="size-5" strokeWidth={2} aria-hidden /></span>
        <div className="min-w-0">
          <p className={cn("text-2xl font-bold tracking-tight tabular-nums", emphasizeValue && toneText[tone])} data-testid="kpi-value">{value}</p>
          <p className="truncate text-xs text-muted-foreground">{label}</p>
        </div>
      </CardContent>
    </Card>
  );
}
