import type { Metadata } from "next";
import { AdminLoginScreen } from "@/features/platform-admin/components/admin-login-screen";
export const metadata: Metadata = { title: "Ingreso · Wanka Admin" };
export default function AdminLoginPage() { return <AdminLoginScreen />; }
