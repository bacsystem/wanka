import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { CreditNoteWizard } from "@/features/documents/components/credit-note-wizard";
import { getDocumentDetail } from "@/features/documents/mocks/documents";

export const metadata: Metadata = { title: "Nota de crédito" };

export default async function CreditNotePage({ params }: PageProps<"/ventas/comprobantes/[id]/nota-credito">) {
  const { id } = await params;
  const doc = getDocumentDetail(decodeURIComponent(id));
  if (!doc) notFound();
  return <CreditNoteWizard doc={doc} />;
}
