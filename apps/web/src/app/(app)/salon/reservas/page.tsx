import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { ReservationsScreen } from "@/features/restaurant/components/reservations-screen";
import { reservations } from "@/features/restaurant/mocks/delivery";

export const metadata: Metadata = { title: "Reservas" };

export default function ReservationsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Restaurante" title="Reservas" description="Reservas del día por franja horaria con confirmación por WhatsApp y asignación de mesa." />
      <ReservationsScreen reservations={reservations} />
    </div>
  );
}
