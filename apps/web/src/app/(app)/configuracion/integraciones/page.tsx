import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { IntegrationsScreen } from "@/features/payables/components/integrations-screen";

export const metadata: Metadata = { title: "Integraciones" };

export default function IntegrationsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Configuración" title="Integraciones" description="Facturación electrónica, SUNAT, identidad, pasarelas de pago, mensajería y contabilidad." status={<StatusBadge tone="warning" dot label="1 integración requiere atención" />} />
      <IntegrationsScreen />
    </div>
  );
}
