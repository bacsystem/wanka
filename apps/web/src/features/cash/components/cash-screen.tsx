"use client";

import { Banknote, Clock, CreditCard, Landmark, Lock, Printer, QrCode, RefreshCw, ShieldCheck, Store, UserCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { NumberInput } from "@/components/shared/number-input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { CashMovement } from "../mocks/cash";
import { shift } from "../mocks/cash";
import { FilterBar } from "@/components/shared/filter-bar";

const channelIcon = { cash: Banknote, qr: QrCode, card: CreditCard, transfer: Landmark } as const;
const methodLabel = { efectivo: "Efectivo", yape: "Yape", plin: "Plin", tarjeta: "Tarjeta", transferencia: "Transferencia" } as const;
const statusMeta: Record<CashMovement["status"], { label: string; tone: BadgeTone }> = {
  valido: { label: "SUNAT válido", tone: "success" },
  pendiente: { label: "Pendiente", tone: "warning" },
  anulado: { label: "Anulado", tone: "danger" },
};

export function CashScreen({ movements }: { movements: CashMovement[] }) {
  const [counts, setCounts] = React.useState<number[]>(shift.denominations.map((d) => d.count));
  const [filter, setFilter] = React.useState("all");
  const counted = counts.reduce((s, c, i) => s + c * shift.denominations[i].value, 0);
  const cashTheoretical = shift.channels[0].theoretical;
  const diff = counted - cashTheoretical;

  const rows = movements.filter((m) => filter === "all" || (filter === "ingreso" ? m.kind === "ingreso" : m.kind === "egreso"));

  const columns: Column<CashMovement>[] = [
    { key: "time", header: "Hora / doc.", cell: (m) => <TwoLine primary={<span className="font-mono tabular-nums">{m.time}</span>} secondary={m.doc} mono /> },
    { key: "concept", header: "Concepto y paciente / proveedor", cell: (m) => <TwoLine primary={<span className="font-medium">{m.concept}</span>} secondary={m.party} /> },
    { key: "method", header: "Medio de pago", cell: (m) => methodLabel[m.method] },
    { key: "status", header: "Estado", cell: (m) => <StatusBadge tone={statusMeta[m.status].tone} label={statusMeta[m.status].label} /> },
    { key: "amount", header: "Monto (PEN)", align: "right", cell: (m) => <span className={cn("font-mono font-medium tabular-nums", m.kind === "egreso" && "text-destructive", m.status === "anulado" && "line-through opacity-60")}>{m.kind === "egreso" ? "−" : ""}{formatCurrency(m.amount)}</span> },
  ];

  return (
    <>
      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Conciliación por canal y medio de pago" className="xl:col-span-2" contentClassName="p-0">
          <ul className="grid divide-y sm:grid-cols-2 sm:divide-y-0">
            {shift.channels.map((c, i) => {
              const Icon = channelIcon[c.id as keyof typeof channelIcon];
              const ok = c.counted === c.theoretical;
              return (
                <li key={c.id} className={cn("flex flex-col gap-2 p-4", i % 2 === 0 && "sm:border-r", i < 2 && "sm:border-b")}>
                  <div className="flex items-center justify-between">
                    <span className="flex items-center gap-2 text-sm font-medium"><Icon className="size-4 text-primary" strokeWidth={1.5} /> {c.label}</span>
                    <StatusBadge tone={ok ? "success" : "danger"} label={ok ? "Cuadrado" : "Diferencia"} />
                  </div>
                  <dl className="grid grid-cols-3 gap-2 text-sm">
                    <div><dt className="text-xs text-muted-foreground">Teórico</dt><dd className="font-mono tabular-nums">{formatCurrency(c.theoretical)}</dd></div>
                    <div><dt className="text-xs text-muted-foreground">Confirmado</dt><dd className="font-mono tabular-nums">{formatCurrency(c.counted)}</dd></div>
                    <div><dt className="text-xs text-muted-foreground">Diferencia</dt><dd className={cn("font-mono tabular-nums", ok ? "text-success" : "text-destructive")}>{formatCurrency(c.counted - c.theoretical)}</dd></div>
                  </dl>
                  <p className="text-xs text-muted-foreground">{c.detail}</p>
                </li>
              );
            })}
          </ul>
        </SectionCard>

        <SectionCard title="Desglose de efectivo físico" contentClassName="p-0">
          <ul className="max-h-72 divide-y overflow-y-auto">
            {shift.denominations.map((d, i) => (
              <li key={d.label} className="flex items-center gap-3 px-4 py-1.5 text-sm">
                <span className="w-16 font-mono tabular-nums">{d.label}</span>
                <span className="text-muted-foreground">×</span>
                <NumberInput
                  size="sm" value={counts[i]} aria-label={`Cantidad de ${d.label}`}
                  onValueChange={(n) => setCounts((c) => c.map((v, j) => (j === i ? n : v)))}
                  className="w-16 text-right font-mono"
                />
                <span className="ml-auto font-mono tabular-nums">{formatCurrency(d.value * counts[i])}</span>
              </li>
            ))}
          </ul>
          <dl className="border-t bg-muted/30 p-4 text-sm">
            <div className="flex justify-between"><dt className="text-muted-foreground">Teórico</dt><dd className="font-mono tabular-nums">{formatCurrency(cashTheoretical)}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Físico recontado</dt><dd className="font-mono font-medium tabular-nums" data-testid="cash-counted">{formatCurrency(counted)}</dd></div>
            <div className="mt-1 flex justify-between border-t pt-1 font-medium"><dt>Diferencia</dt><dd className={cn("font-mono tabular-nums", diff === 0 ? "text-success" : "text-destructive")}>{formatCurrency(diff)} {diff === 0 ? "(exacto)" : diff > 0 ? "(sobrante)" : "(faltante)"}</dd></div>
          </dl>
        </SectionCard>
      </div>

      <SectionCard
        title="Movimientos registrados en el turno"
        action={<div className="flex flex-wrap items-center gap-2"><Button size="xs" variant="ghost" render={<Link href="/finanzas/caja/historial" />} nativeButton={false}>Historial de turnos →</Button>
          <Tabs value={filter} onValueChange={(v) => setFilter(String(v))}>
            <TabsList><TabsTrigger value="all">Todos</TabsTrigger><TabsTrigger value="ingreso">Ingresos</TabsTrigger><TabsTrigger value="egreso">Vales de salida</TabsTrigger></TabsList>
          </Tabs></div>
        }
        contentClassName="p-0"
      >
        <DataTable
          columns={columns} rows={rows} rowKey={(m) => m.id} minWidth="720px"
          mobileCard={(m) => (
            <div className="flex items-center gap-3">
              <span className="w-12 font-mono text-xs tabular-nums">{m.time}</span>
              <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{m.concept}</p><p className="truncate text-xs text-muted-foreground">{m.party} · {methodLabel[m.method]}</p></div>
              <span className={cn("font-mono text-sm font-medium tabular-nums", m.kind === "egreso" && "text-destructive")}>{m.kind === "egreso" ? "−" : ""}{formatCurrency(m.amount)}</span>
            </div>
          )}
        />
      </SectionCard>

      <FilterBar stack>
        <div className="grid gap-3 lg:grid-cols-2">
        <div className="flex min-w-0 items-center gap-3 rounded-lg bg-muted/50 p-3"><span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground"><UserCheck className="size-5" /></span><div className="min-w-0 text-xs"><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Cajero(a) responsable</p><p className="text-sm font-bold">{shift.cashier}</p><p className="text-primary">Turno tarde • DNI 45892104</p></div></div>
        <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg bg-muted/50 p-3"><span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-accent text-primary"><ShieldCheck className="size-5" /></span><div className="min-w-0 text-xs"><p className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Supervisor de turno / caja</p><p className="text-sm font-bold">Mag. Roberto Benavides</p><p className="text-muted-foreground">Auditoría interna • Sede Central</p></div></div>
        </div>
        <div className="flex flex-wrap justify-end gap-2 max-sm:flex-col">
          <Button variant="secondary" className="font-semibold" onClick={() => toast.info("Imprimiendo reporte X del turno")}><Printer data-icon="inline-start" /> Imprimir X de turno</Button>
          <Button variant="secondary" className="font-semibold" onClick={() => toast.success("Lotes Niubiz / Izipay sincronizados")}><RefreshCw data-icon="inline-start" /> Sincronizar lotes POS</Button>
          <Button variant="secondary" className="bg-accent font-semibold text-primary hover:bg-accent/70 dark:bg-muted"><ShieldCheck data-icon="inline-start" /> Generar resumen SUNAT (RC)</Button>
          <Button className="font-semibold" disabled={diff !== 0} onClick={() => toast.success("Turno cerrado · Z-Report firmado", { description: `Saldo final ${formatCurrency(shift.expected)}` })}><Lock data-icon="inline-start" /> Firmar y emitir Z-Report</Button>
        </div>
      </FilterBar>
    </>
  );
}

export function ShiftBadge() {
  return (
    <div className="flex flex-wrap gap-2">
      <span className="flex items-center gap-2 rounded-lg border bg-card px-3 py-1.5 text-xs shadow-xs"><Store className="size-4 text-muted-foreground" /><span className="flex flex-col leading-tight"><span className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Sede</span><span className="font-semibold">{shift.site}</span></span></span>
      <span className="flex items-center gap-2 rounded-lg border bg-card px-3 py-1.5 text-xs shadow-xs"><Clock className="size-4 text-muted-foreground" /><span className="flex flex-col leading-tight"><span className="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Turno y terminal</span><span className="font-semibold">{shift.terminal}</span></span></span>
    </div>
  );
}
