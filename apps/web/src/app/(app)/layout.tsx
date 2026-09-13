import { cookies } from "next/headers";
import { AppShell } from "@/components/layout/app-shell";
import { TENANT_COOKIE } from "@/features/auth/lib/tenant-cookie";
import { tenantOptions } from "@/features/auth/mocks/tenants";

export default async function AppLayout({ children }: LayoutProps<"/">) {
  const cookieStore = await cookies();
  const stored = cookieStore.get("sidebar_state")?.value;
  const sidebarOpen = stored !== "false";
  const tenantId = cookieStore.get(TENANT_COOKIE)?.value;
  const tenant = tenantOptions.find((t) => t.id === tenantId) ?? tenantOptions[0];

  return (
    <AppShell tenant={tenant} defaultSidebarOpen={sidebarOpen} hasStoredSidebarState={stored !== undefined}>
      {children}
    </AppShell>
  );
}
