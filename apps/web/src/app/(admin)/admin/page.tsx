import type { Metadata } from "next";
import { AdminDashboard } from "@/features/platform-admin/components/admin-dashboard";

export const metadata: Metadata = { title: "Dashboard central" };

export default function AdminHomePage() {
  return <AdminDashboard />;
}
