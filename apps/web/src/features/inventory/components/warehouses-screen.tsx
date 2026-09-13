"use client";

import { Armchair, Building2, ClipboardCheck, Gavel, History, MapPin, MoreHorizontal, Pill, PlusCircle, RefreshCw, ShieldAlert, Truck, UserCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { WarehouseCard, WarehouseKind } from "../mocks/warehouses";
import { NewWarehouseSheet } from "./new-warehouse-sheet";

const kindMeta: Record<WarehouseKind, { label: string; tone: BadgeTone; icon: typeof Pill; border: string; box: string }> = {
  principal: { label: "Principal", tone: "success", icon: Pill, border: "border-l-primary", box: "bg-accent text-primary" },
  subalmacen: { label: "Subalmacén operativo", tone: "info", icon: Armchair, border: "border-l-blue-500", box: "bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300" },
  sucursal: { label: "Sucursal", tone: "info", icon: Building2, border: "border-l-teal-400", box: "bg-accent text-primary" },
  restringido: { label: "Área restringida", tone: "danger", icon: ShieldAlert, border: "border-l-rose-500", box: "bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-300" },
};
const siteOptions = [{ value: "all", label: "Todas las sedes (2)" }, { value: "Sede Miraflores", label: "Sede Miraflores (principal)" }, { value: "Sede San Isidro", label: "Sede San Isidro" }];

interface Props {
  cards: WarehouseCard[];
  establishments: { code: string; label: string; address: string; ubigeo: string; status: string; kind: string; linked: number }[];
  custodians: { initials: string; name: string; detail: string }[];
  transfers: { guide: string; route: string; status: string; items: number }[];
}

export function WarehousesScreen({ cards, establishments, custodians, transfers }: Props) {
  const [site, setSite] = React.useState("all");
  const [newOpen, setNewOpen] = React.useState(false);
  const rows = cards.filter((c) => site === "all" || c.site === site);

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Filtrar por sede:</span>
          <Select value={site} onValueChange={(v) => setSite(String(v))} items={siteOptions}>
            <SelectTrigger className="w-56" aria-label="Sede"><SelectValue /></SelectTrigger>
            <SelectContent>{siteOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline"><History data-icon="inline-start" /> Auditoría de custodia</Button>
          <Button variant="outline" onClick={() => toast.success("Ficha RUC sincronizada · 2 establecimientos anexos")}><RefreshCw data-icon="inline-start" /> Sincronizar ficha RUC</Button>
          <Button onClick={() => setNewOpen(true)}><PlusCircle data-icon="inline-start" /> Nuevo almacén / sede</Button>
        </div>
      </div>

      <NewWarehouseSheet open={newOpen} onOpenChange={setNewOpen} />
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <h2 className="flex items-center gap-2 text-lg font-bold">Unidades de almacenamiento <StatusBadge tone="info" label={`${rows.length} activas`} className="rounded-md" /></h2>
          <ul className="flex flex-col gap-4">
            {rows.map((c) => {
              const m = kindMeta[c.kind];
              return (
                <li key={c.id} className={cn("flex min-w-0 flex-col gap-3 rounded-xl border border-l-4 bg-card p-4 shadow-xs", m.border)}>
                  <div className="flex flex-wrap items-start gap-3">
                    <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-lg", m.box)}><m.icon className="size-5" strokeWidth={2} /></span>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-base font-bold">{c.name}</h3>
                        <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase", c.kind === "principal" ? "bg-primary text-primary-foreground" : c.kind === "restringido" ? "bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300" : "bg-accent text-primary dark:bg-muted")}>{m.label}</span>
                        {c.sunatAnnex ? <Tag tone="info" label={`SUNAT Anexo: ${c.sunatAnnex}`} /> : null}
                      </div>
                      <p className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground"><MapPin className="size-3 shrink-0" /> {c.site} • {c.address}</p>
                      <p className="mt-0.5 text-xs text-muted-foreground">{c.description}</p>
                    </div>
                    <div className="text-right"><p className="text-[11px] text-muted-foreground">{c.valueLabel}</p><p className={cn("font-mono text-xl font-bold tabular-nums", c.kind === "restringido" ? "text-destructive" : "text-primary")}>{formatCurrency(c.value)}</p><p className={cn("text-xs font-semibold", c.kind === "restringido" ? "text-muted-foreground" : "text-primary")}>{c.skusLabel}</p></div>
                  </div>
                  <dl className="grid gap-3 border-t pt-3 text-xs sm:grid-cols-3">
                    {c.facts.map((f, i) => (<div key={f.label} className="flex items-start gap-2"><span className="mt-0.5 text-muted-foreground">{[<UserCheck key="a" className="size-4" />, <Truck key="b" className="size-4" />, <ClipboardCheck key="c" className="size-4" />][i % 3]}</span><span><dt className="text-muted-foreground">{f.label}</dt><dd className={cn("text-sm font-semibold", /GRE|habilitada|SUNAT|Descargo|Vencimiento/i.test(f.value) && (c.kind === "restringido" ? "text-destructive" : "text-primary"))}>{f.value}</dd></span></div>))}
                  </dl>
                  <div className="flex flex-wrap items-center gap-2 pt-1">
                    {c.actions.map((a, i) => {
                      const href = a.startsWith("Ver inventario") || a.startsWith("Ver ítems") ? "/inventario/stock" : a === "Recepcionar traslado" ? "/inventario/recepcion/T001-000428" : a === "Auditoría física" || a === "Arqueo de turno" ? "/inventario/toma-fisica" : a === "Configurar series GRE" ? "/configuracion/sunat" : a === "Transferir stock" ? "/inventario/stock" : null;
                      const cls = i === 0 ? "bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted" : "font-semibold";
                      return href ? (
                        <Button key={a} size="sm" variant={i === 0 ? "secondary" : "outline"} className={cls} render={<Link href={href} />} nativeButton={false}>{a}</Button>
                      ) : (
                        <Button key={a} size="sm" variant={i === 0 ? "secondary" : "outline"} className={cls} onClick={() => toast.info(a)}>{a}</Button>
                      );
                    })}
                    <Button variant="ghost" size="icon-sm" className="ml-auto" aria-label="Más opciones"><MoreHorizontal /></Button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Establecimientos SUNAT" icon={Gavel} contentClassName="p-0">
            <p className="border-b px-4 py-2 text-xs text-muted-foreground">RUC 20608941235 · locales validados en la ficha RUC electrónica.</p>
            <ul className="divide-y">
              {establishments.map((e) => (
                <li key={e.code} className="flex flex-col gap-1 px-4 py-3 text-sm">
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-2"><Badge variant="outline" className="font-mono">{e.code}</Badge><span className="font-medium">{e.label}</span></span>
                    <StatusBadge tone="success" dot label={e.status} />
                  </div>
                  <p className="text-xs text-muted-foreground">{e.address} · Ubigeo {e.ubigeo}</p>
                  <p className="text-xs text-muted-foreground">Activo · {e.kind} · almacenes vinculados: {e.linked}</p>
                </li>
              ))}
            </ul>
          </SectionCard>
          <SectionCard title="Custodios con firma digital" icon={UserCheck} action={<Button variant="ghost" size="sm">+ Gestionar</Button>} contentClassName="p-0">
            <ul className="divide-y">
              {custodians.map((c) => (
                <li key={c.initials} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                  <InitialsAvatar initials={c.initials} />
                  <div className="min-w-0"><p className="truncate font-medium">{c.name}</p><p className="text-xs text-muted-foreground">{c.detail}</p></div>
                  <StatusBadge className="ml-auto" tone="success" label="Firma vigente" />
                </li>
              ))}
            </ul>
            <p className="border-t px-4 py-2 text-xs text-muted-foreground"><ClipboardCheck className="mr-1 inline size-3" /> Obligatoriedad GRE: todo traslado entre locales 0000 y 0001 requiere guía de remisión electrónica remitente.</p>
          </SectionCard>
          <SectionCard title="Traslados en tránsito" icon={Truck} contentClassName="p-0">
            <ul className="divide-y">
              {transfers.map((t) => (
                <li key={t.guide} className="flex items-center justify-between gap-2 px-4 py-3 text-sm">
                  <div><Link href={`/inventario/recepcion/${t.guide.replace("GRE ", "")}`} className="font-mono text-xs font-medium text-primary hover:underline">{t.guide}</Link><p className="text-xs text-muted-foreground">{t.route} · {t.items} ítems</p></div>
                  <Button size="xs" variant="outline" render={<Link href={`/inventario/recepcion/${t.guide.replace("GRE ", "")}`} />} nativeButton={false}>Recepcionar</Button>
                </li>
              ))}
            </ul>
          </SectionCard>
        </div>
      </div>
    </>
  );
}
