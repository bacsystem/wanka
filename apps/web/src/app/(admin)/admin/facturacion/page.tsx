import type { Metadata } from "next";
import { BillingScreen } from "@/features/platform-admin/components/billing-screen";
export const metadata: Metadata = { title: "Facturación del SaaS" };
export default function AdminBillingPage() { return <BillingScreen />; }
