"use client";

import { Download, LockOpen, Printer } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { Toolbar } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Shift } from "../mocks/shifts";

const meta: Record<Shift["status"], { label: string; tone: BadgeTone }> = { cuadrado: { label: "Cuadrado", tone: "success" }, sobrante: { label: "Sobrante", tone: "warning" }, faltante: { label: "Faltante", tone: "danger" } };

export function ShiftHistory({ shifts }: { shifts: Shift[] }) {
  const [selId, setSelId] = React.useState(shifts[0].id);
  const [reg, setReg] = React.useState("all");
  const rows = shifts.filter((s) => reg === "all" || s.register === reg);
  const z = shifts.find((s) => s.id === selId)!;
  const cols: Column<Shift>[] = [
    { key: "d", header: "Fecha / caja", cell: (s) => <button type="button" onClick={() => setSelId(s.id)} className={cn("text-left", selId === s.id && "font-semibold text-primary")}><TwoLine primary={formatDate(s.date)} secondary={`${s.register} · ${s.cashier}`} mono /></button> },
    { key: "o", header: "Apertura", align: "right", cell: (s) => <span className="font-mono">{formatCurrency(s.opening)}</span> },
    { key: "v", header: "Ventas", align: "right", cell: (s) => <span className="font-mono">{formatCurrency(s.sales)}</span> },
    { key: "e", header: "Egresos", align: "right", cell: (s) => <span className="font-mono text-destructive">{s.expenses ? `− ${formatCurrency(s.expenses)}` : "—"}</span> },
    { key: "x", header: "Esperado", align: "right", cell: (s) => <span className="font-mono">{formatCurrency(s.expected)}</span> },
    { key: "c", header: "Contado", align: "right", cell: (s) => <span className="font-mono">{formatCurrency(s.counted)}</span> },
    { key: "dif", header: "Diferencia", align: "right", cell: (s) => <span className={cn("font-mono font-semibold", s.counted - s.expected < 0 && "text-destructive", s.counted - s.expected > 0 && "text-warning")}>{formatCurrency(s.counted - s.expected)}</span> },
    { key: "s", header: "Estado", cell: (s) => <StatusBadge tone={meta[s.status].tone} dot label={meta[s.status].label} /> },
    { key: "z", header: "Z-report", cell: (s) => <Badge variant="outline" className="font-mono">{s.z}</Badge> },
  ];
  const diff = z.counted - z.expected;
  return (
    <div className="grid gap-4 xl:grid-cols-5">
      <SectionCard title="Turnos cerrados" className="xl:col-span-3" contentClassName="p-0">
        <Toolbar><Select value={reg} onValueChange={(v) => setReg(String(v))} items={[{ value: "all", label: "Caja: todas" }, { value: "CAJA-01", label: "CAJA-01" }, { value: "CAJA-02", label: "CAJA-02" }]}><SelectTrigger className="w-40" aria-label="Caja"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">Caja: todas</SelectItem><SelectItem value="CAJA-01">CAJA-01</SelectItem><SelectItem value="CAJA-02">CAJA-02</SelectItem></SelectContent></Select><Badge variant="outline" className="font-mono">01/09 – 12/09</Badge></Toolbar>
        <DataTable columns={cols} rows={rows} rowKey={(s) => s.id} minWidth="900px" mobileCard={(s) => <button type="button" onClick={() => setSelId(s.id)} className="flex w-full items-center justify-between gap-3 text-left"><div><p className="font-mono text-xs">{formatDate(s.date)} · {s.register}</p><p className="text-sm">{s.cashier} · ventas {formatCurrency(s.sales)}</p></div><StatusBadge tone={meta[s.status].tone} dot label={`${meta[s.status].label} ${formatCurrency(s.counted - s.expected)}`} /></button>} />
      </SectionCard>
      <SectionCard title={`Z-report ${z.z}`} className="xl:col-span-2" action={<StatusBadge tone={meta[z.status].tone} dot label={meta[z.status].label} />} contentClassName="flex flex-col gap-3 p-4 text-sm">
        <p className="text-xs text-muted-foreground">{formatDate(z.date)} · {z.register} · cajero {z.cashier} · supervisor {z.supervisor}</p>
        <dl className="grid grid-cols-2 gap-2">{z.byMethod.map((m) => <div key={m.method} className="rounded-md border p-2"><dt className="text-xs text-muted-foreground">{m.method}</dt><dd className="font-mono font-medium">{formatCurrency(m.amount)}</dd></div>)}</dl>
        <div className="grid grid-cols-3 gap-2 text-xs">{z.docs.map((d) => <div key={d.type} className="rounded-md bg-muted/40 p-2"><p className="text-muted-foreground">{d.type}</p><p className="font-mono text-base font-semibold">{d.count}</p></div>)}</div>
        <dl className="grid gap-1 text-xs"><div className="flex justify-between"><dt className="text-muted-foreground">Anulaciones</dt><dd className="font-mono">{z.voids}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Descuentos</dt><dd className="font-mono">{formatCurrency(z.discounts)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Propinas</dt><dd className="font-mono">{formatCurrency(z.tips)}</dd></div>{z.withdrawals.map((w, i) => <div key={i} className="flex justify-between"><dt className="text-muted-foreground">Retiro · {w.reason} ({w.by})</dt><dd className="font-mono">− {formatCurrency(w.amount)}</dd></div>)}<div className="mt-1 flex justify-between border-t pt-1 font-semibold"><dt>Diferencia final</dt><dd className={cn("font-mono", diff < 0 && "text-destructive", diff > 0 && "text-warning", diff === 0 && "text-success")}>{formatCurrency(diff)}</dd></div></dl>
        <p className="font-mono text-[11px] text-muted-foreground">Resumen diario SUNAT {z.hash}</p>
        <div className="flex flex-wrap gap-2"><Button onClick={() => window.print()}><Printer data-icon="inline-start" /> Imprimir Z</Button><Button variant="outline"><Download data-icon="inline-start" /> Exportar</Button><Button variant="outline" className="text-warning" onClick={() => toast.info("Requiere autorización de supervisor (OTP)")}><LockOpen data-icon="inline-start" /> Reabrir turno</Button></div>
      </SectionCard>
    </div>
  );
}
