import type { Metadata } from "next";
import { AuditGlobalScreen } from "@/features/platform-admin/components/audit-global-screen";
export const metadata: Metadata = { title: "Auditoría y seguridad" };
export default function Page() { return <AuditGlobalScreen />; }
