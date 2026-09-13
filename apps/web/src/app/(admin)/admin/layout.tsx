import type { Metadata } from "next";
import { cookies } from "next/headers";
import { AdminShell } from "@/features/platform-admin/components/admin-shell";

export const metadata: Metadata = { title: { default: "Consola de administración", template: "%s · Wanka Admin" } };

export default async function AdminLayout({ children }: LayoutProps<"/admin">) {
  const state = (await cookies()).get("sidebar_state")?.value;
  return <AdminShell defaultOpen={state !== "false"} hasStoredState={state !== undefined}>{children}</AdminShell>;
}
