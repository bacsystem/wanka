import type { Metadata } from "next";
import { MonitoringScreen } from "@/features/platform-admin/components/monitoring-screen";
export const metadata: Metadata = { title: "Monitoreo SUNAT / OSE" };
export default function AdminMonitoringPage() { return <MonitoringScreen />; }
