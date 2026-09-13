import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PetDetail } from "@/features/veterinary/components/pet-detail";
import { groomingMock, petsMock, vaccinesMock, vetEvolutions } from "@/features/veterinary/mocks/pets";

export async function generateMetadata({ params }: PageProps<"/mascotas/[id]">): Promise<Metadata> {
  const { id } = await params;
  return { title: petsMock.find((p) => p.id === id)?.name ?? "Mascota" };
}

export default async function PetPage({ params }: PageProps<"/mascotas/[id]">) {
  const { id } = await params;
  const pet = petsMock.find((p) => p.id === id);
  if (!pet) notFound();
  return <PetDetail pet={pet} vaccines={vaccinesMock} evolutions={vetEvolutions} grooming={groomingMock} />;
}
