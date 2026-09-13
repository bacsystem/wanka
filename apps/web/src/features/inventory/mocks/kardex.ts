export type KardexOp = "inicial" | "entrada" | "salida";

export interface KardexRow {
  id: string;
  date: string;
  cpeType: string; // Tabla 10
  number: string;
  party: string;
  opType: string; // Tabla 12
  kind: KardexOp;
  qty: number; // signed; 0 for opening
  unitCost?: number; // entradas
  total: number; // absolute
  balanceQty: number;
  avgCost: number;
  balanceValue: number;
  flag?: string; // lot / CDR
}

export const kardexItem = {
  name: "Resina compuesta 3M Filtek Z350 XT – jeringa 4 g",
  code: "PROD-0042",
  unspsc: "42151605",
  sunatType: "SUNAT tipo 01 (mercaderías)",
  unit: "NIU (unidades)",
  method: "Promedio ponderado móvil",
  period: "Setiembre 2026 (abierto)",
  warehouse: "0000 – Miraflores (principal)",
  opening: { qty: 45, unitCost: 115, value: 5175 },
  entries: { qty: 120, docs: 2, value: 13440 },
  exits: { qty: 110, detail: "2 consumos · 1 merma · 1 traslado · 1 venta", value: 12398.11 },
  closing: { qty: 55, avgCost: 113.0343, value: 6216.89 },
  pleFile: "LE2060894123520260900130100001111.txt",
};

export const kardexRows: KardexRow[] = [
  { id: "k0", date: "2026-09-01", cpeType: "00", number: "INVENTARIO-INIC", party: "Saldo inicial del período", opType: "16 Saldo inicial", kind: "inicial", qty: 0, total: 5175, balanceQty: 45, avgCost: 115, balanceValue: 5175, flag: "Cierre 08/2026" },
  { id: "k1", date: "2026-09-05", cpeType: "01", number: "F001-08492", party: "Dentsply Sirona S.A.C.", opType: "02 Compra nacional", kind: "entrada", qty: 60, unitCost: 110, total: 6600, balanceQty: 105, avgCost: 112.1429, balanceValue: 11775, flag: "LT-77180 · CDR OK" },
  { id: "k2", date: "2026-09-08", cpeType: "00", number: "CNS-2026-082", party: "Box quirúrgico 2", opType: "10 Consumo clínico", kind: "salida", qty: -15, total: 1682.14, balanceQty: 90, avgCost: 112.1429, balanceValue: 10092.86 },
  { id: "k3", date: "2026-09-12", cpeType: "01", number: "F001-000842", party: "Inversiones Alpamayo S.A.C.", opType: "01 Venta", kind: "salida", qty: -25, total: 2803.57, balanceQty: 65, avgCost: 112.1429, balanceValue: 7289.29, flag: "CDR aceptado" },
  { id: "k4", date: "2026-09-18", cpeType: "01", number: "E001-003810", party: "Dental Medical Perú E.I.R.L.", opType: "02 Compra nacional", kind: "entrada", qty: 60, unitCost: 114, total: 6840, balanceQty: 125, avgCost: 113.0343, balanceValue: 14129.29, flag: "LT-80211 · CDR OK" },
  { id: "k5", date: "2026-09-22", cpeType: "09", number: "GRE-T001-0084", party: "Guía remitente SUNAT → San Isidro", opType: "16 Traslado establec.", kind: "salida", qty: -30, total: 3391.03, balanceQty: 95, avgCost: 113.0343, balanceValue: 10738.26 },
  { id: "k6", date: "2026-09-27", cpeType: "00", number: "ACTA-MERM-10", party: "Destrucción notarial", opType: "11 Mermas / desmedro", kind: "salida", qty: -5, total: 565.17, balanceQty: 90, avgCost: 113.0343, balanceValue: 10173.09, flag: "Acta #14" },
  { id: "k7", date: "2026-09-30", cpeType: "00", number: "CNS-2026-104", party: "Endodoncia / estética", opType: "10 Consumo clínico", kind: "salida", qty: -35, total: 3956.2, balanceQty: 55, avgCost: 113.0343, balanceValue: 6216.89 },
];
