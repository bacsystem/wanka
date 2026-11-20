"use client";

import { BarraInferior } from "khipu-design-system/nav/barra-inferior";
import { bottomBarItems } from "@/config/navigation-menus";
import { enabledFeatures, type Feature } from "@/config/navigation";
import { AppSidebar, type ShellSession } from "./app-sidebar";

/** Barra inferior del móvil (≤767 px): las primeras habilitadas y «Más», que abre el sidebar completo. */
export function AppBottomBar({ session, features = enabledFeatures }: { session: ShellSession; features?: readonly Feature[] }) {
  return (
    <BarraInferior
      items={bottomBarItems(features)}
      mas={{ etiqueta: "Más", contenido: <AppSidebar session={session} features={features} /> }}
    />
  );
}
