"use client";

import { Banknote, BarChart3, Download, FileText, MessageCircle, MessageSquare, PlusCircle, SlidersHorizontal } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Button } from "@/components/ui/button";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, Tag, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import type { Receivable, ReceivableStatus } from "../mocks/receivables";

const TODAY = new Date("2026-09-12");
const days = (iso: string) => Math.round((new Date(iso).getTime() - TODAY.getTime()) / 86_400_000);

const statusMeta: Record<ReceivableStatus, { label: string; tone: BadgeTone }> = {
  al_dia: { label: "Al día", tone: "success" },
  por_vencer: { label: "Por vencer", tone: "warning" },
  vencido: { label: "Vencido / moroso", tone: "danger" },
  judicial: { label: "Cobranza judicial", tone: "neutral" },
};

const chartConfig = { real: { label: "Cobrado real", color: "var(--chart-1)" }, projected: { label: "Proyectado", color: "var(--chart-3)" } } satisfies ChartConfig;

interface Props {
  rows: Receivable[];
  weekly: { week: string; real: number; projected: number }[];
  spot: { rate: number; deposited: number; pending: number };
}

export function ReceivablesScreen({ rows: all, weekly, spot, kpis }: Props & { kpis?: React.ReactNode }) {
  const [tab, setTab] = React.useState("all");
  const [query, setQuery] = React.useState("");
  const [paying, setPaying] = React.useState<Receivable | null>(null);

  const rows = all.filter((r) => {
    const s = query.trim().toLowerCase();
    return (tab === "all" || r.status === tab) && (!s || r.customer.toLowerCase().includes(s) || r.number.toLowerCase().includes(s) || r.customerRuc.includes(s));
  });
  const count = (k: string) => (k === "all" ? all.length : all.filter((r) => r.status === k).length);

  const columns: Column<Receivable>[] = [
    { key: "sel", header: <Checkbox aria-label="Seleccionar todos" />, cell: () => <Checkbox aria-label="Seleccionar" />, className: "w-8" },
    { key: "number", header: "Comprobante", cell: (r) => <TwoLine primary={<span className="font-mono text-xs font-medium text-primary tabular-nums">{r.number}</span>} secondary={`${r.type} electrónica`} /> },
    { key: "customer", header: "Cliente / razón social", cell: (r) => <TwoLine primary={<span className="font-medium">{r.customer}</span>} secondary={`RUC ${r.customerRuc}`} mono />, className: "max-w-64" },
    { key: "issued", header: "Emisión", cell: (r) => <span className="font-mono tabular-nums">{formatDate(r.issuedAt)}</span> },
    { key: "due", header: "Vencimiento", cell: (r) => { const d = days(r.dueAt); return <TwoLine primary={<span className="font-mono tabular-nums">{formatDate(r.dueAt)}</span>} secondary={d < 0 ? <span className="text-destructive">Vencido hace {-d} días</span> : d <= 3 ? <span className="text-warning">Vence en {d} día{d === 1 ? "" : "s"}</span> : `En ${d} días`} />; } },
    { key: "term", header: "Plazo", align: "right", cell: (r) => <span className="font-mono tabular-nums">{r.termDays} d</span> },
    { key: "total", header: "Total", align: "right", cell: (r) => <span className="font-mono tabular-nums">{formatCurrency(r.total)}</span> },
    { key: "balance", header: "Saldo pend.", align: "right", cell: (r) => <span className="font-mono font-semibold tabular-nums">{formatCurrency(r.balance)}</span> },
    { key: "inst", header: "Cuota", cell: (r) => <span className="font-mono text-xs tabular-nums">Cuota {r.installment}</span> },
    { key: "status", header: "Estado", cell: (r) => <StatusBadge tone={statusMeta[r.status].tone} dot label={statusMeta[r.status].label} /> },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (r) => (
        <div className="flex justify-end gap-0.5">
          <Button size="xs" onClick={() => setPaying(r)}><Banknote data-icon="inline-start" /> Abonar</Button>
          <Button variant="ghost" size="icon-sm" aria-label="Estado de cuenta"><FileText /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Recordatorio WhatsApp" onClick={() => toast.success(`Recordatorio enviado a ${r.customer}`)}><MessageCircle /></Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Finanzas · Cartera de clientes y cobranzas"
        title="Gestión de cuentas por cobrar y créditos"
        status={<StatusBadge tone="info" dot label="SUNAT sync activo" />}
        actions={
          <>
            <Button variant="outline" className="font-semibold" onClick={() => toast.success("Recordatorios enviados a 5 clientes con deuda vencida")}><MessageSquare data-icon="inline-start" /> Recordatorio WhatsApp / Email</Button>
            <Button variant="outline" className="font-semibold"><Download data-icon="inline-start" /> Exportar cartera</Button>
            <Button className="font-semibold" onClick={() => setPaying(all[0])}><PlusCircle data-icon="inline-start" /> Registrar abono / pago de cuota</Button>
          </>
        }
      />
      {kpis}
      <div className="grid gap-4 xl:grid-cols-3">
        <SectionCard title="Proyección semanal de cobranza vs facturación" icon={BarChart3} className="xl:col-span-2" contentClassName="p-4">
          <ChartContainer config={chartConfig} className="aspect-auto h-56 w-full">
            <BarChart data={weekly} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis dataKey="week" tickLine={false} axisLine={false} tickMargin={8} />
              <YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => `${Math.round(v / 1000)}k`} />
              <ChartTooltip content={<ChartTooltipContent formatter={(v, name) => <span className="flex w-full justify-between gap-4"><span className="text-muted-foreground">{chartConfig[name as keyof typeof chartConfig].label}</span><span className="font-mono font-medium">{formatCurrency(Number(v))}</span></span>} />} />
              <ChartLegend content={<ChartLegendContent />} />
              <Bar dataKey="real" fill="var(--color-real)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
              <Bar dataKey="projected" fill="var(--color-projected)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ChartContainer>
        </SectionCard>
        <SectionCard title="Detracción SUNAT pendiente" action={<Tag label="SPOT BN" />} contentClassName="flex flex-col gap-4 p-4">
          <div className="flex items-center gap-4">
            <span className="relative grid size-20 shrink-0 place-items-center rounded-full" style={{ background: `conic-gradient(var(--primary) ${spot.rate * 3.6}deg, var(--muted) 0)` }} aria-label={`${spot.rate}% depositado`}><span className="grid size-14 place-items-center rounded-full bg-card font-mono text-base font-bold">{spot.rate}%</span></span>
            <div className="text-sm"><p className="text-muted-foreground">Fondos depositados en Cta. Banco de la Nación</p><p className="font-mono text-lg font-bold text-primary tabular-nums">{formatCurrency(spot.deposited)}</p><p className="text-xs font-medium text-destructive">{formatCurrency(spot.pending)} pendientes de abono cliente</p></div>
          </div>
          <div className="flex items-center justify-between border-t pt-3 text-xs"><span className="text-muted-foreground">Tasa media aplicada: <strong className="font-semibold text-foreground">12% / 10%</strong></span><button type="button" className="font-semibold text-primary hover:underline" onClick={() => toast.info("Constancias SPOT")}>Ver constancias SPOT</button></div>
        </SectionCard>
      </div>

      <SectionCard title={<span className="sr-only">Cartera de clientes</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b p-3">
          <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="min-w-0 max-w-full">
            <TabsList variant="pills">
              {([["all", "Todas las deudas"], ["por_vencer", "Por vencer"], ["vencido", "Vencidas / morosas"], ["judicial", "Cobranza judicial"]] as const).map(([v, l]) => (
                <TabsTrigger key={v} value={v}>{l} ({count(v)})</TabsTrigger>
              ))}
              <TabsTrigger value="historial" className="h-9 rounded-full px-4 text-sm font-semibold">Historial de abonos</TabsTrigger>
            </TabsList>
          </Tabs>
          <div className="flex items-center gap-2"><SearchInput placeholder="Buscar cliente, RUC o N° factura…" value={query} onChange={setQuery} className="sm:w-64" /><Button variant="outline" size="sm" className="font-semibold"><SlidersHorizontal data-icon="inline-start" /> Filtros</Button></div>
        </div>
        <DataTable
          columns={columns} rows={rows} rowKey={(r) => r.id} minWidth="1240px"
          mobileCard={(r) => (
            <div className="flex items-start gap-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{r.customer}</p>
                <p className="font-mono text-xs text-muted-foreground tabular-nums">{r.number} · vence {formatDate(r.dueAt)}</p>
                <StatusBadge className="mt-1" tone={statusMeta[r.status].tone} dot label={statusMeta[r.status].label} />
              </div>
              <div className="flex flex-col items-end gap-1">
                <span className="font-mono text-sm font-semibold tabular-nums">{formatCurrency(r.balance)}</span>
                <Button size="xs" onClick={() => setPaying(r)}>Abonar</Button>
              </div>
            </div>
          )}
        />
        <PaginationBar from={1} to={rows.length} total={34} label="deudas" />
      </SectionCard>

      <PaymentDialog r={paying} onClose={() => setPaying(null)} />
    </>
  );
}

function PaymentDialog({ r, onClose }: { r: Receivable | null; onClose: () => void }) {
  return (
    <Dialog open={Boolean(r)} onOpenChange={(o) => !o && onClose()}>
      {r ? <PaymentForm key={r.id} r={r} onClose={onClose} /> : null}
    </Dialog>
  );
}

function PaymentForm({ r, onClose }: { r: Receivable; onClose: () => void }) {
  const [amount, setAmount] = React.useState(String(r.balance));
  return (
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Registrar abono / pago de cuota</DialogTitle>
          <DialogDescription>{r.number} · {r.customer} · saldo {formatCurrency(r.balance)}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Método de cobro">
            <Select defaultValue="transferencia" items={[{ value: "transferencia", label: "Transferencia bancaria" }, { value: "efectivo", label: "Efectivo" }, { value: "yape", label: "Yape / Plin" }, { value: "cheque", label: "Cheque" }]}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="transferencia">Transferencia bancaria</SelectItem><SelectItem value="efectivo">Efectivo</SelectItem><SelectItem value="yape">Yape / Plin</SelectItem><SelectItem value="cheque">Cheque</SelectItem></SelectContent>
            </Select>
          </Field>
          <Field label="N° de operación / referencia"><Input className="font-mono" placeholder="948201" /></Field>
          <Field label="Monto a amortizar (S/)"><Input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
          <Field label="Fecha de pago"><Input type="date" defaultValue="2026-09-12" /></Field>
          {r.detraction ? (
            <div className="rounded-md border bg-muted/40 p-3 text-sm sm:col-span-2">
              <p className="font-medium">Detracción {r.detraction.rate}% · {formatCurrency(r.detraction.amount)} (Cta. BN)</p>
              <p className="text-xs text-muted-foreground">{r.detraction.deposited ? "Constancia de depósito registrada." : "Aún sin constancia: el cliente debe depositar en el Banco de la Nación."}</p>
            </div>
          ) : null}
          <Field label="Nota interna / glosa de tesorería" className="sm:col-span-2"><Textarea rows={2} /></Field>
          <label className="flex items-center gap-2 text-sm sm:col-span-2"><Checkbox defaultChecked /> Emitir y enviar recibo electrónico de caja (PDF + WhatsApp)</label>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancelar</Button>
          <Button onClick={() => { toast.success(`Abono de ${formatCurrency(Number(amount) || 0)} registrado`, { description: r.number }); onClose(); }}><Banknote data-icon="inline-start" /> Confirmar abono</Button>
        </DialogFooter>
      </DialogContent>
  );
}
