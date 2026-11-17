import type { Metadata } from "next";
import { PhysicalCountScreen } from "@/features/inventory/components/physical-count-screen";
import { physicalCountMock } from "@/features/inventory/mocks/reception";

export const metadata: Metadata = { title: "Toma de inventario físico" };

export default function PhysicalCountPage() {
  return <PhysicalCountScreen lines={physicalCountMock} />;
}
