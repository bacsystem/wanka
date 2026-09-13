import type { Metadata } from "next";
import { FlagsScreen } from "@/features/platform-admin/components/flags-screen";
export const metadata: Metadata = { title: "Feature flags y módulos" };
export default function Page() { return <FlagsScreen />; }
