import type { SunatStatus } from "@/types/domain";

export interface CustomerDetail {
  id: string;
  kpis: { billed: number; visits: number; debt: number; creditLimit: number; appointments: number; attendance: number; lastVisit: string; avgTicket: number };
  documents: { type: "boleta" | "factura"; number: string; date: string; total: number; status: SunatStatus; note?: string }[];
  treatments: { name: string; professional: string; status: string; payment: string; date: string }[];
  nextAppointment?: { date: string; time: string; place: string; professional: string; reason: string };
  alerts: { level: "danger" | "info"; title: string; text: string }[];
  billing: { docType: string; kind: string; address: string; email: string; insurer?: string; policy?: string; coverage?: string };
  ledger: { date: string; concept: string; debit: number; credit: number; balance: number }[];
  files: { name: string; kind: string; date: string; size: string }[];
}

export const customerDetails: Record<string, CustomerDetail> = {
  u2: {
    id: "u2",
    kpis: { billed: 1450, visits: 6, debt: 0, creditLimit: 1000, appointments: 12, attendance: 100, lastVisit: "2026-09-10", avgTicket: 120.83 },
    documents: [
      { type: "boleta", number: "B001-000492", date: "2026-09-10", total: 180, status: "aceptado" },
      { type: "boleta", number: "B001-000410", date: "2026-08-10", total: 350, status: "aceptado" },
      { type: "factura", number: "F001-000780", date: "2026-06-14", total: 520, status: "aceptado", note: "Copago Rimac EPS" },
      { type: "boleta", number: "B001-000305", date: "2026-03-02", total: 400, status: "aceptado" },
    ],
    treatments: [
      { name: "Profilaxis completa con ultrasonido y fluorización", professional: "Dra. Claudia Ramos", status: "Completado", payment: "Cancelado (POS Izipay)", date: "2026-09-10" },
      { name: "Curación con resina fotocurable 3M Filtek (pieza 3.6 – oclusal)", professional: "Dr. Carlos Mendoza", status: "Completado", payment: "Cancelado (Yape)", date: "2026-08-10" },
      { name: "Endodoncia unirradicular y reconstrucción de muñón", professional: "Dra. Claudia Ramos", status: "Completado", payment: "Copago aseguradora", date: "2026-06-14" },
    ],
    nextAppointment: { date: "2026-09-18", time: "09:30", place: "Sede Miraflores · Sillón quirúrgico 1", professional: "Dra. Claudia Ramos (endodoncia y estética)", reason: "Control de obturación y pulido estético" },
    alerts: [
      { level: "danger", title: "Alergia severa a medicamentos", text: "Penicilina y derivados betalactámicos. Contraindicada amoxicilina; en profilaxis antibiótica usar clindamicina 300 mg o eritromicina." },
      { level: "info", title: "Condición cardiovascular", text: "Hipertensión arterial leve controlada (120/80 mmHg). Usar anestésico con vasoconstrictor 1:200,000 con precaución." },
    ],
    billing: { docType: "DNI 72109843", kind: "Persona natural (sin negocio)", address: "Av. 28 de Julio 850, Dpto. 402, Miraflores, Lima", email: "maria.castillo@gmail.com", insurer: "Rimac Seguros EPS", policy: "849201-1", coverage: "80% · copago 20%" },
    ledger: [
      { date: "2026-09-10", concept: "Boleta B001-000492 · profilaxis", debit: 180, credit: 0, balance: 180 },
      { date: "2026-09-10", concept: "Pago POS Izipay", debit: 0, credit: 180, balance: 0 },
      { date: "2026-08-10", concept: "Boleta B001-000410 · curación", debit: 350, credit: 0, balance: 350 },
      { date: "2026-08-10", concept: "Pago Yape", debit: 0, credit: 350, balance: 0 },
    ],
    files: [
      { name: "Consentimiento informado – endodoncia.pdf", kind: "Consentimiento", date: "2026-06-14", size: "212 KB" },
      { name: "Radiografía periapical 3.6.jpg", kind: "Imagen", date: "2026-09-12", size: "1.4 MB" },
      { name: "Carta de garantía Rimac EPS.pdf", kind: "Aseguradora", date: "2026-06-10", size: "98 KB" },
    ],
  },
};

const fallback = customerDetails.u2;
export function getCustomerDetail(id: string): CustomerDetail {
  return customerDetails[id] ?? { ...fallback, id };
}
