export interface CashMovement {
  id: string;
  time: string;
  doc: string;
  concept: string;
  party: string;
  method: "efectivo" | "yape" | "plin" | "tarjeta" | "transferencia";
  kind: "ingreso" | "egreso";
  status: "valido" | "pendiente" | "anulado";
  amount: number;
}

export const shift = {
  site: "Sede Central – Miraflores", terminal: "Turno tarde · Caja 01", cashier: "Lucía Espinoza",
  openedAt: "08:30", openingAmount: 300, sales: 4850, salesCount: 24, expenses: 120, expensesCount: 3,
  expected: 5030,
  channels: [
    { id: "cash", label: "Efectivo en bóveda", theoretical: 1420, counted: 1420, detail: "Cuadrado" },
    { id: "qr", label: "Yape y Plin (QR)", theoretical: 1680, counted: 1680, detail: "12 QR OK · BCP Yape S/ 1,220 · Plin S/ 460" },
    { id: "card", label: "Tarjetas POS", theoretical: 1550, counted: 1550, detail: "Niubiz lote 0184 S/ 980 · Izipay lote 0042 S/ 570" },
    { id: "transfer", label: "Transferencias", theoretical: 380, counted: 380, detail: "BCP Cta. Cte. S/ 200 · Interbank S/ 180" },
  ],
  denominations: [
    { label: "S/ 200", value: 200, count: 2 }, { label: "S/ 100", value: 100, count: 5 }, { label: "S/ 50", value: 50, count: 5 },
    { label: "S/ 20", value: 20, count: 7 }, { label: "S/ 10", value: 10, count: 9 }, { label: "S/ 5", value: 5, count: 4 },
    { label: "S/ 2", value: 2, count: 3 }, { label: "S/ 1", value: 1, count: 10 }, { label: "S/ 0.50", value: 0.5, count: 8 },
  ],
};

export const movementsMock: CashMovement[] = [
  { id: "m1", time: "16:42", doc: "B001-004291", concept: "Curación resina", party: "Juan Carlos Pérez", method: "yape", kind: "ingreso", status: "valido", amount: 240 },
  { id: "m2", time: "16:10", doc: "B001-004290", concept: "Profilaxis", party: "Mariana Quispe", method: "efectivo", kind: "ingreso", status: "valido", amount: 85 },
  { id: "m3", time: "15:35", doc: "VALE-018", concept: "Compra de agua y útiles", party: "Bodega San Martín", method: "efectivo", kind: "egreso", status: "valido", amount: 45 },
  { id: "m4", time: "15:10", doc: "F001-000842", concept: "Servicios odontológicos", party: "Inversiones Alpamayo S.A.C.", method: "transferencia", kind: "ingreso", status: "pendiente", amount: 1280 },
  { id: "m5", time: "14:20", doc: "B001-004289", concept: "Consulta general", party: "Cliente genérico", method: "tarjeta", kind: "ingreso", status: "valido", amount: 60 },
  { id: "m6", time: "13:05", doc: "VALE-017", concept: "Movilidad mensajería", party: "R. Medina", method: "efectivo", kind: "egreso", status: "valido", amount: 25 },
  { id: "m7", time: "12:40", doc: "B001-004288", concept: "Blanqueamiento LED", party: "Ana Lucía Paredes", method: "tarjeta", kind: "ingreso", status: "valido", amount: 320 },
  { id: "m8", time: "11:15", doc: "VALE-016", concept: "Reparación menor compresor", party: "Servitec E.I.R.L.", method: "efectivo", kind: "egreso", status: "valido", amount: 50 },
  { id: "m9", time: "10:02", doc: "B001-004287", concept: "Radiografía", party: "Jorge Rojas", method: "plin", kind: "ingreso", status: "anulado", amount: 30 },
];
