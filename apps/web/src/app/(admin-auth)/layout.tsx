import type { ReactNode } from "react";
import { AppDocument, rootMetadata } from "@/components/providers/app-document";
import { AdminPalette } from "@/features/platform-admin/components/admin-palette";
import { ADMIN_PALETTE } from "@/features/platform-admin/components/admin-palette-id";

// Root layout of the admin login: same document as the console (indigo palette from the first paint).
export const metadata = rootMetadata;

export default function AdminAuthRootLayout({ children }: { children: ReactNode }) {
  return (
    <AppDocument palette={ADMIN_PALETTE}>
      <AdminPalette />
      {children}
    </AppDocument>
  );
}
