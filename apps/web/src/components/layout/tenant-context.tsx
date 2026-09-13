"use client";

import * as React from "react";
import type { Tenant } from "@/types/domain";

const TenantContext = React.createContext<Tenant | null>(null);

export function TenantProvider({ tenant, children }: { tenant: Tenant; children: React.ReactNode }) {
  return <TenantContext.Provider value={tenant}>{children}</TenantContext.Provider>;
}

export function useTenant() {
  const t = React.useContext(TenantContext);
  if (!t) throw new Error("useTenant must be used within TenantProvider");
  return t;
}

/** Same as useTenant but returns null outside a TenantProvider (admin console). */
export function useOptionalTenant() {
  return React.useContext(TenantContext);
}
