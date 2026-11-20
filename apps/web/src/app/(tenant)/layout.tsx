import type { Viewport } from "next";
import { AppDocument, rootMetadata } from "@/components/providers/app-document";

// Root layout of the tenant app and its auth pages (the admin console has its own: see app/(admin) and app/(admin-auth)).
export const metadata = rootMetadata;

// `viewport-fit=cover` deja que la barra inferior del DS respete la zona segura del teléfono.
export const viewport: Viewport = { viewportFit: "cover" };

export default function TenantRootLayout({ children }: LayoutProps<"/">) {
  return <AppDocument>{children}</AppDocument>;
}
