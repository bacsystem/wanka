import type { Metadata } from "next";
import { StaffScreen } from "@/features/platform-admin/components/staff-screen";
export const metadata: Metadata = { title: "Usuarios de plataforma" };
export default function AdminStaffPage() { return <StaffScreen />; }
