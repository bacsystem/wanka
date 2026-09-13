import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { DocumentDetail } from "@/features/documents/components/document-detail";
import { getDocumentDetail } from "@/features/documents/mocks/documents";

export async function generateMetadata({ params }: PageProps<"/ventas/comprobantes/[id]">): Promise<Metadata> {
  const { id } = await params;
  return { title: `Comprobante ${id}` };
}

export default async function DocumentDetailPage({ params }: PageProps<"/ventas/comprobantes/[id]">) {
  const { id } = await params;
  const doc = getDocumentDetail(decodeURIComponent(id));
  if (!doc) notFound();
  return <DocumentDetail doc={doc} />;
}
