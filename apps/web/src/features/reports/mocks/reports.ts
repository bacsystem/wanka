export interface SalesRegisterRow {
  cuo: string;
  issuedAt: string;
  type: "01" | "03" | "07";
  number: string;
  docType: "RUC" | "DNI" | "—";
  docNumber: string;
  customer: string;
  base: number;
  igv: number;
  exempt: number;
  total: number;
  sire: "coincidente" | "observado" | "faltante";
}

export const reportsSummary = {
  ruc: "20608941235", period: "2026-09",
  taxableSales: 36313.56, docs: 148, fiscalDebit: 6536.44, fiscalCredit: 2418.1, igvToPay: 4118.34, kardex: 84230.5,
  schedule: [{ label: "Vencimiento PDT 621 (dígito 5)", date: "2026-10-16" }, { label: "Envío RVIE / RCE SIRE", date: "2026-10-14" }, { label: "Libros electrónicos PLE", date: "2026-10-20" }],
};

export const reportTypes = [
  { id: "rvie", label: "Registro de ventas (RVIE – SIRE)", count: "148 regs" },
  { id: "kardex", label: "Kardex valorizado SUNAT", count: "3 almacenes" },
  { id: "f621", label: "Liquidación IGV-Renta (F-621)", count: "2026-09" },
  { id: "export", label: "Exportaciones contables (Concar / Siscont)", count: "3 formatos" },
];

export const salesRegisterMock: SalesRegisterRow[] = [
  { cuo: "000142", issuedAt: "2026-09-12", type: "01", number: "F001-000842", docType: "RUC", docNumber: "20554182910", customer: "Inversiones Alpamayo S.A.C.", base: 1084.75, igv: 195.25, exempt: 0, total: 1280, sire: "coincidente" },
  { cuo: "000143", issuedAt: "2026-09-12", type: "03", number: "B001-004291", docType: "DNI", docNumber: "45892147", customer: "Juan Carlos Pérez Huamán", base: 203.39, igv: 36.61, exempt: 0, total: 240, sire: "coincidente" },
  { cuo: "000144", issuedAt: "2026-09-12", type: "03", number: "B001-004290", docType: "DNI", docNumber: "72109843", customer: "Mariana Quispe Flores", base: 72.03, igv: 12.97, exempt: 0, total: 85, sire: "coincidente" },
  { cuo: "000145", issuedAt: "2026-09-11", type: "01", number: "F001-000841", docType: "RUC", docNumber: "20489123019", customer: "Distribuidora Médica del Sur S.R.L.", base: 2923.73, igv: 526.27, exempt: 0, total: 3450, sire: "coincidente" },
  { cuo: "000146", issuedAt: "2026-09-11", type: "07", number: "FC01-000104", docType: "RUC", docNumber: "20554182910", customer: "Inversiones Alpamayo S.A.C.", base: -152.54, igv: -27.46, exempt: 0, total: -180, sire: "observado" },
  { cuo: "000147", issuedAt: "2026-09-11", type: "03", number: "B001-004289", docType: "—", docNumber: "—", customer: "Cliente genérico", base: 50.85, igv: 9.15, exempt: 0, total: 60, sire: "coincidente" },
  { cuo: "000148", issuedAt: "2026-09-10", type: "01", number: "F001-000840", docType: "RUC", docNumber: "20601284759", customer: "Transportes & Logística Andina S.A.", base: 4644.07, igv: 835.93, exempt: 0, total: 5480, sire: "coincidente" },
  { cuo: "000149", issuedAt: "2026-09-10", type: "03", number: "B001-004288", docType: "DNI", docNumber: "41234567", customer: "Ana Lucía Paredes Vega", base: 271.19, igv: 48.81, exempt: 8, total: 328, sire: "faltante" },
];
