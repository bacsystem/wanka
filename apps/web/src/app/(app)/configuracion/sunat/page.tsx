import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { SunatSettings } from "@/features/settings/components/sunat-settings";
import { sunatConfig } from "@/features/settings/mocks/settings";

export const metadata: Metadata = { title: "Configuración SUNAT" };

export default function SunatSettingsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Administración" title="Facturación electrónica y certificado digital" description="Llaves criptográficas, canal de timbrado, numeración fiscal autorizada y plantilla de impresión." status={<StatusBadge tone="success" dot label="Servidor SUNAT operativo" />} />
      <SunatSettings config={sunatConfig} />
    </div>
  );
}
