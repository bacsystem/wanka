"use client";

import { Banknote, CalendarClock, Download, Mail } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Payable, PayableStatus } from "../mocks/payables";

const meta: Record<PayableStatus, { label: string; tone: BadgeTone }> = { programada: { label: "Programada", tone: "info" }, por_vencer: { label: "Por vencer", tone: "warning" }, vencida: { label: "Vencida", tone: "danger" }, pagada: { label: "Pagada", tone: "success" } };
const week = ["L 14", "M 15", "X 16", "J 17", "V 18", "S 19", "D 20"];

export function PayablesScreen({ payables: initial }: { payables: Payable[] }) {
  const [rows, setRows] = React.useState(initial);
  const [sel, setSel] = React.useState<Set<string>>(new Set());
  const [open, setOpen] = React.useState(false);
  const selected = rows.filter((r) => sel.has(r.id) && r.status !== "pagada");
  const amount = selected.reduce((s, r) => s + r.balance, 0);
  const spot = selected.reduce((s, r) => s + (r.detractionPending ?? 0), 0);
  const cols: Column<Payable>[] = [
    { key: "sel", header: <span className="sr-only">Seleccionar</span>, cell: (r) => <Checkbox aria-label={`Seleccionar ${r.number}`} disabled={r.status === "pagada"} checked={sel.has(r.id)} onCheckedChange={(v) => setSel((s) => { const n = new Set(s); if (v) n.add(r.id); else n.delete(r.id); return n; })} />, className: "w-8" },
    { key: "sup", header: "Proveedor", cell: (r) => <TwoLine primary={<span className="font-medium">{r.supplier}</span>} secondary={`RUC ${r.ruc}`} mono /> },
    { key: "num", header: "Comprobante", cell: (r) => <span className="font-mono text-xs">{r.number}</span> },
    { key: "d", header: "Emisión / vencimiento", cell: (r) => <TwoLine primary={formatDate(r.issuedAt)} secondary={`Vence ${formatDate(r.dueAt)}`} mono /> },
    { key: "t", header: "Total", align: "right", cell: (r) => <span className="font-mono">{formatCurrency(r.total)}</span> },
    { key: "spot", header: "Detracción pend.", align: "right", cell: (r) => r.detractionPending ? <span className="font-mono text-warning">{formatCurrency(r.detractionPending)}</span> : <span className="text-muted-foreground">—</span> },
    { key: "b", header: "Saldo", align: "right", cell: (r) => <span className="font-mono font-semibold">{formatCurrency(r.balance)}</span> },
    { key: "s", header: "Estado", cell: (r) => <StatusBadge tone={meta[r.status].tone} dot label={meta[r.status].label} /> },
  ];
  return (
    <>
      <SectionCard title="Calendario de vencimientos · semana del 14/09" contentClassName="p-3">
        <div className="grid grid-cols-7 gap-1.5">{week.map((d, i) => { const due = rows.filter((r) => r.status !== "pagada" && new Date(r.dueAt).getDate() === 14 + i); const total = due.reduce((s, r) => s + r.balance, 0); return <div key={d} className={cn("rounded-md border p-2 text-xs", due.length && "border-warning/50 bg-warning/10")}><p className="font-mono text-muted-foreground">{d}</p>{due.length ? <><p className="font-mono font-semibold">{formatCurrency(total)}</p><p className="truncate text-muted-foreground">{due.length} factura{due.length === 1 ? "" : "s"}</p></> : <p className="text-muted-foreground">—</p>}</div>; })}</div>
      </SectionCard>
      <SectionCard title="Facturas de proveedores" action={<div className="flex items-center gap-2 text-xs">{selected.length ? <span className="text-muted-foreground">{selected.length} seleccionadas · {formatCurrency(amount)}</span> : null}<Button size="sm" disabled={!selected.length} onClick={() => setOpen(true)}><CalendarClock data-icon="inline-start" /> Programar pago</Button></div>} contentClassName="p-0">
        <DataTable columns={cols} rows={rows} rowKey={(r) => r.id} minWidth="980px" mobileCard={(r) => <label className="flex items-center gap-3"><Checkbox aria-label={`Seleccionar ${r.number}`} disabled={r.status === "pagada"} checked={sel.has(r.id)} onCheckedChange={(v) => setSel((s) => { const n = new Set(s); if (v) n.add(r.id); else n.delete(r.id); return n; })} /><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{r.supplier}</p><p className="font-mono text-xs text-muted-foreground">{r.number} · vence {formatDate(r.dueAt)}</p></div><div className="flex flex-col items-end gap-1"><span className="font-mono text-sm font-semibold">{formatCurrency(r.balance)}</span><StatusBadge tone={meta[r.status].tone} dot label={meta[r.status].label} /></div></label>} />
      </SectionCard>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-lg">
          <SheetHeader><SheetTitle>Programar pago a proveedores</SheetTitle><SheetDescription>{selected.length} factura{selected.length === 1 ? "" : "s"} · {formatCurrency(amount)}</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4">
            <Field label="Cuenta bancaria de origen"><Select defaultValue="bcp" items={[{ value: "bcp", label: "BCP cta. cte. 193-2345678-0-11 · S/ 84,120.00" }, { value: "ibk", label: "Interbank cta. cte. 200-3001234567 · S/ 21,400.00" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="bcp">BCP cta. cte. 193-2345678-0-11 · S/ 84,120.00</SelectItem><SelectItem value="ibk">Interbank cta. cte. 200-3001234567 · S/ 21,400.00</SelectItem></SelectContent></Select></Field>
            <div className="grid grid-cols-2 gap-3"><Field label="Fecha de pago"><Input type="date" defaultValue="2026-09-15" /></Field><Field label="Medio"><Select defaultValue="transfer" items={[{ value: "transfer", label: "Transferencia" }, { value: "cheque", label: "Cheque" }, { value: "yape", label: "Yape empresa" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="transfer">Transferencia</SelectItem><SelectItem value="cheque">Cheque</SelectItem><SelectItem value="yape">Yape empresa</SelectItem></SelectContent></Select></Field></div>
            <dl className="rounded-md bg-muted/40 p-3 text-sm"><div className="flex justify-between"><dt className="text-muted-foreground">Monto a proveedores</dt><dd className="font-mono">{formatCurrency(amount - spot)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">Detracción SPOT a depositar en BN</dt><dd className="font-mono text-warning">{formatCurrency(spot)}</dd></div><div className="flex justify-between border-t pt-1 font-semibold"><dt>Total</dt><dd className="font-mono">{formatCurrency(amount)}</dd></div></dl>
            <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> Generar archivo de pago masivo (Telecrédito BCP)</label>
            <label className="flex items-center gap-2 text-sm"><Checkbox defaultChecked /> <Mail className="size-4" /> Enviar constancia por correo a cada proveedor</label>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => setOpen(false)}>Cancelar</Button><Button onClick={() => { setRows((rs) => rs.map((r) => (sel.has(r.id) ? { ...r, status: "programada" as const } : r))); setSel(new Set()); setOpen(false); toast.success(`Pago programado · ${formatCurrency(amount)} · archivo Telecrédito generado`); }}><Banknote data-icon="inline-start" /> Programar {formatCurrency(amount)}</Button><Button variant="ghost" size="icon" aria-label="Descargar archivo"><Download /></Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  );
}
