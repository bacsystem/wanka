import type { Metadata } from "next";
import { NewQuoteWizard } from "@/features/quotes/components/new-quote-wizard";

export const metadata: Metadata = { title: "Nueva cotización" };

export default function NewQuotePage() {
  return <NewQuoteWizard />;
}
