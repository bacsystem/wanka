import { CheckCircle2, Clock, XCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { toneClass } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import type { SunatStatus } from "@/types/domain";

const config = {
  aceptado: { label: "Aceptado", icon: CheckCircle2, className: toneClass.success },
  pendiente: { label: "Pendiente", icon: Clock, className: toneClass.warning },
  rechazado: { label: "Rechazado", icon: XCircle, className: toneClass.danger },
} satisfies Record<SunatStatus, { label: string; icon: typeof CheckCircle2; className: string }>;

export function SunatStatusBadge({ status, className }: { status: SunatStatus; className?: string }) {
  const { label, icon: Icon, className: statusClass } = config[status];
  return (
    <Badge variant="outline" className={cn("gap-1 font-medium", statusClass, className)} data-status={status}>
      <Icon strokeWidth={2} aria-hidden />
      {label}
    </Badge>
  );
}
