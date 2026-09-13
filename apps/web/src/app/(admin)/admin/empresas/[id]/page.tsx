import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { TenantDetail } from "@/features/platform-admin/components/tenant-detail";
import { platformTenants } from "@/features/platform-admin/mocks/tenants";

export const metadata: Metadata = { title: "Detalle de empresa" };

export default async function AdminTenantPage({ params }: PageProps<"/admin/empresas/[id]">) {
  const { id } = await params;
  const tenant = platformTenants.find((t) => t.id === decodeURIComponent(id));
  if (!tenant) notFound();
  return <TenantDetail tenant={tenant} />;
}
