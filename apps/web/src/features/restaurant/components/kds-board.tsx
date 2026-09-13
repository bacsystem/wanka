"use client";

import { AlertTriangle, Bell, Check, Play } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { cn } from "@/lib/utils";
import type { KdsTicket } from "../mocks/floor";

const lanes: { id: KdsTicket["lane"]; label: string; head: string; dot: string; chip: string; card: string }[] = [
  { id: "pendiente", label: "Pendiente en cola", head: "bg-muted/60", dot: "bg-slate-400", chip: "bg-muted text-foreground", card: "" },
  { id: "preparacion", label: "En preparación", head: "bg-amber-50 dark:bg-amber-950/30", dot: "bg-amber-500", chip: "bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-200", card: "border-emerald-200 dark:border-emerald-900" },
  { id: "listo", label: "Listo para servir (pase)", head: "bg-emerald-50 dark:bg-emerald-950/30", dot: "bg-emerald-500", chip: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200", card: "border-emerald-200 dark:border-emerald-900" },
];

export function KdsBoard({ tickets: initial }: { tickets: KdsTicket[] }) {
  const [tickets, setTickets] = React.useState(initial);
  const move = (id: string, lane: KdsTicket["lane"]) => setTickets((ts) => ts.map((t) => (t.id === id ? { ...t, lane } : t)));
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      {lanes.map((lane) => {
        const rows = tickets.filter((t) => t.lane === lane.id);
        return (
          <section key={lane.id} aria-label={lane.label} className="flex min-w-0 flex-col gap-2 rounded-xl border bg-card p-3 shadow-xs">
            <header className={cn("flex items-center justify-between rounded-lg px-3 py-2", lane.head)}><h3 className="flex items-center gap-2 text-sm font-bold"><span className={cn("size-2 rounded-full", lane.dot)} />{lane.label}</h3><span className={cn("rounded px-2 py-0.5 font-mono text-[11px] font-semibold", lane.chip)}>{rows.length} ticket{rows.length === 1 ? "" : "s"}</span></header>
            {rows.length === 0 ? <p className="rounded-md border border-dashed p-6 text-center text-xs text-muted-foreground">Sin tickets</p> : null}
            {rows.map((t) => { const late = t.minutes >= 15; return (
              <article key={t.id} className={cn("rounded-lg border bg-muted/30 p-3 text-sm", lane.card, late && "border-destructive bg-rose-50/60 ring-1 ring-destructive/30 dark:bg-rose-950/20")}>
                <div className="flex items-center justify-between gap-2"><p className="flex items-center gap-2 font-semibold"><span className={cn("rounded px-2 py-0.5 text-xs font-bold text-background", late ? "bg-destructive" : "bg-foreground")}>{t.table}</span><span className="font-mono text-xs font-normal text-muted-foreground">Ticket {t.id}</span></p><span className={cn("font-mono text-xs tabular-nums", late ? "font-semibold text-destructive" : "text-muted-foreground")}>{String(t.minutes).padStart(2, "0")}:{t.id.endsWith("9") ? "22" : "12"} min</span></div>
                <p className="text-xs text-muted-foreground">Mozo {t.waiter}</p>
                <ul className="mt-2 flex flex-col gap-1">{t.items.map((i, k) => <li key={k} className="flex items-center justify-between gap-2 text-xs"><span><span className="font-mono font-semibold">{i.qty}×</span> {i.name}</span><span className="flex items-center gap-1">{i.note ? <span className={cn("text-muted-foreground", i.note === "Retrasado" && "font-medium text-destructive")}>{i.note}</span> : null}<Badge variant="outline">{i.station}</Badge></span></li>)}</ul>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {lane.id === "pendiente" ? <Button size="xs" onClick={() => move(t.id, "preparacion")}><Play data-icon="inline-start" /> Iniciar preparación</Button> : null}
                  {lane.id === "preparacion" ? <><Button size="xs" onClick={() => move(t.id, "listo")}><Check data-icon="inline-start" /> Marcar todo listo</Button>{late ? <Button size="xs" variant="outline" className="text-destructive" onClick={() => toast.info(`${t.table} priorizada en pase`)}><AlertTriangle data-icon="inline-start" /> Priorizar</Button> : null}<Button size="xs" variant="ghost" onClick={() => toast.success(`Aviso enviado a ${t.waiter}`)}><Bell data-icon="inline-start" /> Avisar mozo</Button></> : null}
                  {lane.id === "listo" ? <Button size="xs" variant="outline" onClick={() => { setTickets((ts) => ts.filter((x) => x.id !== t.id)); toast.success(`${t.table} servida`); }}><Check data-icon="inline-start" /> Servido en mesa</Button> : null}
                </div>
                {late ? <StatusBadge className="mt-2" tone="danger" label="> 15 min · retrasado" /> : null}
              </article>
            ); })}
          </section>
        );
      })}
    </div>
  );
}
