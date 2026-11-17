import type { ReactNode } from "react";
import { AppDocument, rootMetadata } from "@/components/providers/app-document";
import { AdminPalette } from "@/features/platform-admin/components/admin-palette";
import { ADMIN_PALETTE } from "@/features/platform-admin/components/admin-palette-id";

// Root layout of the admin console: its own <html data-paleta="indigo"> so the first paint already wears the console palette.
export const metadata = rootMetadata;

export default function AdminRootLayout({ children }: { children: ReactNode }) {
  return (
    <AppDocument palette={ADMIN_PALETTE}>
      <AdminPalette />
      {children}
    </AppDocument>
  );
}
