import type { ReactNode } from "react";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import type { Tenant } from "@/types/domain";
import { AppSidebar } from "./app-sidebar";
import { BottomNav } from "./bottom-nav";
import { SidebarAutoCollapse } from "./sidebar-auto-collapse";
import { TenantProvider } from "./tenant-context";
import { Topbar } from "./topbar";

interface AppShellProps {
  tenant: Tenant;
  defaultSidebarOpen: boolean;
  hasStoredSidebarState: boolean;
  children: ReactNode;
}

export function AppShell({ tenant, defaultSidebarOpen, hasStoredSidebarState, children }: AppShellProps) {
  return (
    <TenantProvider tenant={tenant}>
    <SidebarProvider defaultOpen={defaultSidebarOpen}>
      <SidebarAutoCollapse hasStoredState={hasStoredSidebarState} />
      <AppSidebar tenant={tenant} />
      <SidebarInset className="min-w-0 pb-28 md:pb-0">
        <Topbar tenant={tenant} />
        <div className="flex flex-1 flex-col">{children}</div>
      </SidebarInset>
      <BottomNav />
    </SidebarProvider>
    </TenantProvider>
  );
}
