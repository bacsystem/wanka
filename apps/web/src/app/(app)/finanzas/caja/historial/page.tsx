import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { ShiftHistory } from "@/features/cash/components/shift-history";
import { shiftsMock } from "@/features/cash/mocks/shifts";

export const metadata: Metadata = { title: "Historial de turnos" };

export default function ShiftHistoryPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/finanzas/caja" />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Caja y arqueo</Button>
      <PageHeader eyebrow="Finanzas · Caja" title="Historial de turnos y Z-report" description="Turnos cerrados por caja con arqueo, diferencias y resumen diario firmado." />
      <ShiftHistory shifts={shiftsMock} />
    </div>
  );
}
