import type { Metadata } from "next";
import { CalendarCheck, ShieldCheck, Users } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { CustomersScreen } from "@/features/customers/components/customers-table";
import { customersMock } from "@/features/customers/mocks/customers";

export const metadata: Metadata = { title: "Clientes" };

export default function CustomersPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <CustomersScreen customers={customersMock} kpis={<StatGrid columns={3}>
        <StatCard label="Total clientes registrados" value="1,248" icon={Users} hint={<StatusBadge tone="success" label="↗ +82 este mes" />} footer="892 pacientes | 356 empresas" />
        <StatCard label="Validación RENIEC / SUNAT" value="98.4%" icon={ShieldCheck} tone="success" hint={<span className="font-semibold text-primary">Alta fiabilidad</span>} footer={<span className="h-1.5 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full w-[98%] rounded-full bg-primary" /></span>} />
        <StatCard label="Pacientes activos (30 d)" value="412" icon={CalendarCheck} tone="info" hint="Citas y venta POS" footer="33.01% frecuencia mensual" />
      </StatGrid>} />
    </div>
  );
}
