import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Feature } from "@/config/navigation";
import { AppBottomBar } from "./app-bottom-bar";
import { AppSidebar, type ShellSession } from "./app-sidebar";
import { AppTopBar } from "./app-top-bar";

const navigation = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
}));

const Icon = (() => <svg />) as unknown as Feature["icon"];
const withClientes: Feature[] = [
  { code: "home", label: "Inicio", href: "/", icon: Icon },
  { code: "customers", label: "Clientes", href: "/clientes", icon: Icon },
];

const session: ShellSession = {
  tenants: [{ id: "t_1", nombre: "Empresa Uno", detalle: "RUC 20000000001" }],
  activeTenantId: "t_1",
  user: { nombre: "Usuario de prueba", email: "usuario@wanka.pe", rol: "Administrador" },
};

afterEach(() => {
  cleanup();
  navigation.pathname = "/";
});

describe("sidebar", () => {
  it("con la lista actual muestra solo Inicio", () => {
    render(<AppSidebar session={session} />);
    const menu = screen.getByRole("navigation", { name: "Menú principal" });
    expect(within(menu).getAllByRole("link").map((l) => l.textContent)).toEqual(["Inicio"]);
    expect(within(menu).queryByText(/Ventas|Clientes|Inventario|Configuración/)).toBeNull();
  });

  it("muestra la empresa activa y el menú de usuario", () => {
    render(<AppSidebar session={session} />);
    expect(screen.getByRole("combobox", { name: "Cambiar de empresa" })).toHaveTextContent("Empresa Uno");
    expect(screen.getByRole("button", { name: "Menú de usuario" })).toBeInTheDocument();
  });

  it("una funcionalidad agregada a la lista aparece en el menú", () => {
    render(<AppSidebar session={session} features={withClientes} />);
    expect(screen.getByRole("link", { name: "Clientes" })).toHaveAttribute("href", "/clientes");
  });
});

describe("barra inferior", () => {
  it("con la lista actual tiene Inicio y Más, sin las pantallas mock", () => {
    render(<AppBottomBar session={session} />);
    const bar = screen.getByRole("navigation", { name: "Navegación principal" });
    expect(within(bar).getAllByRole("link").map((l) => l.textContent)).toEqual(["Inicio"]);
    expect(within(bar).getByRole("button", { name: "Más" })).toBeInTheDocument();
    expect(within(bar).queryByText(/Ventas|Clientes|Inventario/)).toBeNull();
  });

  it("una funcionalidad agregada a la lista aparece en la barra", () => {
    render(<AppBottomBar session={session} features={withClientes} />);
    expect(screen.getByRole("link", { name: "Clientes" })).toBeInTheDocument();
  });
});

describe("top bar y paleta Ctrl+K", () => {
  it("la ubicación de Inicio es el título de la página", () => {
    render(<AppTopBar />);
    expect(screen.getByRole("heading", { level: 1, name: "Inicio" })).toBeInTheDocument();
  });

  it("en una ruta no habilitada no hay miga", () => {
    navigation.pathname = "/ventas";
    render(<AppTopBar />);
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });

  it("la paleta lista solo las habilitadas", async () => {
    render(<AppTopBar />);
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const list = await screen.findByRole("listbox");
    expect(within(list).getAllByRole("option").map((b) => b.textContent)).toEqual([expect.stringContaining("Inicio")]);
    expect(within(list).queryByText(/Ventas|Clientes|Inventario|Comprobante/)).toBeNull();
  });

  it("la paleta ofrece una funcionalidad agregada a la lista", async () => {
    render(<AppTopBar features={withClientes} />);
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(within(await screen.findByRole("listbox")).getByText("Clientes")).toBeInTheDocument();
  });
});
