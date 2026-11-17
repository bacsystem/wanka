import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { AuditScreen } from "@/features/settings/components/audit-screen";
import { auditMock } from "@/features/cash/mocks/shifts";

export const metadata: Metadata = { title: "Auditoría" };

export default function AuditPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Configuración y seguridad" title="Auditoría del sistema" description="Trazabilidad de acciones por usuario, módulo y sede, con valores antes/después y alertas de seguridad." />
      <AuditScreen events={auditMock} />
    </div>
  );
}
