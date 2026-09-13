export type DeliveryLane = "nuevo" | "cocina" | "listo" | "ruta" | "entregado";
export interface DeliveryOrder { id: string; code: string; customer: string; phone: string; address: string; reference: string; channel: "WhatsApp" | "Rappi" | "PedidosYa" | "Web"; items: { qty: number; name: string }[]; total: number; payment: "Yape" | "Tarjeta" | "Contra entrega" | "Pagado en app"; rider?: string; minutes: number; lane: DeliveryLane }
export interface Rider { id: string; name: string; status: "en_ruta" | "disponible" | "descanso"; orders: string[]; vehicle: string }
export interface Reservation { id: string; time: string; name: string; people: number; table?: string; phone: string; status: "confirmada" | "pendiente" | "no_show" | "sentada"; deposit?: number; area: string }

export const deliverySummary = { active: 4, avgMinutes: 32, ridersOnRoute: 2, ridersTotal: 3, shiftSales: 640 };
export const deliveryOrders: DeliveryOrder[] = [
  { id: "d1", code: "DLV-0412", customer: "Ana Lucía Paredes", phone: "+51 955 444 333", address: "Av. Benavides 1250, Dpto. 802, Miraflores", reference: "Edificio Torre Azul · portero", channel: "WhatsApp", items: [{ qty: 2, name: "Ceviche clásico" }, { qty: 1, name: "Chicha morada 1 L" }], total: 100, payment: "Yape", minutes: 6, lane: "nuevo" },
  { id: "d2", code: "DLV-0411", customer: "Jorge Rojas", phone: "+51 999 888 777", address: "Calle Schell 320, Miraflores", reference: "Oficina 301", channel: "Rappi", items: [{ qty: 1, name: "Arroz con mariscos" }, { qty: 1, name: "Causa de cangrejo" }], total: 78, payment: "Pagado en app", minutes: 14, lane: "cocina" },
  { id: "d3", code: "DLV-0410", customer: "Carlos Benavides", phone: "+51 944 555 666", address: "Av. Larco 1301, Miraflores", reference: "Casa con reja verde", channel: "Web", items: [{ qty: 1, name: "Parihuela especial" }, { qty: 2, name: "Chilcano clásico" }], total: 102, payment: "Tarjeta", rider: "Miguel", minutes: 21, lane: "listo" },
  { id: "d4", code: "DLV-0409", customer: "María Castillo", phone: "+51 987 111 222", address: "Av. 28 de Julio 850, Dpto. 402", reference: "Tocar intercomunicador 402", channel: "PedidosYa", items: [{ qty: 1, name: "Chicharrón mixto familiar" }], total: 72, payment: "Pagado en app", rider: "Rosa", minutes: 27, lane: "ruta" },
  { id: "d5", code: "DLV-0408", customer: "Renzo Valdivia", phone: "+51 944 000 111", address: "Jr. Bolognesi 402, Miraflores", reference: "—", channel: "WhatsApp", items: [{ qty: 2, name: "Tiradito al ají amarillo" }], total: 72, payment: "Contra entrega", rider: "Miguel", minutes: 41, lane: "entregado" },
];
export const riders: Rider[] = [
  { id: "r1", name: "Miguel Quispe", status: "en_ruta", orders: ["DLV-0410"], vehicle: "Moto · B7S-911" },
  { id: "r2", name: "Rosa Huamán", status: "en_ruta", orders: ["DLV-0409"], vehicle: "Bicicleta eléctrica" },
  { id: "r3", name: "Pedro Salas", status: "disponible", orders: [], vehicle: "Moto · A2K-330" },
];
export const reservations: Reservation[] = [
  { id: "rs1", time: "13:00", name: "Familia Torres", people: 5, table: "M3", phone: "+51 977 100 200", status: "sentada", area: "Salón principal" },
  { id: "rs2", time: "14:00", name: "Dr. Mendoza", people: 4, table: "M6", phone: "+51 987 654 321", status: "confirmada", area: "Salón principal" },
  { id: "rs3", time: "14:30", name: "Lucía Fernández", people: 2, phone: "+51 955 222 333", status: "pendiente", area: "Terraza" },
  { id: "rs4", time: "20:30", name: "Familia Wong", people: 6, table: "M12", phone: "+51 966 000 111", status: "confirmada", deposit: 100, area: "Salón principal" },
  { id: "rs5", time: "21:00", name: "Empresa Andina S.A.C.", people: 10, phone: "+51 (01) 442-8990", status: "pendiente", area: "Terraza" },
  { id: "rs6", time: "12:30", name: "Sr. Paredes", people: 2, phone: "+51 944 888 111", status: "no_show", area: "Barra" },
];
