import type { Metadata } from "next";
import { Building2, FileText, Lock, Sparkles } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { TenantsTable } from "@/features/platform-admin/components/tenants-table";
import { platformSummary as s, platformTenants } from "@/features/platform-admin/mocks/tenants";

export const metadata: Metadata = { title: "Empresas (tenants)" };

export default function AdminTenantsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Plataforma · Directorio de empresas (tenants)" title="Directorio de empresas (tenants)" description="Supervisa consumo mensual de CPE, conectores OSE certificados, límites de emisión y control de acceso administrativo." status={<StatusBadge tone="info" dot label={`${s.active.toLocaleString("es-PE")} clientes activos`} />} />
      <StatGrid>
        <StatCard label="Total empresas" value={s.active.toLocaleString("es-PE")} icon={Building2} hint={<span className="font-semibold text-emerald-600">+{s.activeDelta} netas este mes (set 2026)</span>} />
        <StatCard label="En periodo de prueba (trial)" value={String(s.trial)} icon={Sparkles} tone="info" footer={<span className="text-amber-700">{s.trialExpiring} vencen en ≤ 48 horas</span>} />
        <StatCard label="Emisión CPE total (mes)" value={s.cpeToday.toLocaleString("es-PE")} icon={FileText} tone="success" footer={<span className="text-emerald-600">{s.cdrRate}% CDR aprobados SUNAT</span>} />
        <StatCard label="Suspendidas / bloqueadas" value={String(s.suspended)} icon={Lock} tone="danger" emphasizeValue footer="5 por mora, 4 cert. caduco" />
      </StatGrid>
      <TenantsTable tenants={platformTenants} />
    </div>
  );
}
