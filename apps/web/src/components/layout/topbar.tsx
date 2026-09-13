import { PlusCircle } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { ConnectionIcon } from "@/components/shared/dynamic-icon";
import type { Tenant } from "@/types/domain";
import { Notifications } from "./notifications";
import { StatusPill } from "./shell-parts";
import { TenantSwitcher } from "./tenant-switcher";
import { TopbarSearch } from "./topbar-search";
import { ThemeToggle } from "./theme-toggle";
import { UserMenu } from "./user-menu";

interface TopbarProps {
  /** Left context: tenant switcher + SUNAT pill in the app; cluster/entorno chips in the console. */
  context?: ReactNode;
  /** Primary action on the right ("+ Nueva Venta" by default). */
  action?: ReactNode;
  search?: ReactNode;
  notifications?: ReactNode;
  userMenu?: ReactNode;
  tenant?: Tenant;
}

/** Shared topbar anatomy: trigger · context · [search · bell · theme · user · action]. */
export function Topbar({ context, action, search, notifications, userMenu, tenant }: TopbarProps) {
  return (
    <header className="sticky top-0 z-20 flex h-16 min-w-0 shrink-0 items-center gap-2 overflow-hidden border-b bg-card px-3 md:gap-3 md:px-6">
      <SidebarTrigger className="hidden md:inline-flex" />
      <Separator orientation="vertical" className="hidden h-5! md:block" />
      {context ?? (tenant ? <><TenantSwitcher tenant={tenant} /><StatusPill tone="ok" icon={<ConnectionIcon online className="size-3.5" />} label="SUNAT Conectado" detail="CDR OK" /></> : null)}
      <div className="ml-auto flex items-center gap-1 md:gap-2">
        {search ?? <TopbarSearch />}
        {notifications ?? <Notifications />}
        <ThemeToggle />
        <div className="flex items-center sm:border-l sm:pl-2">{userMenu ?? <UserMenu />}</div>
        {action ?? (
          <Button render={<Link href="/ventas/nueva" />} nativeButton={false} className="hidden text-xs font-semibold shadow-sm md:inline-flex">
            <PlusCircle data-icon="inline-start" /> Nueva Venta
          </Button>
        )}
      </div>
    </header>
  );
}
