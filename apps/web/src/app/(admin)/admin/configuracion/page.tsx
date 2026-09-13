import type { Metadata } from "next";
import { PlatformConfigScreen } from "@/features/platform-admin/components/platform-config-screen";
export const metadata: Metadata = { title: "Configuración de plataforma" };
export default function Page() { return <PlatformConfigScreen />; }
