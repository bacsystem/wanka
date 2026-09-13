import { PawPrint, ShoppingBag, Stethoscope, UtensilsCrossed } from "lucide-react";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import { tenantStatusLabel, verticalLabel, type TenantStatus, type Vertical } from "../mocks/tenants";

export const verticalIcon = { odontologia: Stethoscope, veterinaria: PawPrint, restaurante: UtensilsCrossed, retail: ShoppingBag } as const;
const verticalTone: Record<Vertical, string> = {
  odontologia: "border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-900 dark:bg-sky-950/60 dark:text-sky-300",
  veterinaria: "border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  restaurante: "border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-300",
  retail: "border-fuchsia-200 bg-fuchsia-50 text-fuchsia-800 dark:border-fuchsia-900 dark:bg-fuchsia-950/60 dark:text-fuchsia-300",
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
