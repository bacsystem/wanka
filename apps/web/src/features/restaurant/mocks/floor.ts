export type TableStatus = "libre" | "ocupada" | "por_cobrar" | "reservada" | "sucia";
export interface DiningTable { id: string; label: string; area: string; seats: number; status: TableStatus; since?: string; amount?: number; waiter?: string; note?: string; items?: number }
export interface OrderItem { id: string; qty: number; name: string; detail?: string; price: number; station: "Frío" | "Caliente" | "Bar" | "Entrada"; status: "en_cocina" | "listo" | "servido" }
export interface KdsTicket { id: string; table: string; waiter: string; minutes: number; lane: "pendiente" | "preparacion" | "listo"; items: { qty: number; name: string; station: string; note?: string }[] }

export const floorSummary = { shift: "Turno almuerzo · 12:00–17:00", occupied: 9, total: 22, avgTicket: 68, kitchenTickets: 6, critical: 1, avgMinutes: 38 };
export const areas = [{ id: "salon", label: "Salón principal", count: 12 }, { id: "terraza", label: "Terraza", count: 6 }, { id: "barra", label: "Barra pisco y ceviche", count: 4 }];

export const tablesMock: DiningTable[] = [
  { id: "M1", label: "M1", area: "salon", seats: 2, status: "libre" },
  { id: "M2", label: "M2", area: "salon", seats: 4, status: "ocupada", since: "00:24", amount: 142, waiter: "Juan" },
  { id: "M3", label: "M3", area: "salon", seats: 6, status: "ocupada", since: "00:55", amount: 285.5, waiter: "María" },
  { id: "M4", label: "M4", area: "salon", seats: 4, status: "por_cobrar", amount: 194, note: "Pre-cuenta lista" },
  { id: "M5", label: "M5", area: "salon", seats: 2, status: "libre" },
  { id: "M6", label: "M6", area: "salon", seats: 4, status: "reservada", note: "14:00 · Dr. Mendoza (4) · confirmada WA" },
  { id: "M7", label: "M7", area: "salon", seats: 4, status: "ocupada", since: "00:42", amount: 178, waiter: "Luis", items: 3 },
  { id: "M8", label: "M8", area: "salon", seats: 2, status: "ocupada", since: "00:15", amount: 86, waiter: "Juan" },
  { id: "M9", label: "M9", area: "salon", seats: 4, status: "sucia", note: "Por limpiar" },
  { id: "M10", label: "M10", area: "salon", seats: 2, status: "libre" },
  { id: "M11", label: "M11", area: "salon", seats: 6, status: "ocupada", since: "00:36", amount: 235, waiter: "Carla" },
  { id: "M12", label: "M12", area: "salon", seats: 6, status: "reservada", note: "20:30 · Familia Wong (6) · adelanto S/ 100" },
  { id: "T1", label: "T1", area: "terraza", seats: 4, status: "ocupada", since: "00:20", amount: 96, waiter: "Luis" },
  { id: "T2", label: "T2", area: "terraza", seats: 4, status: "libre" },
  { id: "T3", label: "T3", area: "terraza", seats: 2, status: "por_cobrar", amount: 64 },
  { id: "T4", label: "T4", area: "terraza", seats: 6, status: "ocupada", since: "01:02", amount: 312, waiter: "María" },
  { id: "T5", label: "T5", area: "terraza", seats: 2, status: "libre" },
  { id: "T6", label: "T6", area: "terraza", seats: 4, status: "libre" },
  { id: "B1", label: "B1", area: "barra", seats: 1, status: "ocupada", since: "00:10", amount: 38, waiter: "Bar" },
  { id: "B2", label: "B2", area: "barra", seats: 1, status: "libre" },
  { id: "B3", label: "B3", area: "barra", seats: 1, status: "libre" },
  { id: "B4", label: "B4", area: "barra", seats: 1, status: "ocupada", since: "00:31", amount: 54, waiter: "Bar" },
];

export const orderM7: OrderItem[] = [
  { id: "i1", qty: 2, name: "Ceviche clásico de pesca del día", detail: "Corvina · picante medio · con canchita y choclo", price: 38, station: "Frío", status: "listo" },
  { id: "i2", qty: 3, name: "Chicha morada (vaso)", price: 8, station: "Bar", status: "servido" },
  { id: "i3", qty: 1, name: "Arroz con mariscos", detail: "Término normal", price: 46, station: "Caliente", status: "en_cocina" },
  { id: "i4", qty: 1, name: "Causa rellena de cangrejo", price: 32, station: "Entrada", status: "servido" },
];

export const menu = {
  Entradas: [{ name: "Causa rellena de cangrejo", price: 32 }, { name: "Tiradito al ají amarillo", price: 36 }, { name: "Choritos a la chalaca", price: 28 }],
  Fondos: [{ name: "Ceviche clásico de pesca del día", price: 38 }, { name: "Arroz con mariscos", price: 46 }, { name: "Parihuela especial", price: 58 }, { name: "Chicharrón mixto familiar", price: 72 }],
  Bebidas: [{ name: "Chicha morada (vaso)", price: 8 }, { name: "Chilcano clásico", price: 22 }, { name: "Cerveza Cusqueña trigo", price: 14 }, { name: "Jarra chicha morada 1 L", price: 24 }],
  Postres: [{ name: "Suspiro a la limeña", price: 16 }, { name: "Picarones (6)", price: 18 }],
};

export const kdsTickets: KdsTicket[] = [
  { id: "#1042", table: "Mesa 8", waiter: "Juan", minutes: 4, lane: "pendiente", items: [{ qty: 1, name: "Tiradito al ají amarillo", station: "Frío" }, { qty: 2, name: "Chilcano clásico", station: "Bar" }] },
  { id: "#1043", table: "Mesa 11", waiter: "Carla", minutes: 2, lane: "pendiente", items: [{ qty: 1, name: "Causa rellena de cangrejo", station: "Entrada" }] },
  { id: "#1040", table: "Mesa 7", waiter: "Luis", minutes: 9, lane: "preparacion", items: [{ qty: 1, name: "Arroz con mariscos", station: "Caliente", note: "En sartén" }, { qty: 2, name: "Ceviche clásico", station: "Frío", note: "Listos para pase" }] },
  { id: "#1039", table: "Mesa 3", waiter: "María", minutes: 18, lane: "preparacion", items: [{ qty: 1, name: "Parihuela especial", station: "Caliente", note: "Retrasado" }, { qty: 1, name: "Chicharrón mixto familiar", station: "Caliente", note: "En freidora" }] },
  { id: "#1038", table: "Mesa 2", waiter: "Juan", minutes: 1, lane: "listo", items: [{ qty: 2, name: "Chilcano de maracuyá", station: "Bar" }, { qty: 1, name: "Cerveza Cusqueña trigo", station: "Bar" }] },
  { id: "#1037", table: "Barra", waiter: "Bar", minutes: 3, lane: "listo", items: [{ qty: 3, name: "Jarra chicha morada 1 L", station: "Bar" }] },
];
