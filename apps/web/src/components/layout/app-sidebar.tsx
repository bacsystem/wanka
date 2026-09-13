import { Sidebar, SidebarContent, SidebarFooter, SidebarRail } from "@/components/ui/sidebar";
import type { Tenant } from "@/types/domain";
import { SidebarBrand } from "./shell-parts";
import { SidebarNav } from "./sidebar-nav";
import { SidebarTenantCard } from "./tenant-switcher";
import { SidebarUserCard } from "./user-menu";

export function AppSidebar({ tenant }: { tenant: Tenant }) {
  return (
    <Sidebar collapsible="icon">
      <SidebarBrand name="Wanka" tagline="Gestión Empresarial" />
      <div className="border-b border-sidebar-border bg-muted/40 p-3 group-data-[collapsible=icon]:hidden">
        <SidebarTenantCard tenant={tenant} />
      </div>
      <SidebarContent className="py-2">
        <SidebarNav />
      </SidebarContent>
      <SidebarFooter className="border-t border-sidebar-border p-3 group-data-[collapsible=icon]:p-2">
        <SidebarUserCard />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
