import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The dev badge overlapped the sidebar footer / bottom nav; build errors still surface in the overlay.
  devIndicators: false,
  // El proxy (proxy.ts cubre /api/*) guarda en memoria el cuerpo de cada petición: por defecto 10 MB. Tope de conventions.md §7 (1 MB).
  experimental: { proxyClientMaxBodySize: "1mb" },
};

export default nextConfig;
