import type { Metadata } from "next";
import { LockOpen, MinusCircle, ShieldCheck, TrendingUp, Wallet } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { CashScreen, ShiftBadge } from "@/features/cash/components/cash-screen";
import { movementsMock, shift } from "@/features/cash/mocks/cash";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Caja y arqueo" };

export default function CashPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Caja y finanzas · Arqueo y cierre de turno" title="Control de caja y turnos (arqueo diario)" status={<StatusBadge tone="info" dot label="Turno en curso (auditoría activa)" />} actions={<ShiftBadge />} />
      <StatGrid>
        <StatCard label="Monto de apertura" value={formatCurrency(shift.openingAmount)} icon={LockOpen} tone="info" footer={<span className="flex items-center gap-1"><LockOpen className="size-3" /> Fondo sencillo • {shift.openedAt}</span>} />
        <StatCard label="Ventas totales turno" value={formatCurrency(shift.sales)} icon={TrendingUp} emphasizeValue footer={<span className="text-primary">{shift.salesCount} transacciones SUNAT válidas</span>} />
        <StatCard label="Egresos / gastos menores" value={`-${formatCurrency(shift.expenses)}`} icon={MinusCircle} tone="danger" emphasizeValue footer={`${shift.expensesCount} vales de caja autorizados`} />
        <StatCard label="Saldo teórico esperado" value={formatCurrency(shift.expected)} icon={Wallet} highlight footer={<span className="flex items-center gap-1"><ShieldCheck className="size-3" /> Total acumulado todos los medios</span>} />
      </StatGrid>
      <CashScreen movements={movementsMock} />
    </div>
  );
}
