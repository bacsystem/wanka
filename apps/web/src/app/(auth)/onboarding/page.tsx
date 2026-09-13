import type { Metadata } from "next";
import { OnboardingScreen } from "@/features/auth/components/onboarding-screen";

export const metadata: Metadata = { title: "Activar empresa" };

export default function OnboardingPage() {
  return <OnboardingScreen />;
}
