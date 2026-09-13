import type { NextConfig } from "next";
import { groupRedirects } from "./src/config/redirects";

const nextConfig: NextConfig = {
  // The dev badge overlapped the sidebar footer / bottom nav; build errors still surface in the overlay.
  devIndicators: false,
  async redirects() {
    // Navigation groups land on the module declared in config/redirects.ts.
    return groupRedirects();
  },
};

export default nextConfig;
