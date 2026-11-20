"use client";

import { useState } from "react";
import { usePathname } from "next/navigation";
import { Paleta } from "khipu-design-system/navegacion/paleta";
import { TopBar } from "khipu-design-system/nav/top-bar";
import { breadcrumbsFor, paletteCommands } from "@/config/navigation-menus";
import { enabledFeatures, type Feature } from "@/config/navigation";

/** Barra superior (ubicación y búsqueda) con la paleta Ctrl+K, que lee la misma lista de habilitadas que el sidebar. */
export function AppTopBar({ features = enabledFeatures }: { features?: readonly Feature[] }) {
  const pathname = usePathname();
  const [paletteOpen, setPaletteOpen] = useState(false);

  return (
    <>
      <TopBar
        migas={breadcrumbsFor(pathname, features)}
        migaComoTitulo
        buscador={{ placeholder: "Buscar…", atajo: "Ctrl K", onClick: () => setPaletteOpen(true) }}
      />
      <Paleta
        comandos={paletteCommands(features)}
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        placeholder="Ir a una funcionalidad…"
      />
    </>
  );
}
