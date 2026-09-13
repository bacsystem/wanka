export type MovementKind = "entrada" | "salida" | "traslado" | "merma" | "ajuste";

export interface InventoryMovement {
  id: string;
  folio: string;
  at: string; // ISO datetime
  kind: MovementKind;
  sunatCode: string; // Tabla 12
  sunatLabel: string;
  reference: string;
  warehouse: string;
  location: string;
  itemName: string;
  sku: string;
  presentation: string;
  lot?: string;
  expires?: string;
  quantity: number; // signed
  unit: string;
  balance?: string;
  unitCost: number;
  responsible: string;
  role: string;
}

export const movementsSummary = {
  entries: 142, entriesDelta: 12, entriesValue: 28450,
  exits: 318, exitsProcedures: 98, exitsValue: 19820,
  losses: 5, lossesValue: 480,
  inTransit: 1, transitGuide: "GRE-000492",
  total: 465,
};

export const sunatTable12 = [
  { code: "01", label: "01 – Compra nacional (factura / guía)" },
  { code: "02", label: "02 – Venta / salida por tratamiento" },
  { code: "11", label: "11 – Merma / deterioro / caducidad" },
  { code: "21", label: "21 – Traslado entre almacenes" },
  { code: "99", label: "99 – Ajuste de inventario físico" },
];

export const movementsMock: InventoryMovement[] = [
  { id: "m1", folio: "MOV-2026-001290", at: "2026-09-12T10:30:00", kind: "entrada", sunatCode: "01", sunatLabel: "Compra nacional", reference: "Fact. F001-4921 (DentiPerú S.A.C.)", warehouse: "Alm. Central Miraflores", location: "Estante A-04", itemName: "Lidocaína 2% c/epinefrina", sku: "MED-0441", presentation: "Fco 50 carpules", lot: "LT-98214", expires: "11/2027", quantity: 50, unit: "cajas", balance: "Stock saldo: 140", unitCost: 78.5, responsible: "Lic. Patricia Vega", role: "Farmacia y almacén" },
  { id: "m2", folio: "MOV-2026-001289", at: "2026-09-12T09:15:00", kind: "salida", sunatCode: "02", sunatLabel: "Consumo clínico", reference: "Tratamiento Curación #892 (Box 2)", warehouse: "Box operatorio 02", location: "Desde Alm. Central", itemName: "Resina Filtek Z350 XT Body A2", sku: "ODO-1102", presentation: "Jeringa 4 g", lot: "LT-77180", expires: "04/2027", quantity: -2, unit: "unids", balance: "Stock saldo: 14", unitCost: 115, responsible: "Dra. Claudia Ramos", role: "Odontóloga COP 45112" },
  { id: "m3", folio: "MOV-2026-001288", at: "2026-09-11T16:40:00", kind: "traslado", sunatCode: "21", sunatLabel: "Traslado e/ almacenes", reference: "Guía rem. GRE-000492 SUNAT", warehouse: "San Isidro → Miraflores", location: "En tránsito (vehículo B7S-911)", itemName: "Brackets metálicos Roth 0.22", sku: "INS-0102", presentation: "Kit 20 piezas", lot: "LT-24018", quantity: 15, unit: "kits", balance: "Por recepcionar", unitCost: 65, responsible: "Juan Carlos M.", role: "Logística San Isidro" },
  { id: "m4", folio: "MOV-2026-001287", at: "2026-09-10T18:10:00", kind: "merma", sunatCode: "11", sunatLabel: "Merma por caducidad", reference: "Acta destrucción interna #14", warehouse: "Farmacia clínica", location: "Refrigerador 01", itemName: "Mepivacaína 3% s/vaso", sku: "MED-0199", presentation: "Caja 50 cartuchos", lot: "LT-51002", expires: "08/2026", quantity: -1, unit: "caja", balance: "Stock saldo: 8", unitCost: 92, responsible: "Dr. Miguel Mendoza", role: "Director médico" },
  { id: "m5", folio: "MOV-2026-001286", at: "2026-09-09T20:00:00", kind: "ajuste", sunatCode: "99", sunatLabel: "Ajuste de inventario", reference: "Toma trimestral de stock físico", warehouse: "Alm. Central Miraflores", location: "Gaveta B-02", itemName: "Agujas dentales descartables 27G", sku: "MED-0034", presentation: "Caja 100 u", lot: "LT-88192", expires: "08/2028", quantity: 3, unit: "cajas", balance: "Sobrante de conteo", unitCost: 24, responsible: "Auditoría clínica", role: "Inventario" },
  { id: "m6", folio: "MOV-2026-001285", at: "2026-09-09T11:20:00", kind: "entrada", sunatCode: "01", sunatLabel: "Compra nacional", reference: "Fact. F002-011820 (Dental Import)", warehouse: "Alm. Central Miraflores", location: "Estante C-01", itemName: "Guantes de nitrilo talla M", sku: "INS-0210", presentation: "Caja x 100", lot: "LT-2025-01", expires: "01/2029", quantity: 20, unit: "cajas", balance: "Stock saldo: 23", unitCost: 38, responsible: "Lic. Patricia Vega", role: "Farmacia y almacén" },
  { id: "m7", folio: "MOV-2026-001284", at: "2026-09-08T15:05:00", kind: "salida", sunatCode: "02", sunatLabel: "Consumo clínico", reference: "Profilaxis #880 (Sillón 3)", warehouse: "Box higiene", location: "Desde Farmacia clínica", itemName: "Clorhexidina 0.12% 500 ml", sku: "MED-0120", presentation: "Frasco", lot: "LT-4410", expires: "08/2027", quantity: -1, unit: "fco", balance: "Stock saldo: 25", unitCost: 21, responsible: "Higienista R. Cano", role: "Higiene y profilaxis" },
];
