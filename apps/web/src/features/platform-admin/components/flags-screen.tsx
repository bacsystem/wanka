"use client";

import { History, Plus, ToggleRight } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Switch } from "@/components/ui/switch";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { flags as initial, type FeatureFlag } from "../mocks/ops";

const planKeys = ["basico", "pro", "enterprise"] as const;
const planLabel = { basico: "Básico", pro: "Pro", enterprise: "Enterprise" };

export function FlagsScreen() {
  const [flags, setFlags] = React.useState(initial);
  const toggle = (id: string, plan: (typeof planKeys)[number]) => setFlags((fs) => fs.map((f) => (f.id === id ? { ...f, plans: { ...f.plans, [plan]: !f.plans[plan] } } : f)));
  const setRollout = (id: string, v: number) => setFlags((fs) => fs.map((f) => (f.id === id ? { ...f, rollout: v } : f)));
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Plataforma · Feature flags y módulos por rubro" title="Feature flags y módulos por rubro" description="Activa módulos por plan y por tenant, con lanzamiento gradual por porcentaje y trazabilidad de cambios." status={<StatusBadge tone="info" label={`${flags.length} módulos`} className="rounded-md" />} actions={<><Button variant="outline" className="font-semibold"><History data-icon="inline-start" /> Historial de cambios</Button><Button className="font-semibold" onClick={() => toast.info("Nuevo flag")}><Plus data-icon="inline-start" /> Nuevo flag</Button></>} />
      <SectionCard title="Módulos y disponibilidad por plan" description="Los switches aplican a todos los tenants del plan; el rollout limita el porcentaje que lo recibe." icon={ToggleRight} contentClassName="p-0">
        <div className="overflow-x-auto">
          <Table className="min-w-[960px]">
            <TableHeader><TableRow><TableHead className="text-left">Módulo</TableHead><TableHead className="text-left">Rubro</TableHead>{planKeys.map((p) => <TableHead key={p} className="text-center">Plan {planLabel[p]}</TableHead>)}<TableHead className="text-left">Lanzamiento gradual</TableHead><TableHead className="text-right">Tenants</TableHead><TableHead className="text-left">Último cambio</TableHead></TableRow></TableHeader>
            <TableBody>
              {flags.map((f: FeatureFlag) => (
                <TableRow key={f.id} className="align-top">
                  <TableCell className="px-4 py-3"><p className="font-semibold">{f.name}</p><p className="text-xs text-muted-foreground">{f.description}</p></TableCell>
                  <TableCell className="px-3 py-3 text-xs"><span className="rounded-md bg-accent px-2 py-0.5 font-medium text-accent-foreground">{f.vertical}</span></TableCell>
                  {planKeys.map((p) => <TableCell key={p} className="px-3 py-3 text-center"><Switch checked={f.plans[p]} onCheckedChange={() => { toggle(f.id, p); toast.success(`${f.name}: ${planLabel[p]} ${f.plans[p] ? "desactivado" : "activado"}`); }} aria-label={`${f.name} en plan ${planLabel[p]}`} /></TableCell>)}
                  <TableCell className="px-3 py-3"><div className="flex items-center gap-2"><input type="range" min={0} max={100} step={5} value={f.rollout} onChange={(e) => setRollout(f.id, Number(e.target.value))} aria-label={`Rollout ${f.name}`} className="h-1.5 w-32 accent-primary" /><span className={cn("w-10 font-mono text-xs font-semibold", f.rollout < 100 ? "text-amber-700" : "text-emerald-600")}>{f.rollout}%</span>{f.rollout < 100 ? <StatusBadge tone="warning" label="Gradual" /> : <StatusBadge tone="success" label="GA" />}</div></TableCell>
                  <TableCell className="px-3 py-3 text-right font-mono font-semibold">{f.tenants}</TableCell>
                  <TableCell className="px-4 py-3 text-xs text-muted-foreground">{formatDate(f.updatedAt)}<br />{f.by}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </SectionCard>
    </div>
  );
}
