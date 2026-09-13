"use client";

import { Bell, Building, ExternalLink, Plus, Search, Server } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { Sidebar, SidebarContent, SidebarFooter, SidebarInset, SidebarProvider, SidebarRail } from "@/components/ui/sidebar";
import { BottomNav } from "@/components/layout/bottom-nav";
import { SidebarBrand, SidebarContextCard, SidebarSections, SidebarUserFooter, StatusPill } from "@/components/layout/shell-parts";
import { SidebarAutoCollapse } from "@/components/layout/sidebar-auto-collapse";
import { Topbar } from "@/components/layout/topbar";
import { ConnectionIcon } from "@/components/shared/dynamic-icon";
import { adminNavigation, adminSectionLabel, isAdminActive } from "@/config/admin-navigation";
import { AdminSidebarUser, AdminUserMenu } from "./admin-user-menu";

/**
 * Shell of the SaaS admin console. Same anatomy as the tenant AppShell (brand → context card →
 * sectioned nav → user footer; topbar with context, search, bell, theme, user, primary action);
 * only the theme (`.theme-admin`, indigo) and the content differ.
 */
export function AdminShell({ children, defaultOpen = true, hasStoredState = false }: { children: ReactNode; defaultOpen?: boolean; hasStoredState?: boolean }) {
  const sections = (["supervision", "facturacion", "plataforma"] as const).map((id) => ({ id, label: adminSectionLabel[id], items: adminNavigation.filter((i) => i.section === id) }));
  return (
    <div className="theme-admin flex min-h-svh w-full bg-background text-foreground">
      <SidebarProvider defaultOpen={defaultOpen}>
        <SidebarAutoCollapse hasStoredState={hasStoredState} />
        <Sidebar collapsible="icon">
          <SidebarBrand name="Wanka" tagline="Consola de plataforma" badge="ADMIN" href="/admin" />
          <SidebarContextCard icon={Server} title="Cluster prod-lima-01" chip={<><span className="size-1.5 rounded-full bg-emerald-500" /> Producción · SLA 99.98%</>} trailing={<Tag tone="success" label="PE-Lima" />} />
          <SidebarContent className="py-2">
            <SidebarSections sections={sections} isActive={isAdminActive} />
          </SidebarContent>
          <SidebarFooter className="border-t border-sidebar-border p-3 group-data-[collapsible=icon]:p-2">
            <SidebarUserFooter initials="CB" name="Carlos M. Benavides" role="SRE / SuperAdmin N3" settingsHref="/admin/configuracion" menu={<AdminSidebarUser />} />
          </SidebarFooter>
          <SidebarRail />
        </Sidebar>
        <SidebarInset className="min-w-0 pb-28 md:pb-0">
          <Topbar
            context={<><span className="flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-xs font-semibold"><Server className="size-3.5 shrink-0 text-muted-foreground" /><span>prod-lima-01</span><span className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground sm:inline">Producción</span></span><StatusPill tone="ok" icon={<ConnectionIcon online className="size-3.5" />} label="OSE / SUNAT en línea" detail="385 ms" /></>}
            search={<><button type="button" onClick={() => toast.info("Búsqueda global de la consola")} className="relative hidden h-10 w-64 items-center rounded-lg border border-input bg-muted/50 pl-9 text-left text-xs text-muted-foreground transition-colors hover:bg-card xl:flex" aria-label="Buscar tenant, RUC o ticket (Ctrl+K)"><Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" /><span className="truncate">Buscar por RUC, razón social o ticket…</span><kbd className="pointer-events-none absolute top-1/2 right-2 -translate-y-1/2 rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] font-semibold">⌘K</kbd></button><Button variant="ghost" size="icon" aria-label="Buscar" className="xl:hidden" onClick={() => toast.info("Búsqueda global de la consola")}><Search /></Button></>}
            notifications={<Button variant="ghost" size="icon" aria-label="Notificaciones (1 sin leer)" className="relative" onClick={() => toast.info("3 incidentes abiertos · 1 certificado vencido")}><Bell /><span className="absolute top-1.5 right-1.5 size-2 rounded-full bg-rose-500 ring-2 ring-background" /></Button>}
            userMenu={<AdminUserMenu />}
            action={<><Button variant="outline" className="hidden font-semibold 2xl:inline-flex" render={<Link href="/" />} nativeButton={false}><Building data-icon="inline-start" /> App de empresa <ExternalLink data-icon="inline-end" className="size-3" /></Button><Button variant="outline" size="icon" aria-label="Ir a la app de empresa" className="hidden md:inline-flex 2xl:hidden" render={<Link href="/" />} nativeButton={false}><Building /></Button><Button className="hidden text-xs font-semibold shadow-sm md:inline-flex" render={<Link href="/admin/empresas" />} nativeButton={false}><Plus data-icon="inline-start" /> Alta de empresa</Button></>}
          />
          <div className="flex flex-1 flex-col">{children}</div>
          <footer className="flex flex-wrap items-center justify-between gap-2 border-t bg-card px-6 py-2 text-[11px] text-muted-foreground"><span><strong className="font-semibold text-foreground">Wanka Admin v4.12.0-prod</strong> • America/Lima (UTC-5) • Moneda base: PEN (S/)</span><span className="flex gap-4"><span>Documentación API OSE</span><span>Estado de pasarelas</span><span>Bitácora de auditoría</span></span></footer>
        </SidebarInset>
        <BottomNav items={adminNavigation.map((i) => ({ label: i.label.replace(" (tenants)", "").replace("Dashboard central", "Inicio"), href: i.href, icon: i.icon }))} primaryHrefs={["/admin", "/admin/empresas", "/admin/monitoreo", "/admin/soporte"]} isActive={isAdminActive} fab={{ href: "/admin/empresas", label: "Alta de empresa" }} />
      </SidebarProvider>
    </div>
  );
}
