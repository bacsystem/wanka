import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { QuoteDetail } from "@/features/quotes/components/quote-detail";
import { quoteDetailMock } from "@/features/quotes/mocks/quote-detail";
import { quotesMock } from "@/features/quotes/mocks/quotes";

export async function generateMetadata({ params }: PageProps<"/ventas/proformas/[id]">): Promise<Metadata> {
  const { id } = await params;
  return { title: `Cotización ${id}` };
}

export default async function QuotePage({ params }: PageProps<"/ventas/proformas/[id]">) {
  const { id } = await params;
  const q = quotesMock.find((x) => x.code === id);
  if (!q) notFound();
  const quote = { ...quoteDetailMock, code: q.code, status: (q.status === "aprobada" ? "aprobada" : "aprobada") as "aprobada", customer: { ...quoteDetailMock.customer, name: q.customerName, ruc: q.customerRuc }, issuedAt: `${q.issuedAt}T09:15:00`, expiresAt: q.expiresAt };
  return <QuoteDetail quote={quote} />;
}
