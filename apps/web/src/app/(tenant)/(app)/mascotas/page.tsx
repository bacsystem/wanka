import type { Metadata } from "next";
import { Bell, PawPrint, Stethoscope, Syringe } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { PetsList } from "@/features/veterinary/components/pets-list";
import { petsMock } from "@/features/veterinary/mocks/pets";

export const metadata: Metadata = { title: "Mascotas" };

export default function PetsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Veterinaria" title="Pacientes (mascotas)" description="Fichas clínicas, carné de vacunas con recordatorios y control de peso por paciente." status={<StatusBadge tone="success" dot label="Lector de microchip en línea" />} />
      <StatGrid>
        <StatCard label="Mascotas registradas" value="1,412" icon={PawPrint} hint="1,020 caninos · 360 felinos · 32 otros" />
        <StatCard label="Consultas hoy" value="18" icon={Stethoscope} hint="4 en espera · 2 hospitalizados" />
        <StatCard label="Vacunas por vencer (30 d)" value="37" icon={Syringe} tone="warning" hint="Recordatorios WhatsApp programados" />
        <StatCard label="Recordatorios enviados" value="124" icon={Bell} tone="success" hint="Últimos 30 días · 68% confirmados" />
      </StatGrid>
      <PetsList pets={petsMock} />
    </div>
  );
}
