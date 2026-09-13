import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { GroomingScreen } from "@/features/veterinary/components/grooming-screen";
import { groomingJobs } from "@/features/veterinary/mocks/hospital";

export const metadata: Metadata = { title: "Grooming" };

export default function GroomingPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Veterinaria" title="Peluquería y grooming" description="Agenda del día por groomer con estados y aviso al dueño con foto cuando la mascota está lista." />
      <GroomingScreen jobs={groomingJobs} />
    </div>
  );
}
