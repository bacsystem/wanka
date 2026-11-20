import { expect, test } from "./fixtures";

// `story`: pantalla mock inaccesible (404) hasta que esa historia la rehaga sobre el DS, la conecte a la API y la habilite.
const screens: { path: string; heading: RegExp; story?: string }[] = [
  { path: "/", heading: /^Inicio$/ },
  { path: "/ventas/nueva", heading: /Nueva venta/, story: "#48" },
  { path: "/ventas/comprobantes", heading: /Comprobantes de pago/, story: "#52" },
  { path: "/ventas/comprobantes/F001-00000842", heading: /Factura electrónica F001-00000842/, story: "#52" },
  { path: "/ventas/proformas", heading: /Cotizaciones y proformas/, story: "#53" },
  { path: "/ventas/guias", heading: /Guías de remisión/, story: "#61" },
  { path: "/clientes", heading: /Directorio de clientes/, story: "#34" },
  { path: "/agenda", heading: /Agenda de citas/, story: "#75" },
  { path: "/inventario/stock", heading: /Control de stock/, story: "#42" },
  { path: "/compras", heading: /Gestión de compras y SIRE/, story: "#63" },
  { path: "/finanzas/cuentas-por-cobrar", heading: /cuentas por cobrar/i, story: "#69" },
  { path: "/finanzas/caja", heading: /Control de caja y turnos/, story: "#66" },
  { path: "/reportes", heading: /Reportes tributarios/, story: "#73" },
  { path: "/configuracion/sunat", heading: /Facturación electrónica y certificado/, story: "#55" },
  { path: "/configuracion/usuarios", heading: /Gobierno de identidad/, story: "#28" },
  { path: "/catalogo", heading: /productos y servicios/i, story: "#37" },
  { path: "/catalogo/productos", heading: /productos y servicios/i, story: "#37" },
  { path: "/catalogo/categorias", heading: /productos y servicios/i, story: "#37" },
  { path: "/inventario/movimientos", heading: /Movimientos de inventario/, story: "#43" },
  { path: "/inventario/almacenes", heading: /Almacenes y sucursales/, story: "#41" },
  { path: "/compras/proveedores", heading: /^Proveedores$/, story: "#62" },
  { path: "/compras/ordenes", heading: /Órdenes de compra/, story: "#64" },
  { path: "/inventario/kardex", heading: /Kardex físico y valorizado/, story: "#45" },
  { path: "/clientes/u2", heading: /María Castillo Quispe/, story: "#34" },
  { path: "/clientes/u2/historia", heading: /María Castillo Quispe/, story: "#34" },
  { path: "/ventas/proformas/COT-2026-00128", heading: /Cotización COT-2026-00128/, story: "#53" },
  { path: "/ventas/comprobantes/F001-00000842/nota-credito", heading: /Emitir nota de crédito/, story: "#52" },
  { path: "/inventario/recepcion/T001-000428", heading: /Recepción de traslado/, story: "#65" },
  { path: "/inventario/toma-fisica", heading: /Toma de inventario físico/, story: "#46" },
  { path: "/ventas/guias/T001-000145", heading: /Guía de remisión electrónica remitente T001-000145/, story: "#61" },
  { path: "/compras/ordenes/nueva", heading: /Nueva orden de compra/, story: "#64" },
  { path: "/compras/proveedores/s1", heading: /Dental Import/, story: "#62" },
  { path: "/salon", heading: /Salón y mesas/, story: "#113" },
  { path: "/salon/cocina", heading: /Tablero de cocina/, story: "#114" },
  { path: "/mascotas", heading: /Pacientes \(mascotas\)/, story: "#116" },
  { path: "/mascotas/p1", heading: /^Rocky$/, story: "#116" },
  { path: "/reportes/liquidacion", heading: /Liquidación mensual IGV-Renta/, story: "#74" },
  { path: "/perfil", heading: /Mi perfil y seguridad/, story: "#30" },
  { path: "/acceso-denegado", heading: /Sin permisos/, story: "#29" },
  { path: "/salon/delivery", heading: /^Delivery$/, story: "#115" },
  { path: "/salon/reservas", heading: /^Reservas$/, story: "#115" },
  { path: "/hospitalizacion", heading: /Hospitalización y pacientes críticos/, story: "#118" },
  { path: "/grooming", heading: /Peluquería y grooming/, story: "#118" },
  { path: "/catalogo/variantes/POL-001", heading: /SKU POL-001/, story: "#40" },
  { path: "/reportes/gestion", heading: /Reporte de gestión comercial/, story: "#72" },
  { path: "/finanzas/cuentas-por-pagar", heading: /Cuentas por pagar/, story: "#70" },
  { path: "/finanzas/caja/historial", heading: /Historial de turnos/, story: "#68" },
  { path: "/configuracion/auditoria", heading: /Auditoría del sistema/, story: "#79" },
  { path: "/configuracion/integraciones", heading: /^Integraciones$/, story: "#77" },
  { path: "/ventas/proformas/nueva", heading: /Nueva cotización COT-2026-00129/, story: "#53" },
  { path: "/ventas/comprobantes/F001-00000842/nota-debito", heading: /Emitir nota de débito/, story: "#52" },
  { path: "/login", heading: /Iniciar sesión/ },
  { path: "/seleccionar-empresa", heading: /elige con qué empresa trabajar/ },
  { path: "/onboarding", heading: /Configura tu espacio de trabajo/ },
];

for (const s of screens) {
  const run = s.story ? test.skip : test;
  run(`renders ${s.path} without console errors`, async ({ page }) => {
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
