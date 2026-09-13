export type ReceivableStatus = "por_vencer" | "vencido" | "judicial" | "al_dia";

export interface Receivable {
  id: string;
  number: string;
  type: "Factura" | "Boleta";
  customer: string;
  customerRuc: string;
  issuedAt: string;
  dueAt: string;
  termDays: number;
  total: number;
  balance: number;
  installment: string; // "2/3"
  status: ReceivableStatus;
  detraction?: { rate: number; amount: number; deposited: boolean };
}

export const receivablesSummary = {
  portfolio: 52430, invoices: 34, creditLine: 150000,
  overdue: 8120, overdueCount: 5,
  dueSoon: 14350, dueSoonCount: 9,
  collected: 41200, recovery: 83.5, goal: 45000,
  weekly: [
    { week: "Sem 1", real: 9800, projected: 10500 },
    { week: "Sem 2", real: 11200, projected: 11000 },
    { week: "Sem 3", real: 10600, projected: 11500 },
    { week: "Sem 4 (actual)", real: 9600, projected: 12000 },
  ],
  spot: { rate: 72, deposited: 4896, pending: 1840 },
};

export const receivablesMock: Receivable[] = [
  { id: "r1", number: "F001-000835", type: "Factura", customer: "Consorcio Minero del Centro S.A.C.", customerRuc: "20554891204", issuedAt: "2026-08-12", dueAt: "2026-09-15", termDays: 30, total: 6800, balance: 2800, installment: "2/3", status: "por_vencer", detraction: { rate: 12, amount: 816, deposited: true } },
  { id: "r2", number: "F001-000792", type: "Factura", customer: "Inversiones Médicas del Sur E.I.R.L.", customerRuc: "20491028371", issuedAt: "2026-07-01", dueAt: "2026-08-31", termDays: 45, total: 4200, balance: 4200, installment: "1/1", status: "vencido" },
  { id: "r3", number: "F001-000840", type: "Factura", customer: "Transportes & Logística Andina S.A.", customerRuc: "20601284759", issuedAt: "2026-09-10", dueAt: "2026-10-10", termDays: 30, total: 5480, balance: 5480, installment: "1/2", status: "al_dia", detraction: { rate: 10, amount: 548, deposited: false } },
  { id: "r4", number: "F001-000761", type: "Factura", customer: "Clínica Privada Los Olivos S.A.C.", customerRuc: "20478123900", issuedAt: "2026-05-15", dueAt: "2026-06-15", termDays: 30, total: 3100, balance: 3100, installment: "1/1", status: "judicial" },
  { id: "r5", number: "F001-000842", type: "Factura", customer: "Inversiones Alpamayo S.A.C.", customerRuc: "20554182910", issuedAt: "2026-09-12", dueAt: "2026-09-13", termDays: 1, total: 1280, balance: 1280, installment: "1/1", status: "por_vencer" },
  { id: "r6", number: "F001-000811", type: "Factura", customer: "Servicios Logísticos Lima S.A.C.", customerRuc: "20601829411", issuedAt: "2026-08-05", dueAt: "2026-09-04", termDays: 30, total: 2400, balance: 820, installment: "3/3", status: "vencido" },
  { id: "r7", number: "B001-004102", type: "Boleta", customer: "Ana Lucía Paredes Vega", customerRuc: "41234567", issuedAt: "2026-08-28", dueAt: "2026-09-27", termDays: 30, total: 1800, balance: 900, installment: "2/4", status: "al_dia" },
];
