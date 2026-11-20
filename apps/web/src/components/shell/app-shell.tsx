import type { ReactNode } from "react";
import { Shell } from "khipu-design-system/nav/shell";
import { AppBottomBar } from "./app-bottom-bar";
import { AppSidebar, type ShellSession } from "./app-sidebar";
import { AppTopBar } from "./app-top-bar";

/** Shell de la app de la empresa sobre el design system: sidebar (rail entre 768 y 1279 px), top bar y barra inferior en móvil. */
export function AppShell({ session, children }: { session: ShellSession; children: ReactNode }) {
  return (
    <Shell
      rail="auto"
      sidebar={<AppSidebar session={session} />}
      topBar={<AppTopBar />}
      barraInferior={<AppBottomBar session={session} />}
    >
      {children}
    </Shell>
  );
}
