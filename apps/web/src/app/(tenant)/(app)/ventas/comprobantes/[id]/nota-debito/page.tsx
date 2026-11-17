import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { DebitNoteForm } from "@/features/documents/components/debit-note-form";
import { getDocumentDetail } from "@/features/documents/mocks/documents";

export const metadata: Metadata = { title: "Nota de débito" };

export default async function DebitNotePage({ params }: PageProps<"/ventas/comprobantes/[id]/nota-debito">) {
  const { id } = await params;
  const doc = getDocumentDetail(decodeURIComponent(id));
  if (!doc) notFound();
  return <DebitNoteForm doc={doc} />;
}
