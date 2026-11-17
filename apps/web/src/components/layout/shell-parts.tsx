"use client";

import { type IconoComponente, PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import { ChevronsUpDown, Layers, LogOut, Settings } from "lucide-react";
import { m } from "motion/react";
import Link from "next/link";
import { RESORTE } from "@/lib/movimiento";
import { usePathname } from "next/navigation";
import * as React from "react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { SidebarGroup, SidebarGroupContent, SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuAction, SidebarMenuBadge, SidebarMenuButton, SidebarMenuItem, SidebarMenuSub, SidebarMenuSubButton, SidebarMenuSubItem } from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import { TARJETA } from "@/lib/estilos";

/**
 * Building blocks shared by the tenant app shell and the admin console shell,
 * so both interfaces use the same sidebar/topbar anatomy and differ only by theme + content.
 */

export function SidebarBrand({ name, tagline, badge, href = "/", icon: Icon = Layers as IconoComponente }: { name: string; tagline: string; badge?: string; href?: string; icon?: IconoComponente }) {
  return (
    <SidebarHeader className="h-14 justify-center px-3 group-data-[collapsible=icon]:px-2">
      <Link href={href} className="flex min-w-0 items-center gap-2.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={`${name} · Inicio`}>
        <span className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-acento-superficie text-acento-superficie-texto"><Icon className="size-4" /></span>
        <span className="flex min-w-0 flex-col leading-none group-data-[collapsible=icon]:hidden">
          <span className="flex items-center gap-1.5 truncate text-sm font-bold tracking-tight">{name}{badge ? <span className="rounded bg-accent px-1 py-0.5 text-[9px] font-semibold tracking-wide text-accent-foreground">{badge}</span> : null}</span>
          <span className="mt-0.5 truncate text-[10px] font-medium text-muted-foreground">{tagline}</span>
        </span>
      </Link>
    </SidebarHeader>
  );
}

/** Context card under the brand (tenant switcher in the app, cluster/entorno in the console). */
export function SidebarContextCard({ icon: Icon, title, chip, trailing = <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" />, onClick, ariaLabel }: { icon: IconoComponente; title: string; chip: ReactNode; trailing?: ReactNode; onClick?: () => void; ariaLabel?: string }) {
  const Comp = onClick ? "button" : "div";
  return (
    <div className="border-b border-sidebar-border bg-muted/40 p-3 group-data-[collapsible=icon]:hidden">
      <Comp type={onClick ? "button" : undefined} onClick={onClick} aria-label={ariaLabel} className={cn(TARJETA, "group flex w-full items-center justify-between gap-2 p-2 text-left text-card-foreground transition-colors hover:border-ring/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none")}>
        <span className="flex min-w-0 items-center gap-2">
          <span className="flex size-7 shrink-0 items-center justify-center rounded border border-primary/15 bg-accent text-primary"><Icon className="size-3.5" /></span>
          <span className="flex min-w-0 flex-col leading-tight"><span className="truncate text-xs font-semibold group-hover:text-primary">{title}</span><span className="mt-0.5 flex items-center gap-1 text-[9px] font-medium text-muted-foreground">{chip}</span></span>
        </span>
        {trailing}
      </Comp>
    </div>
  );
}

export interface ShellNavItem { label: string; href: string; icon: IconoComponente; badge?: string; children?: ShellNavItem[] }
export interface ShellNavSection { id: string; label: string; items: ShellNavItem[] }

const activeButtonClass = "relative isolate overflow-visible rounded-full transition-colors duration-(--duration-fast) ease-salida data-active:bg-transparent data-active:font-semibold data-active:text-sidebar-primary-foreground data-active:[&>svg]:text-sidebar-primary-foreground";

/** Sectioned navigation with collapsible groups; identical markup in both shells. */
export function SidebarSections({ sections, isActive }: { sections: ShellNavSection[]; isActive: (pathname: string, href: string) => boolean }) {
  const pathname = usePathname();
  const indicatorId = React.useId();
  const [toggled, setToggled] = React.useState<Record<string, boolean>>({});
  return (
    <>
      {sections.filter((s) => s.items.length).map((section) => (
        <SidebarGroup key={section.id} className="py-1.5">
          <SidebarGroupLabel className="h-6 px-3 text-[10px] font-bold tracking-wider text-muted-foreground/80 uppercase">{section.label}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="gap-0.5">
              {section.items.map((item) => {
                const active = isActive(pathname, item.href);
                const expanded = item.children ? (toggled[item.href] ?? active) : false;
                return (
                  <SidebarMenuItem key={item.href}>
                    <SidebarMenuButton tooltip={item.label} isActive={active} render={<Link href={item.href} />} className={cn("h-control px-3 text-control text-sidebar-foreground/90 [&>svg]:text-muted-foreground", activeButtonClass)}>
                      {active ? <m.span layoutId={`${indicatorId}-active`} aria-hidden transition={RESORTE} className="border-acento-borde bg-sidebar-primary absolute inset-0 -z-10 rounded-full border" /> : null}
                      <item.icon weight={active ? PESO_ICONO_ACTIVO : PESO_ICONO} className="size-[18px] shrink-0" />
                      <span className="font-medium">{item.label}</span>
                    </SidebarMenuButton>
                    {item.badge ? <SidebarMenuBadge className="rounded-full bg-muted px-1.5 font-mono text-[10px] text-muted-foreground">{item.badge}</SidebarMenuBadge> : null}
                    {item.children ? <SidebarMenuAction onClick={() => setToggled((t) => ({ ...t, [item.href]: !(t[item.href] ?? active) }))} aria-label={`${expanded ? "Contraer" : "Expandir"} ${item.label}`} aria-expanded={expanded} className="top-2.5 text-muted-foreground"><ChevronsUpDown className="size-3" /></SidebarMenuAction> : null}
                    {item.children && expanded ? (
                      <SidebarMenuSub className="mr-0 ml-6 gap-0.5 border-l-0 px-0 py-1">
                        {item.children.map((child) => { const childActive = child.href === item.href ? pathname === child.href : isActive(pathname, child.href); return (
                          <SidebarMenuSubItem key={child.href}><SidebarMenuSubButton isActive={childActive} render={<Link href={child.href} />} className={cn("h-7 gap-2 px-2 text-xs text-muted-foreground hover:text-primary [&>svg]:size-3.5", childActive && "font-medium text-primary")}><child.icon weight={childActive ? PESO_ICONO_ACTIVO : PESO_ICONO} className="size-3.5 shrink-0" /><span>{child.label}</span></SidebarMenuSubButton></SidebarMenuSubItem>
                        ); })}
                      </SidebarMenuSub>
                    ) : null}
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      ))}
    </>
  );
}

/** Footer user card: avatar, name, role, settings shortcut / logout. */
export function SidebarUserFooter({ initials, name, role, settingsHref, logoutHref, menu }: { initials: string; name: string; role: string; settingsHref?: string; logoutHref?: string; menu?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2 group-data-[collapsible=icon]:justify-center">
      {menu ?? (
        <div className="flex min-w-0 items-center gap-2">
          <InitialsAvatar initials={initials} size="sm" tone="muted" />
          <span className="flex min-w-0 flex-col leading-tight group-data-[collapsible=icon]:hidden"><span className="truncate text-xs font-semibold">{name}</span><span className="truncate text-[10px] text-muted-foreground">{role}</span></span>
        </div>
      )}
      {settingsHref ? <Button variant="ghost" size="icon-xs" aria-label="Configuración" className="text-muted-foreground group-data-[collapsible=icon]:hidden" render={<Link href={settingsHref} />} nativeButton={false}><Settings /></Button> : null}
      {logoutHref ? <Button variant="ghost" size="icon-xs" aria-label="Cerrar sesión" className="text-muted-foreground group-data-[collapsible=icon]:hidden" render={<Link href={logoutHref} />} nativeButton={false}><LogOut /></Button> : null}
    </div>
  );
}

/** Status pill used in both topbars (SUNAT in the app, entorno/SLA in the console). */
export function StatusPill({ tone, icon, label, detail, className }: { tone: "ok" | "warn" | "down"; icon?: ReactNode; label: string; detail?: string; className?: string }) {
  const cls = { ok: "bg-success text-success-foreground", warn: "bg-warning text-warning-foreground", down: "bg-destructive-suave text-destructive-texto" }[tone];
  const dot = { ok: "bg-success-solid", warn: "bg-warning-solid", down: "bg-destructive" }[tone];
  return (
    <span className={cn("hidden shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-etiqueta font-semibold whitespace-nowrap xl:inline-flex", cls, className)} aria-label={`${label}${detail ? ` · ${detail}` : ""}`}>
      {icon}<span className={`size-1.5 rounded-full ${dot}`} />{label}
      {detail ? <span className="hidden border-l border-current/20 pl-1.5 text-[10px] font-semibold opacity-80 2xl:inline">{detail}</span> : null}
    </span>
  );
}
