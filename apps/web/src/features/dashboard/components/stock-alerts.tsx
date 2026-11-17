import { AlertTriangle, Layers } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { SectionCard } from "@/components/shared/section-card";
import { cn } from "@/lib/utils";
import type { StockAlert } from "@/types/domain";

export function StockAlerts({ alerts }: { alerts: StockAlert[] }) {
  const critical = alerts.filter((a) => a.stock / a.minimum <= 0.25).length;
  return (
    <SectionCard
      title="Alertas de stock"
      description="Inventario crítico en reposición"
      icon={AlertTriangle}
      iconTone="danger"
      action={<Tag tone="danger" label={`${critical} Críticos`} />}
      contentClassName="flex flex-col gap-3 p-4"
    >
      {alerts.slice(0, 4).map((a) => {
        const pct = Math.round((a.stock / a.minimum) * 100);
        const crit = pct <= 25;
        return (
          <div key={a.id} className="flex flex-col gap-2 rounded-xl bg-secondary p-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate text-xs font-semibold">{a.productName}</p>
                <p className="truncate text-[11px] text-muted-foreground">{a.warehouse}</p>
              </div>
              <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase", crit ? "bg-destructive-soft text-destructive-text " : "bg-warning text-warning-foreground ")}>{a.stock === 0 ? "Agotado" : crit ? "Crítico" : "Bajo"}</span>
            </div>
            <div className="flex items-center justify-between text-xs">
              <span className={cn("font-mono font-bold", crit ? "text-destructive-text" : "text-warning-foreground")}>{a.stock} {a.unit ?? "un"} <span className="font-sans font-normal text-muted-foreground">/ Mín. {a.minimum} {a.unit ?? "un"}</span></span>
              <span className="text-[10px] font-medium text-muted-foreground">{pct}% disponible</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-border"><div className={cn("h-full rounded-full", crit ? "bg-destructive" : "bg-warning-solid")} style={{ width: `${Math.max(pct, 2)}%` }} /></div>
          </div>
        );
      })}
      <Button variant="outline" className="w-full text-xs font-semibold" render={<Link href="/inventario/stock" />} nativeButton={false}><Layers data-icon="inline-start" /> Gestionar reposición</Button>
    </SectionCard>
  );
}
