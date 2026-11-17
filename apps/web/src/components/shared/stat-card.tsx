import type { LucideIcon } from "lucide-react";
import { TrendingDown, TrendingUp } from "lucide-react";
import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { ENTRADA } from "@/lib/estilos";
import { formatPercent } from "@/lib/format";
import { cn } from "@/lib/utils";

export type StatTone = "default" | "success" | "warning" | "danger" | "info";

interface StatCardProps {
  label: string;
  value: string;
  suffix?: string;
  icon: LucideIcon;
  deltaPercent?: number | null;
  deltaLabel?: string;
  hint?: ReactNode;
  footer?: ReactNode;
  tone?: StatTone;
  emphasizeValue?: boolean;
  highlight?: boolean;
  className?: string;
}

export const toneBox: Record<StatTone, string> = {
  default: "bg-acento-suave text-acento-texto",
  success: "bg-success text-success-foreground",
  info: "bg-info text-info-foreground",
  warning: "bg-warning text-warning-foreground",
  danger: "bg-destructive-suave text-destructive-texto",
};

export const toneText: Record<StatTone, string> = {
  default: "text-acento-texto",
  success: "text-success-foreground",
  info: "text-info-foreground",
  warning: "text-warning-foreground",
  danger: "text-destructive-texto",
};

export function StatCard({ label, value, suffix, icon: Icon, deltaPercent, deltaLabel, hint, footer, tone = "default", emphasizeValue, highlight, className }: StatCardProps) {
  const hasDelta = deltaPercent !== undefined && deltaPercent !== null;
  const positive = hasDelta && deltaPercent >= 0;
  return (
    <Card
      className={cn(
        "gap-0 py-0 transition-colors",
        ENTRADA,
        highlight && "border-acento-borde bg-acento-superficie text-acento-superficie-texto [&_.border-t]:border-acento-superficie-texto/15 [&_.text-muted-foreground]:text-acento-superficie-texto/80",
        className,
      )}
    >
      <CardContent className="flex h-full flex-col p-4">
        <div className="flex items-center justify-between gap-2 text-muted-foreground">
          <span className="text-etiqueta font-medium">{label}</span>
          <span className={cn("flex size-7 shrink-0 items-center justify-center rounded-lg", highlight ? "bg-acento-superficie-texto/10 text-acento-superficie-texto" : toneBox[tone])}>
            <Icon className="size-4" strokeWidth={2} aria-hidden />
          </span>
        </div>
        <div className="mt-2 mb-1">
          <div className={cn("text-metrica font-semibold tracking-tight tabular-nums", emphasizeValue && toneText[tone])} data-testid="kpi-value">
            {value}
            {suffix ? <span className="ml-1.5 font-sans text-etiqueta font-normal text-muted-foreground">{suffix}</span> : null}
          </div>
          {hasDelta || hint ? (
            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-etiqueta">
              {hasDelta ? (
                <span className={cn("inline-flex items-center gap-0.5 font-semibold", positive ? "text-success-foreground" : "text-destructive-texto")}>
                  {positive ? <TrendingUp className="size-3.5" /> : <TrendingDown className="size-3.5" />}
                  {formatPercent(deltaPercent)}
                </span>
              ) : null}
              {deltaLabel ? <span className="text-muted-foreground">{deltaLabel}</span> : null}
              {hint ? <span className={cn("text-muted-foreground", emphasizeValue && cn("font-medium", toneText[tone]))}>{hint}</span> : null}
            </div>
          ) : null}
        </div>
        {footer ? <div className="mt-auto flex items-center gap-2 border-t border-border/60 pt-2 text-[11px] font-medium text-muted-foreground">{footer}</div> : null}
      </CardContent>
    </Card>
  );
}

export function StatChip({ children }: { children: ReactNode }) {
  return <span className="rounded-full bg-secondary px-2.5 py-0.5 font-semibold text-foreground/80">{children}</span>;
}

export function StatGrid({ children, columns = 4 }: { children: React.ReactNode; columns?: 3 | 4 | 5 }) {
  const cols = { 3: "xl:grid-cols-3", 4: "xl:grid-cols-4", 5: "xl:grid-cols-5" }[columns];
  return (
    <div className={cn("-mx-4 flex snap-x snap-mandatory gap-3 overflow-x-auto px-4 pb-1 sm:mx-0 sm:grid sm:grid-cols-2 sm:overflow-visible sm:px-0 sm:pb-0", cols)}>
      {children}
    </div>
  );
}

interface StatCompactProps {
  label: string;
  value: string;
  icon: LucideIcon;
  tone?: StatTone;
  emphasizeValue?: boolean;
  className?: string;
}

export function StatCompact({ label, value, icon: Icon, tone = "default", emphasizeValue, className }: StatCompactProps) {
  return (
    <Card className={cn("gap-0 py-0", className)}>
      <CardContent className="flex items-center gap-3 p-3.5">
        <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-xl", toneBox[tone])}>
          <Icon className="size-4" strokeWidth={2} aria-hidden />
        </span>
        <div className="min-w-0">
          <p className="truncate text-etiqueta font-medium text-muted-foreground">{label}</p>
          <p className={cn("font-mono text-titulo font-bold tracking-tight tabular-nums", emphasizeValue && toneText[tone])}>{value}</p>
        </div>
      </CardContent>
    </Card>
  );
}
