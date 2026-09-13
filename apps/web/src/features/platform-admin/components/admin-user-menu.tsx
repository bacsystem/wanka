"use client";

import { CircleUser, KeyRound, LogOut, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

export const adminUser = { name: "Carlos M. Benavides", role: "SRE / SuperAdmin N3", email: "superadmin@perusaas.pe", initials: "CB" };

function Menu({ align }: { align: "start" | "end" }) {
  return (
    <DropdownMenuContent align={align} className="w-60">
      <DropdownMenuGroup>
        <DropdownMenuLabel className="flex flex-col"><span className="text-sm font-medium">{adminUser.name}</span><span className="text-xs font-normal text-muted-foreground">{adminUser.role} · {adminUser.email}</span></DropdownMenuLabel>
      </DropdownMenuGroup>
      <DropdownMenuSeparator />
      <DropdownMenuItem render={<Link href="/admin/perfil" />}><CircleUser /> Mi perfil</DropdownMenuItem>
      <DropdownMenuItem render={<Link href="/admin/perfil" />}><KeyRound /> Seguridad y 2FA</DropdownMenuItem>
      <DropdownMenuItem render={<Link href="/admin/auditoria" />}><ShieldCheck /> Mis sesiones y auditoría</DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem variant="destructive" render={<Link href="/admin/login" />}><LogOut /> Cerrar sesión</DropdownMenuItem>
    </DropdownMenuContent>
  );
}

/** Topbar avatar menu of the console. */
export function AdminUserMenu() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label="Menú de usuario" />}><span className="flex size-8 items-center justify-center rounded-full border bg-muted text-xs font-semibold text-muted-foreground">{adminUser.initials}</span></DropdownMenuTrigger>
      <Menu align="end" />
    </DropdownMenu>
  );
}

/** Sidebar footer card of the console (same anatomy as the tenant SidebarUserCard). */
export function AdminSidebarUser() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex min-w-0 items-center gap-2 rounded-md text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={`Cuenta: ${adminUser.name}`}>
        <InitialsAvatar initials={adminUser.initials} size="sm" tone="muted" />
        <span className="flex min-w-0 flex-col leading-tight group-data-[collapsible=icon]:hidden"><span className="truncate text-xs font-semibold">{adminUser.name}</span><span className="truncate text-[10px] text-muted-foreground">{adminUser.role}</span></span>
      </DropdownMenuTrigger>
      <Menu align="start" />
    </DropdownMenu>
  );
}
