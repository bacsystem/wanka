import type { Metadata } from "next";
import { PlansScreen } from "@/features/platform-admin/components/plans-screen";
export const metadata: Metadata = { title: "Planes y suscripciones" };
export default function AdminPlansPage() { return <PlansScreen />; }
