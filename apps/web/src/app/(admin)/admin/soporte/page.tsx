import type { Metadata } from "next";
import { SupportScreen } from "@/features/platform-admin/components/support-screen";
export const metadata: Metadata = { title: "Soporte y tickets" };
export default function AdminSupportPage() { return <SupportScreen />; }
