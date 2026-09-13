"use client";

import { CircleUser, LogOut, Settings, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export const currentUser = { name: "Dr. Ricardo Ruiz", role: "Administrador sede", initials: "RR" };

/** Sidebar footer card (Stitch): avatar, name, role and a settings shortcut. */
export function SidebarUserCard() {
  return (
    <div className="flex items-center justify-between gap-2 group-data-[collapsible=icon]:justify-center">
      <DropdownMenu>
        <DropdownMenuTrigger className="flex min-w-0 items-center gap-2 rounded-md text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={`Cuenta: ${currentUser.name}`}>
          <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"><CircleUser className="size-4" /></span>
          <span className="flex min-w-0 flex-col leading-tight group-data-[collapsible=icon]:hidden">
            <span className="truncate text-xs font-semibold">{currentUser.name}</span>
            <span className="truncate text-[10px] text-muted-foreground">{currentUser.role}</span>
          </span>
        </DropdownMenuTrigger>
        <UserMenuContent align="start" />
      </DropdownMenu>
      <Button variant="ghost" size="icon-xs" aria-label="Configuración" className="text-muted-foreground group-data-[collapsible=icon]:hidden" render={<Link href="/configuracion/usuarios" />} nativeButton={false}><Settings /></Button>
    </div>
  );
}

export function UserMenu() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label="Menú de usuario" />}>
        <Avatar className="size-8 border">
          <AvatarFallback className="bg-muted text-xs font-semibold text-muted-foreground">{currentUser.initials}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <UserMenuContent align="end" />
    </DropdownMenu>
  );
}

function UserMenuContent({ align }: { align: "start" | "end" }) {
  return (
      <DropdownMenuContent align={align} className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="flex flex-col">
            <span className="text-sm font-medium">{currentUser.name}</span>
            <span className="text-xs font-normal text-muted-foreground">{currentUser.role}</span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem render={<Link href="/perfil" />}>
          <CircleUser /> Mi perfil
        </DropdownMenuItem>
        <DropdownMenuItem render={<Link href="/configuracion/usuarios" />}>
          <Settings /> Configuración
        </DropdownMenuItem>
        <DropdownMenuItem render={<Link href="/admin" />}>
          <ShieldCheck /> Consola de administración
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" render={<Link href="/login" />}>
          <LogOut /> Cerrar sesión
        </DropdownMenuItem>
      </DropdownMenuContent>
  );
}
