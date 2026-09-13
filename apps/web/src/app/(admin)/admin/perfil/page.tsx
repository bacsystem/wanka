import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { ProfileScreen } from "@/features/settings/components/profile-screen";

export const metadata: Metadata = { title: "Mi perfil" };

export default function AdminProfilePage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
      <PageHeader eyebrow="Menú de usuario" title="Mi perfil y seguridad" description="Carlos M. Benavides · SRE / SuperAdmin N3 · superadmin@perusaas.pe" status={<StatusBadge tone="success" dot label="2FA activo (FIDO2)" />} />
      <ProfileScreen />
    </div>
  );
}
