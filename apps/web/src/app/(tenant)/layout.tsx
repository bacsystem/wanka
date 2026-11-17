import { AppDocument, rootMetadata } from "@/components/providers/app-document";

// Root layout of the tenant app and its auth pages (the admin console has its own: see app/(admin) and app/(admin-auth)).
export const metadata = rootMetadata;

export default function TenantRootLayout({ children }: LayoutProps<"/">) {
  return <AppDocument>{children}</AppDocument>;
}
