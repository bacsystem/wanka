import type { Metadata } from "next";
import { Armchair, CalendarX2, CreditCard, CalendarDays } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCompact, StatGrid } from "@/components/shared/stat-card";
import { Agenda } from "@/features/appointments/components/agenda";
import { agendaSummary as s, appointmentsMock, chairs, hours } from "@/features/appointments/mocks/appointments";

export const metadata: Metadata = { title: "Agenda" };

export default function AgendaPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Atención y clínica" title="Agenda de citas y turnos" description="Programación por sillón y especialista, sala de espera y paso directo al cobro en POS." />
      <StatGrid>
        <StatCompact label="Citas programadas hoy" value={String(s.today)} icon={CalendarDays} tone="info" />
        <StatCompact label="En sala de espera" value={String(s.waiting)} icon={Armchair} />
        <StatCompact label="Listos para facturar POS" value={String(s.readyToBill)} icon={CreditCard} tone="default" />
        <StatCompact label="Canceladas / no asistió" value={String(s.cancelled)} icon={CalendarX2} tone="danger" emphasizeValue />
      </StatGrid>
      <Agenda chairs={chairs} appointments={appointmentsMock} hours={hours} />
    </div>
  );
}
