import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The dev badge overlapped the sidebar footer / bottom nav; build errors still surface in the overlay.
  devIndicators: false,
};

export default nextConfig;
