import { describe, expect, it } from "vitest";
import { bottomBarItems, breadcrumbsFor, paletteCommands, sidebarGroups } from "./navigation-menus";
import { bottomBarFeatures, enabledFeatures, featureForPath, isEnabledPath, type Feature } from "./navigation";
import { DISABLED_ROUTE, routeAccess } from "./route-access";

const Icon = (() => null) as unknown as Feature["icon"];
const fixture: Feature[] = [
  { code: "home", label: "Inicio", href: "/", icon: Icon },
  { code: "customers", label: "Clientes", href: "/clientes", icon: Icon },
  { code: "settings", label: "Ajustes", href: "/ajustes", icon: Icon, section: "operaciones" },
];

describe("lista de funcionalidades habilitadas", () => {
  it("hoy solo habilita Inicio", () => {
    expect(enabledFeatures.map((f) => f.label)).toEqual(["Inicio"]);
  });

  it("los códigos y las rutas no se repiten", () => {
    expect(new Set(enabledFeatures.map((f) => f.code)).size).toBe(enabledFeatures.length);
    expect(new Set(enabledFeatures.map((f) => f.href)).size).toBe(enabledFeatures.length);
  });
});

describe("rutas", () => {
  it.each(["/ventas", "/ventas/nueva", "/clientes", "/clientes/c_001", "/configuracion/usuarios", "/inventario/stock", "/perfil", "/acceso-denegado", "/no-habilitada", "/Ventas"])(
    "%s no está habilitada (404)",
    (path) => {
      expect(isEnabledPath(path)).toBe(false);
      expect(routeAccess(path)).toBe("disabled");
    },
  );

  it("Inicio solo coincide exacta", () => {
    expect(routeAccess("/")).toBe("enabled");
    expect(isEnabledPath("/cualquier-cosa")).toBe(false);
  });

  it.each(["/login", "/seleccionar-empresa", "/onboarding", "/admin", "/admin/empresas/e_1"])("%s es del flujo de ingreso o de la consola", (path) => {
    expect(routeAccess(path)).toBe("open");
  });

  it.each(["/loginx", "/administrador", "/login-falso/ventas"])("%s no se cuela por prefijo", (path) => {
    expect(routeAccess(path)).toBe("disabled");
  });

  it("la ruta interna del 404 también responde 404", () => {
    expect(routeAccess(DISABLED_ROUTE)).toBe("disabled");
  });

  it("habilitar una funcionalidad habilita su ruta y sus subrutas, y nada más", () => {
    expect(routeAccess("/clientes", fixture)).toBe("enabled");
    expect(routeAccess("/clientes/c_001", fixture)).toBe("enabled");
    expect(routeAccess("/clientesx", fixture)).toBe("disabled");
    expect(routeAccess("/ventas", fixture)).toBe("disabled");
  });

  it("featureForPath elige la funcionalidad más específica", () => {
    const nested: Feature[] = [...fixture, { code: "customer-groups", label: "Grupos", href: "/clientes/grupos", icon: Icon }];
    expect(featureForPath("/clientes/grupos/x", nested)?.code).toBe("customer-groups");
    expect(featureForPath("/clientes/c_1", nested)?.code).toBe("customers");
    expect(featureForPath("/otra", nested)).toBeUndefined();
  });
});

describe("menús derivados de la lista", () => {
  it("hoy los tres menús muestran solo Inicio", () => {
    expect(sidebarGroups().flatMap((g) => g.items.map((i) => i.label))).toEqual(["Inicio"]);
    expect(bottomBarItems().map((i) => i.label)).toEqual(["Inicio"]);
    expect(paletteCommands().map((c) => c.label)).toEqual(["Inicio"]);
  });

  it("una funcionalidad habilitada aparece en los tres menús y en las rutas", () => {
    expect(sidebarGroups(fixture).flatMap((g) => g.items.map((i) => i.label))).toEqual(["Inicio", "Clientes", "Ajustes"]);
    expect(bottomBarItems(fixture).map((i) => i.label)).toEqual(["Inicio", "Clientes", "Ajustes"]);
    expect(paletteCommands(fixture).map((c) => c.href)).toEqual(["/", "/clientes", "/ajustes"]);
    for (const f of fixture) expect(routeAccess(f.href, fixture)).toBe("enabled");
  });

  it("agrupa por sección y omite los grupos vacíos", () => {
    expect(sidebarGroups(fixture).map((g) => g.titulo)).toEqual(["Módulos core", "Operaciones"]);
    expect(sidebarGroups(fixture.slice(0, 1)).map((g) => g.titulo)).toEqual(["Módulos core"]);
    expect(sidebarGroups([])).toEqual([]);
  });

  it("la barra inferior tiene como máximo 4 entradas y el resto queda en «Más»", () => {
    const many: Feature[] = Array.from({ length: 6 }, (_, i) => ({ code: `f${i}`, label: `F${i}`, href: `/f${i}`, icon: Icon }));
    expect(bottomBarFeatures(many).map((f) => f.code)).toEqual(["f0", "f1", "f2", "f3"]);
    expect(sidebarGroups(many).flatMap((g) => g.items)).toHaveLength(6);
  });

  it("la miga existe solo en rutas habilitadas", () => {
    expect(breadcrumbsFor("/")).toEqual([{ prefijo: "/", seccion: "Wanka", pagina: "Inicio" }]);
    expect(breadcrumbsFor("/ventas")).toEqual([]);
  });
});
