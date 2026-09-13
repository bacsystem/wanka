import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { StatCard, type StatTone } from "@/components/shared/stat-card";
import { formatCurrency, formatInteger } from "@/lib/format";
import type { Kpi } from "@/types/domain";

interface KpiCardProps {
  kpi: Kpi;
  icon: LucideIcon;
  tone?: StatTone;
  suffix?: string;
  deltaLabel?: string;
  footer?: ReactNode;
  emphasizeValue?: boolean;
  className?: string;
}

export function KpiCard({ kpi, icon, tone = "default", suffix, deltaLabel, footer, emphasizeValue, className }: KpiCardProps) {
  const value = kpi.format === "currency" ? formatCurrency(kpi.value) : formatInteger(kpi.value);
  return (
    <div data-testid="kpi-card" className="contents">
      <StatCard
        label={kpi.label}
        value={value}
        suffix={suffix}
        icon={icon}
        tone={tone}
        deltaPercent={kpi.deltaPercent}
        deltaLabel={deltaLabel}
        hint={kpi.hint}
        footer={footer}
        emphasizeValue={emphasizeValue}
        className={className}
      />
    </div>
  );
}
