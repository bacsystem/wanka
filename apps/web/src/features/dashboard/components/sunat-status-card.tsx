"use client";

import { Clock, FileCode, RefreshCw } from "lucide-react";
import * as React from "react";
import { StateIcon } from "@/components/shared/dynamic-icon";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { SectionCard } from "@/components/shared/section-card";
import type { SunatQueueItem } from "@/types/domain";

export function SunatStatusCard({ queue }: { queue: SunatQueueItem[] }) {
  const [syncing, setSyncing] = React.useState(false);
  const retry = (label: string) => { setSyncing(true); setTimeout(() => { setSyncing(false); toast.success(`${label} reenviado a SUNAT`, { description: "CDR recibido · aceptado" }); }, 1400); };
  const failed = queue.filter((q) => q.status === "rechazado");
  const pending = queue.filter((q) => q.status === "pendiente");
  const first = pending[0];
  return (
    <SectionCard
      title="Estado de envíos SUNAT"
      description="Conexión OSE Bizlinks / Nubefact"
      icon={RefreshCw}
      action={<span className="flex items-center gap-1 text-[11px] font-medium text-success-foreground"><StateIcon state={syncing ? "syncing" : "ok"} className="size-3.5" />OSE Online</span>}
      contentClassName="flex flex-col gap-3 p-4"
    >
      {first ? (
        <div className="flex flex-col gap-2.5 rounded-lg border border-transparent bg-warning p-3">
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1.5 text-xs font-semibold text-warning-foreground"><Clock className="size-3.5" />{pending.length} comprobante{pending.length === 1 ? "" : "s"} en cola OSE</span>
            <span className="font-mono text-xs font-bold text-warning-foreground">S/ 3,420.00</span>
          </div>
          <div className="flex items-center justify-between text-xs text-warning-foreground "><span>Factura {first.documentLabel}</span><span className="text-[11px] font-medium">Espera: 3 min</span></div>
          <Button className="w-full text-xs font-semibold" disabled={syncing} onClick={() => retry(first.documentLabel)}><StateIcon state={syncing ? "syncing" : "pending"} className="text-current" data-icon="inline-start" /> {syncing ? "Transmitiendo al OSE…" : "Reintentar envío ahora a SUNAT"}</Button>
        </div>
      ) : null}
      {failed.map((f) => (
        <div key={f.id} className="flex flex-col gap-2 rounded-lg border border-transparent bg-destructive-soft p-3">
          <div className="flex items-center justify-between gap-2 text-xs font-semibold text-destructive-text"><span className="font-mono">{f.documentLabel}</span><span>Rechazado</span></div>
          <p className="text-xs text-destructive-text ">{f.message}</p>
          <Button size="xs" variant="outline" className="w-fit" onClick={() => toast.info(`Corrige ${f.documentLabel} y reintenta`)}><RefreshCw data-icon="inline-start" /> Reintentar</Button>
        </div>
      ))}
      <div className="grid grid-cols-2 gap-2 text-xs">
        <div className="rounded-xl bg-secondary p-2.5"><p className="text-[11px] text-muted-foreground">Rechazados 24h</p><p className="mt-0.5 text-sm font-bold">{failed.length} docs</p></div>
        <div className="rounded-xl bg-secondary p-2.5"><p className="text-[11px] text-muted-foreground">Latencia OSE</p><p className="mt-0.5 font-mono text-sm font-bold">1.2s <span className="font-sans text-[10px] font-normal text-muted-foreground">prom.</span></p></div>
      </div>
      <div className="flex items-center justify-between gap-2 rounded-xl bg-secondary p-2.5 text-xs">
        <span className="flex min-w-0 items-center gap-2"><FileCode className="size-4 shrink-0 text-muted-foreground" /><span className="min-w-0"><span className="block truncate font-medium">Certificado digital tributario</span><span className="block text-[10px] text-muted-foreground">Firma electrónica activa</span></span></span>
        <span className="shrink-0 rounded border border-transparent bg-success px-2 py-0.5 font-mono text-xs font-semibold text-success-foreground ">Vence en 245 días</span>
      </div>
    </SectionCard>
  );
}
