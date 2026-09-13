import type { Metadata } from "next";
import { GuideDetail } from "@/features/dispatch-guides/components/guide-detail";
import { guideDetailMock } from "@/features/dispatch-guides/mocks/guide-detail";
import { guidesMock } from "@/features/dispatch-guides/mocks/guides";

export const metadata: Metadata = { title: "Guía de remisión" };

export default async function GuidePage({ params }: PageProps<"/ventas/guias/[id]">) {
  const { id } = await params;
  const g = guidesMock.find((x) => x.number === id);
  const guide = g ? { ...guideDetailMock, number: g.number, issuedAt: `${g.issuedAt}T09:15:00`, transferAt: g.transferAt, sunat: g.status === "rechazado" ? ("rechazado" as const) : g.status === "pendiente" ? ("pendiente" as const) : ("aceptado" as const), logistic: g.status === "entregado" ? "Entregada" : g.status === "en_transito" ? "En tránsito" : "Por despachar", vehicle: { ...guideDetailMock.vehicle, plate: g.plate }, driver: { name: g.carrier, license: g.carrierDoc }, weightKg: g.weightKg } : guideDetailMock;
  return <GuideDetail guide={guide} />;
}
