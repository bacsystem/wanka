import { cookies } from "next/headers";
import { AppShell } from "@/components/shell/app-shell";
import { TENANT_COOKIE } from "@/features/auth/lib/tenant-cookie";
import { buildShellSession } from "@/features/auth/lib/shell-session";

export default async function AppLayout({ children }: LayoutProps<"/">) {
  const tenantId = (await cookies()).get(TENANT_COOKIE)?.value;
  return <AppShell session={buildShellSession(tenantId)}>{children}</AppShell>;
}
