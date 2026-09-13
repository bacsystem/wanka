import type { Metadata } from "next";
import { BedDouble, HeartPulse, Sparkles, Syringe } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { HospitalScreen } from "@/features/veterinary/components/hospital-screen";
import { hospitalSummary as s, kennels, medsMock, vitalsMock } from "@/features/veterinary/mocks/hospital";

export const metadata: Metadata = { title: "Hospitalización" };

export default function HospitalPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Veterinaria" title="Hospitalización y pacientes críticos" description="Kennels por área, constantes vitales por turno, plan terapéutico con trazabilidad y alta con liquidación a POS." status={<StatusBadge tone="info" dot label={`Capacidad ${s.occupied}/${s.capacity}`} />} />
      <StatGrid>
        <StatCard label="Internados" value={String(s.occupied)} icon={BedDouble} hint={`${s.free} kennels libres · ${s.cleaning} en desinfección`} />
        <StatCard label="Críticos (UCI / aislamiento)" value="2" icon={HeartPulse} tone="danger" hint="Luna (FLUTD) · Simba (panleucopenia)" />
        <StatCard label="Medicaciones pendientes (turno)" value="5" icon={Syringe} tone="warning" hint="Próxima 14:00 · metronidazol K-01" />
        <StatCard label="Altas previstas hoy" value="1" icon={Sparkles} tone="success" hint="Nala (K-08) · 17:00" />
      </StatGrid>
      <HospitalScreen kennels={kennels} vitals={vitalsMock} meds={medsMock} />
    </div>
  );
}
