"use client";

import { Building, Building2, Check, ChevronsUpDown, PlusCircle } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { TENANT_COOKIE, setClientCookie } from "@/features/auth/lib/tenant-cookie";
import { tenantOptions } from "@/features/auth/mocks/tenants";
import { industryLabel, initials } from "@/lib/tenant";
import type { Tenant } from "@/types/domain";

function useChooseTenant(tenant: Tenant) {
  const router = useRouter();
  return function choose(id: string) {
    const t = tenantOptions.find((x) => x.id === id);
    if (!t || t.id === tenant.id) return;
    setClientCookie(TENANT_COOKIE, t.id);
    toast.success(`Trabajando en ${t.name}`);
    router.refresh();
  };
}

/** Short name without the legal suffix (S.A.C., E.I.R.L., …) for tight places. */
export function shortTenantName(name: string) {
  return name.replace(/\s+(S\.A\.C\.|S\.A\.|E\.I\.R\.L\.|S\.R\.L\.)$/i, "");
}

/** Sidebar card version (Stitch: building icon, name, industry chip). */
export function SidebarTenantCard({ tenant }: { tenant: Tenant }) {
  const choose = useChooseTenant(tenant);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="group flex w-full items-center justify-between gap-2 rounded-lg border bg-card p-2 text-left shadow-xs transition-colors hover:border-ring/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={`Empresa seleccionada: ${tenant.name}`}>
        <span className="flex min-w-0 items-center gap-2">
          <span className="flex size-7 shrink-0 items-center justify-center rounded border border-primary/15 bg-accent text-primary"><Building className="size-3.5" /></span>
          <span className="flex min-w-0 flex-col leading-tight">
            <span className="truncate text-xs font-semibold group-hover:text-primary">{shortTenantName(tenant.name)}</span>
            <span className="mt-0.5 w-fit rounded border bg-muted px-1.5 text-[9px] font-medium text-muted-foreground">{industryLabel[tenant.industry]}</span>
          </span>
        </span>
        <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" />
      </DropdownMenuTrigger>
      <TenantMenu tenant={tenant} choose={choose} align="start" />
    </DropdownMenu>
  );
}

export function TenantSwitcher({ tenant }: { tenant: Tenant }) {
  const choose = useChooseTenant(tenant);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" className="min-w-0 shrink gap-1.5 px-2 text-xs font-semibold hover:text-primary" aria-label={`Empresa actual: ${tenant.name}. Cambiar empresa`} />}>
        <Building className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate">{shortTenantName(tenant.name)}</span>
        <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" />
        <Badge variant="outline" className="hidden shrink-0 bg-muted text-[10px] font-medium text-muted-foreground sm:inline-flex">{industryLabel[tenant.industry]}</Badge>
      </DropdownMenuTrigger>
      <TenantMenu tenant={tenant} choose={choose} align="start" />
    </DropdownMenu>
  );
}

function TenantMenu({ tenant, choose, align }: { tenant: Tenant; choose: (id: string) => void; align: "start" | "end" }) {
  return (
      <DropdownMenuContent align={align} className="w-80">
        <DropdownMenuGroup>
        <DropdownMenuLabel className="text-xs text-muted-foreground">Empresas vinculadas a tu usuario</DropdownMenuLabel>
        {tenantOptions.map((t) => (
          <DropdownMenuItem key={t.id} onClick={() => choose(t.id)} className="gap-3">
            <InitialsAvatar initials={initials(t.name)} size="sm" shape="square" />
            <span className="flex min-w-0 flex-1 flex-col"><span className="truncate text-sm">{t.name}</span><span className="text-xs text-muted-foreground">RUC {t.ruc} · {t.industryLabel} · {t.role}</span></span>
            {t.id === tenant.id ? <Check className="size-4 text-primary" /> : null}
          </DropdownMenuItem>
        ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem render={<Link href="/seleccionar-empresa" />}><Building2 /> Cambiar sede o caja</DropdownMenuItem>
        <DropdownMenuItem render={<Link href="/onboarding" />}><PlusCircle /> Registrar nueva empresa (RUC)</DropdownMenuItem>
      </DropdownMenuContent>
  );
}
