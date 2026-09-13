import type { Metadata } from "next";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { KdsBoard } from "@/features/restaurant/components/kds-board";
import { kdsTickets } from "@/features/restaurant/mocks/floor";

export const metadata: Metadata = { title: "Comandas y cocina" };

export default function KdsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Restaurante" title="Tablero de cocina y barra (KDS)" description="Control de tiempos de preparación y tickets de comanda en tiempo real · alerta en rojo a partir de 15 min." status={<StatusBadge tone="success" dot label="Actualizado hace 8 s" />} />
      <KdsBoard tickets={kdsTickets} />
    </div>
  );
}
