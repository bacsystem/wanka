import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The dev badge overlapped the sidebar footer / bottom nav; build errors still surface in the overlay.
  devIndicators: false,
  async redirects() {
    // Navigation groups land on their first module.
    return [
      { source: "/ventas", destination: "/ventas/comprobantes", permanent: false },
      { source: "/inventario", destination: "/inventario/stock", permanent: false },
      { source: "/finanzas", destination: "/finanzas/cuentas-por-cobrar", permanent: false },
      { source: "/configuracion", destination: "/configuracion/sunat", permanent: false },
    ];
  },
};

export default nextConfig;
