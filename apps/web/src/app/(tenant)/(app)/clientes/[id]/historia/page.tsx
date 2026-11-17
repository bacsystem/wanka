import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ClinicalRecord } from "@/features/clinical/components/clinical-record";
import { customersMock } from "@/features/customers/mocks/customers";

export const metadata: Metadata = { title: "Historia clínica" };

export default async function ClinicalRecordPage({ params }: PageProps<"/clientes/[id]/historia">) {
  const { id } = await params;
  const customer = customersMock.find((c) => c.id === id);
  if (!customer) notFound();
  return <ClinicalRecord customer={customer} />;
}
