import type { IconoComponente } from "khipu-design-system/lib/iconos";
import type { ReactNode } from "react";
import { StatCard, type StatTone } from "@/components/shared/stat-card";
import { formatCurrency, formatInteger } from "@/lib/format";
import type { Kpi } from "@/types/domain";

interface KpiCardProps {
  kpi: Kpi;
  icon: IconoComponente;
  tone?: StatTone;
  suffix?: string;
  deltaLabel?: string;
  footer?: ReactNode;
  emphasizeValue?: boolean;
}

export function KpiCard({ kpi, icon, tone = "default", suffix, deltaLabel, footer, emphasizeValue }: KpiCardProps) {
  const value = kpi.format === "currency" ? formatCurrency(kpi.value) : formatInteger(kpi.value);
  return (
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
    />
  );
}
