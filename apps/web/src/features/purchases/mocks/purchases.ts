export type SireStatus = "aceptado" | "discrepancia" | "excluido";
export type PayStatus = "pagado" | "pendiente" | "por_vencer";

export interface Purchase {
  id: string;
  cuo: string;
  issuedAt: string;
  dueAt?: string;
  type: "01" | "03" | "07" | "14";
  number: string;
  supplier: string;
  supplierRuc: string;
  base: number;
  igv: number;
  total: number;
  detraction?: { rate: number; amount: number; paid: boolean };
  payStatus: PayStatus;
  sire: SireStatus;
  expenseClass: string;
}

export const purchasesSummary = {
  monthTotal: 28450, monthCount: 42, monthDelta: 4.2,
  fiscalCredit: 4339.83, pendingPay: 9200, pendingCount: 8, dueSoon: 2,
  sireRate: 98, validated: 41, discrepancies: 1,
  discrepancy: { doc: "F001-000429", supplier: "Importaciones Biomédicas E.I.R.L.", detail: "SUNAT registró T.C. 3.742 (S/ 4,116.20); el ERP tiene 3.748 (S/ 4,122.80). Diferencia S/ 6.60 en base." },
};

export const purchasesMock: Purchase[] = [
  { id: "p1", cuo: "000201", issuedAt: "2026-09-11", dueAt: "2026-10-11", type: "01", number: "F001-000429", supplier: "Importaciones Biomédicas E.I.R.L.", supplierRuc: "20512389012", base: 4122.8, igv: 742.1, total: 4864.9, detraction: { rate: 12, amount: 583.79, paid: false }, payStatus: "pendiente", sire: "discrepancia", expenseClass: "Insumos clínicos" },
  { id: "p2", cuo: "000200", issuedAt: "2026-09-10", type: "01", number: "F002-011820", supplier: "Dental Import S.A.C.", supplierRuc: "20487120933", base: 2150, igv: 387, total: 2537, payStatus: "pagado", sire: "aceptado", expenseClass: "Material dental" },
  { id: "p3", cuo: "000199", issuedAt: "2026-09-09", dueAt: "2026-09-14", type: "01", number: "E001-000982", supplier: "Distribuidora Médica del Sur S.R.L.", supplierRuc: "20489123019", base: 1380, igv: 248.4, total: 1628.4, payStatus: "por_vencer", sire: "aceptado", expenseClass: "Medicamentos" },
  { id: "p4", cuo: "000198", issuedAt: "2026-09-08", type: "14", number: "R001-8823110", supplier: "Luz del Sur S.A.A.", supplierRuc: "20331898008", base: 412.5, igv: 74.25, total: 486.75, payStatus: "pagado", sire: "aceptado", expenseClass: "Servicios públicos" },
  { id: "p5", cuo: "000197", issuedAt: "2026-09-06", dueAt: "2026-10-06", type: "01", number: "F001-002211", supplier: "Sonrisa Lab Prótesis S.A.C.", supplierRuc: "20601442907", base: 3200, igv: 576, total: 3776, detraction: { rate: 10, amount: 377.6, paid: true }, payStatus: "pendiente", sire: "aceptado", expenseClass: "Laboratorio dental" },
  { id: "p6", cuo: "000196", issuedAt: "2026-09-05", type: "07", number: "FC01-000031", supplier: "Dental Import S.A.C.", supplierRuc: "20487120933", base: -180, igv: -32.4, total: -212.4, payStatus: "pagado", sire: "aceptado", expenseClass: "Material dental" },
  { id: "p7", cuo: "000195", issuedAt: "2026-09-03", type: "03", number: "B012-004521", supplier: "Sodimac Perú S.A.", supplierRuc: "20389230724", base: 254.24, igv: 45.76, total: 300, payStatus: "pagado", sire: "excluido", expenseClass: "Mantenimiento" },
  { id: "p8", cuo: "000194", issuedAt: "2026-09-02", dueAt: "2026-09-13", type: "01", number: "F003-000118", supplier: "Corporación Andina de Salud S.A.C.", supplierRuc: "20605551234", base: 1800, igv: 324, total: 2124, payStatus: "por_vencer", sire: "aceptado", expenseClass: "Servicios profesionales" },
];
