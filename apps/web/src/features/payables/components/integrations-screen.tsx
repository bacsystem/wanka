"use client";

import { Plug, RefreshCw, Settings2 } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { integrations as data, type Integration } from "../mocks/payables";

const meta: Record<string, { label: string; tone: BadgeTone }> = { conectado: { label: "Conectado", tone: "success" }, atencion: { label: "Requiere atención", tone: "warning" }, desconectado: { label: "Desconectado", tone: "neutral" } };

export function IntegrationsScreen() {
  const [items, setItems] = React.useState<Integration[]>(data);
  return (
    <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      {items.map((i) => (
        <li key={i.id} className="flex flex-col gap-2 rounded-lg border bg-card p-4 text-sm">
          <div className="flex items-start justify-between gap-2"><div className="flex items-center gap-2"><span className="flex size-9 items-center justify-center rounded-md bg-accent text-accent-foreground"><Plug className="size-4" strokeWidth={1.5} /></span><div><p className="font-semibold">{i.name}</p><p className="text-xs text-muted-foreground">{i.kind}</p></div></div><StatusBadge tone={meta[i.status].tone} dot label={meta[i.status].label} /></div>
          <p className="text-xs text-muted-foreground">{i.detail}</p>
          <p className="text-xs text-muted-foreground">Última sincronización: {i.last}</p>
          <div className="mt-auto flex gap-1.5 pt-1">
            <Button size="xs" variant="outline"><Settings2 data-icon="inline-start" /> Configurar</Button>
            {i.status === "desconectado" ? <Button size="xs" onClick={() => { setItems((it) => it.map((x) => (x.id === i.id ? { ...x, status: "conectado" as const, last: "ahora" } : x))); toast.success(`${i.name} conectado`); }}>Conectar</Button> : <Button size="xs" variant="ghost" onClick={() => toast.success(`${i.name}: conexión correcta (${Math.round(120 + Math.random() * 200)} ms)`)}><RefreshCw data-icon="inline-start" /> Probar conexión</Button>}
          </div>
        </li>
      ))}
    </ul>
  );
}
