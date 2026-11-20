"use client";

import { useRouter } from "next/navigation";
import { SelectorContexto, type ContextoItem } from "khipu-design-system/nav/selector-contexto";
import { MenuUsuario, type UsuarioMenu } from "khipu-design-system/nav/menu-usuario";
import { LogoMarca } from "khipu-design-system/nav/logo";
import { SidebarContent } from "khipu-design-system/nav/sidebar";
import { sidebarGroups } from "@/config/navigation-menus";
import { enabledFeatures, type Feature } from "@/config/navigation";
import { setClientCookie, TENANT_COOKIE } from "@/features/auth/lib/tenant-cookie";

/** Datos de sesión que el layout (servidor) pasa al shell; solo valores serializables. */
export interface ShellSession {
  tenants: ContextoItem[];
  activeTenantId: string;
  user: UsuarioMenu;
}

/** Contenido del sidebar: marca, empresa activa, menú de habilitadas y usuario. También es el cajón «Más» del móvil. */
export function AppSidebar({ session, features = enabledFeatures }: { session: ShellSession; features?: readonly Feature[] }) {
  const router = useRouter();

  function changeTenant(id: string) {
    // Mientras la selección de empresa es un flujo mock (#27), la elegida se guarda en la cookie de desarrollo existente.
    setClientCookie(TENANT_COOKIE, id);
    router.refresh();
  }

  return (
    <SidebarContent
      marca={<LogoMarca nombre="Wanka" />}
      selector={
        <SelectorContexto
          items={session.tenants}
          activoId={session.activeTenantId}
          etiqueta="Cambiar de empresa"
          onCambio={changeTenant}
        />
      }
      grupos={sidebarGroups(features)}
      etiquetaMenu="Menú principal"
      usuario={<MenuUsuario usuario={session.user} onCerrarSesion={() => router.push("/login")} />}
    />
  );
}
