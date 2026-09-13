import type { Metadata } from "next";
import { cookies } from "next/headers";
import { CompanySelectScreen } from "@/features/auth/components/company-select-screen";
import { TENANT_COOKIE } from "@/features/auth/lib/tenant-cookie";
import { tenantOptions } from "@/features/auth/mocks/tenants";

export const metadata: Metadata = { title: "Seleccionar empresa" };

export default async function CompanySelectPage() {
  const current = (await cookies()).get(TENANT_COOKIE)?.value;
  return <CompanySelectScreen tenants={tenantOptions} currentId={current} />;
}
