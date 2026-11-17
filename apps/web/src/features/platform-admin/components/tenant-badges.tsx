import { PawPrint, ShoppingBag, Stethoscope, UtensilsCrossed } from "lucide-react";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import { tenantStatusLabel, verticalLabel, type TenantStatus, type Vertical } from "../mocks/tenants";

export const verticalIcon = { odontologia: Stethoscope, veterinaria: PawPrint, restaurante: UtensilsCrossed, retail: ShoppingBag } as const;
const verticalTone: Record<Vertical, string> = {
  odontologia: "border-info-foreground/20 bg-info text-info-foreground dark:border-info-foreground/20 dark:bg-info dark:text-info-foreground",
  veterinaria: "border-transparent bg-warning text-warning-foreground",
  restaurante: "border-transparent bg-success text-success-foreground",
  retail: "border-accent-border bg-acento-suave text-acento-superficie-texto dark:border-accent-border dark:bg-acento-suave dark:text-acento-superficie-texto",
};
const statusTone: Record<TenantStatus, BadgeTone> = { activa: "success", trial: "info", suspendida: "danger", cuota: "warning" };

export function VerticalBadge({ vertical, className }: { vertical: Vertical; className?: string }) {
  const Icon = verticalIcon[vertical];
  return <span className={cn("inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-semibold", verticalTone[vertical], className)}><Icon className="size-3" /> {verticalLabel[vertical]}</span>;
}
export function TenantStatusBadge({ status }: { status: TenantStatus }) {
  return <StatusBadge tone={statusTone[status]} dot label={tenantStatusLabel[status]} />;
}
export function TenantAvatar({ code, vertical, className }: { code: string; vertical: Vertical; className?: string }) {
  return <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg border text-xs font-bold", verticalTone[vertical], className)}>{code}</span>;
}
