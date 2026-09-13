import type { Metadata } from "next";
import { ReceptionScreen } from "@/features/inventory/components/reception-screen";
import { receptionMock } from "@/features/inventory/mocks/reception";

export const metadata: Metadata = { title: "Recepción de traslado" };

export default async function ReceptionPage({ params }: PageProps<"/inventario/recepcion/[guia]">) {
  const { guia } = await params;
  return <ReceptionScreen data={{ ...receptionMock, guide: decodeURIComponent(guia) }} />;
}
