import type { Metadata } from "next";
import { Building2, MonitorSmartphone, ShieldAlert, Users } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { SecurityScreen } from "@/features/settings/components/security-screen";
import { collaboratorsMock, permissionMatrix, roles, securitySummary as s } from "@/features/settings/mocks/settings";

export const metadata: Metadata = { title: "Usuarios y roles" };

export default function UsersPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <SecurityScreen collaborators={collaboratorsMock} roles={roles} matrix={permissionMatrix} kpis={<StatGrid>
        <StatCard label="Total colaboradores" value={String(s.total)} suffix="Activos" icon={Users} footer={<span className="flex w-full flex-col gap-1.5"><span className="flex items-center gap-1 text-destructive">⚠ {s.pending2fa} con 2FA pendiente</span><span className="h-1 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full w-[89%] rounded-full bg-primary" /></span></span>} />
        <StatCard label="Sesiones concurrentes" value={String(s.sessions)} suffix="Terminales POS/Web" icon={MonitorSmartphone} tone="info" footer={<span className="flex w-full flex-col gap-1.5"><span className="flex justify-between"><span>Máx autorizado: {s.maxSessions} puestos</span><strong className="text-primary">{Math.round((s.sessions / s.maxSessions) * 100)}% cupo</strong></span><span className="h-1 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full rounded-full bg-blue-600" style={{ width: `${Math.round((s.sessions / s.maxSessions) * 100)}%` }} /></span></span>} />
        <StatCard label="Sedes registradas" value={String(s.sites)} suffix="Multisede activa" icon={Building2} footer={<span className="flex w-full flex-col gap-1.5"><span>San Isidro • Miraflores • Surquillo</span><span className="h-1 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full w-full rounded-full bg-primary" /></span></span>} />
        <StatCard label="Incidentes 48h" value={String(s.incidents48h)} suffix="Intentos fallidos" icon={ShieldAlert} tone="success" footer={<span className="flex w-full flex-col gap-1.5"><span className="flex items-center gap-1"><span className="size-1.5 rounded-full bg-primary" /> Firewall de IPs clínicas óptimo</span><span className="h-1 w-full overflow-hidden rounded-full bg-muted"><span className="block h-full w-full rounded-full bg-primary" /></span></span>} />
      </StatGrid>} />
    </div>
  );
}
