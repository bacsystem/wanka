import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { CustomerDetail } from "@/features/customers/components/customer-detail";
import { getCustomerDetail } from "@/features/customers/mocks/customer-detail";
import { customersMock } from "@/features/customers/mocks/customers";

export async function generateMetadata({ params }: PageProps<"/clientes/[id]">): Promise<Metadata> {
  const { id } = await params;
  return { title: customersMock.find((c) => c.id === id)?.name ?? "Cliente" };
}

export default async function CustomerPage({ params }: PageProps<"/clientes/[id]">) {
  const { id } = await params;
  const customer = customersMock.find((c) => c.id === id);
  if (!customer) notFound();
  return <CustomerDetail customer={customer} detail={getCustomerDetail(id)} />;
}
