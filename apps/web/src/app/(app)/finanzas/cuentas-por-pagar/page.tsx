import type { Metadata } from "next";
import { AlertTriangle, CalendarClock, CheckCircle2, Wallet } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { PayablesScreen } from "@/features/payables/components/payables-screen";
import { payablesMock, payablesSummary as s } from "@/features/payables/mocks/payables";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Cuentas por pagar" };

export default function PayablesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Finanzas" title="Cuentas por pagar y programación de pagos" description="Facturas de proveedores, vencimientos, detracciones SPOT y archivos de pago masivo." />
      <StatGrid>
        <StatCard label="Por pagar" value={formatCurrency(s.total)} icon={Wallet} hint="6 facturas vigentes" />
        <StatCard label="Vence esta semana" value={formatCurrency(s.week)} icon={CalendarClock} tone="warning" hint="3 facturas · programar antes del viernes" />
        <StatCard label="Vencidas" value={formatCurrency(s.overdue)} icon={AlertTriangle} tone="danger" hint="1 factura · Dental Import" />
        <StatCard label="Pagado en el mes" value={formatCurrency(s.paidMonth)} icon={CheckCircle2} tone="success" hint="14 pagos · 100% con constancia" />
      </StatGrid>
      <PayablesScreen payables={payablesMock} />
    </div>
  );
}
