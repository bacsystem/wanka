import { expect, test } from "@playwright/test";

const screens: { path: string; heading: RegExp }[] = [
  { path: "/", heading: /^Inicio$/ },
  { path: "/ventas/nueva", heading: /Nueva venta/ },
  { path: "/ventas/comprobantes", heading: /Comprobantes de pago/ },
  { path: "/ventas/comprobantes/F001-00000842", heading: /Factura electrónica F001-00000842/ },
  { path: "/ventas/proformas", heading: /Cotizaciones y proformas/ },
  { path: "/ventas/guias", heading: /Guías de remisión/ },
  { path: "/clientes", heading: /Directorio de clientes/ },
  { path: "/agenda", heading: /Agenda de citas/ },
  { path: "/inventario/stock", heading: /Control de stock/ },
  { path: "/compras", heading: /Gestión de compras y SIRE/ },
  { path: "/finanzas/cuentas-por-cobrar", heading: /cuentas por cobrar/i },
  { path: "/finanzas/caja", heading: /Control de caja y turnos/ },
  { path: "/reportes", heading: /Reportes tributarios/ },
  { path: "/configuracion/sunat", heading: /Facturación electrónica y certificado/ },
  { path: "/configuracion/usuarios", heading: /Gobierno de identidad/ },
  { path: "/catalogo", heading: /productos y servicios/i },
  { path: "/catalogo/productos", heading: /productos y servicios/i },
  { path: "/catalogo/categorias", heading: /productos y servicios/i },
  { path: "/inventario/movimientos", heading: /Movimientos de inventario/ },
  { path: "/inventario/almacenes", heading: /Almacenes y sucursales/ },
  { path: "/compras/proveedores", heading: /^Proveedores$/ },
  { path: "/compras/ordenes", heading: /Órdenes de compra/ },
  { path: "/inventario/kardex", heading: /Kardex físico y valorizado/ },
  { path: "/clientes/u2", heading: /María Castillo Quispe/ },
  { path: "/clientes/u2/historia", heading: /María Castillo Quispe/ },
  { path: "/ventas/proformas/COT-2026-00128", heading: /Cotización COT-2026-00128/ },
  { path: "/ventas/comprobantes/F001-00000842/nota-credito", heading: /Emitir nota de crédito/ },
  { path: "/inventario/recepcion/T001-000428", heading: /Recepción de traslado/ },
  { path: "/inventario/toma-fisica", heading: /Toma de inventario físico/ },
  { path: "/ventas/guias/T001-000145", heading: /Guía de remisión electrónica remitente T001-000145/ },
  { path: "/compras/ordenes/nueva", heading: /Nueva orden de compra/ },
  { path: "/compras/proveedores/s1", heading: /Dental Import/ },
  { path: "/salon", heading: /Salón y mesas/ },
  { path: "/salon/cocina", heading: /Tablero de cocina/ },
  { path: "/mascotas", heading: /Pacientes \(mascotas\)/ },
  { path: "/mascotas/p1", heading: /^Rocky$/ },
  { path: "/reportes/liquidacion", heading: /Liquidación mensual IGV-Renta/ },
  { path: "/perfil", heading: /Mi perfil y seguridad/ },
  { path: "/acceso-denegado", heading: /Sin permisos/ },
  { path: "/salon/delivery", heading: /^Delivery$/ },
  { path: "/salon/reservas", heading: /^Reservas$/ },
  { path: "/hospitalizacion", heading: /Hospitalización y pacientes críticos/ },
  { path: "/grooming", heading: /Peluquería y grooming/ },
  { path: "/catalogo/variantes/POL-001", heading: /SKU POL-001/ },
  { path: "/reportes/gestion", heading: /Reporte de gestión comercial/ },
  { path: "/finanzas/cuentas-por-pagar", heading: /Cuentas por pagar/ },
  { path: "/finanzas/caja/historial", heading: /Historial de turnos/ },
  { path: "/configuracion/auditoria", heading: /Auditoría del sistema/ },
  { path: "/configuracion/integraciones", heading: /^Integraciones$/ },
  { path: "/ventas/proformas/nueva", heading: /Nueva cotización COT-2026-00129/ },
  { path: "/ventas/comprobantes/F001-00000842/nota-debito", heading: /Emitir nota de débito/ },
  { path: "/login", heading: /Bienvenido de nuevo/ },
  { path: "/seleccionar-empresa", heading: /elige con qué empresa trabajar/ },
  { path: "/onboarding", heading: /Configura tu espacio de trabajo/ },
];

for (const s of screens) {
  test(`renders ${s.path} without console errors`, async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(s.path);
    await expect(page.getByRole("heading", { level: 1, name: s.heading })).toBeVisible();
    // No horizontal page overflow at any viewport. Polled instead of a fixed wait: on tablet the sidebar
    // auto-collapse transition is still running right after the heading appears.
    // On mobile emulation Chrome widens the layout viewport instead of scrolling, so compare against the visual viewport too.
    await expect
      .poll(
        () => page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, window.innerWidth - window.visualViewport!.width)),
        { message: "horizontal overflow", timeout: 3000 },
      )
      .toBeLessThanOrEqual(0);
    expect(errors).toEqual([]);
  });
}

test("group routes redirect to their first module", async ({ page }) => {
  await page.goto("/finanzas");
  await expect(page).toHaveURL(/\/finanzas\/cuentas-por-cobrar$/);
});
