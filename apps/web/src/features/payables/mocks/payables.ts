export type PayableStatus = "programada" | "por_vencer" | "vencida" | "pagada";
export interface Payable { id: string; supplier: string; ruc: string; number: string; issuedAt: string; dueAt: string; total: number; detractionPending?: number; balance: number; status: PayableStatus }
export const payablesSummary = { total: 21400, week: 6420, overdue: 1628, paidMonth: 38900 };
export const payablesMock: Payable[] = [
  { id: "y1", supplier: "Importaciones Biomédicas E.I.R.L.", ruc: "20512389012", number: "F001-000429", issuedAt: "2026-09-11", dueAt: "2026-10-11", total: 4864.9, detractionPending: 583.79, balance: 4864.9, status: "programada" },
  { id: "y2", supplier: "Distribuidora Médica del Sur S.R.L.", ruc: "20489123019", number: "E001-000982", issuedAt: "2026-09-09", dueAt: "2026-09-14", total: 1628.4, balance: 1628.4, status: "por_vencer" },
  { id: "y3", supplier: "Sonrisa Lab Prótesis S.A.C.", ruc: "20601442907", number: "F001-002211", issuedAt: "2026-09-06", dueAt: "2026-10-06", total: 3776, balance: 3776, status: "programada" },
  { id: "y4", supplier: "Corporación Andina de Salud S.A.C.", ruc: "20605551234", number: "F003-000118", issuedAt: "2026-09-02", dueAt: "2026-09-13", total: 2124, balance: 2124, status: "por_vencer" },
  { id: "y5", supplier: "Dental Import S.A.C.", ruc: "20487120933", number: "F002-011820", issuedAt: "2026-08-10", dueAt: "2026-09-09", total: 2537, balance: 2537, status: "vencida" },
  { id: "y6", supplier: "Luz del Sur S.A.A.", ruc: "20331898008", number: "R001-8823110", issuedAt: "2026-09-08", dueAt: "2026-09-20", total: 486.75, balance: 486.75, status: "por_vencer" },
  { id: "y7", supplier: "Dental Import S.A.C.", ruc: "20487120933", number: "F002-011702", issuedAt: "2026-07-15", dueAt: "2026-08-14", total: 3180, balance: 0, status: "pagada" },
];
export interface Integration { id: string; name: string; kind: string; status: "conectado" | "atencion" | "desconectado"; last: string; detail: string }
export const integrations: Integration[] = [
  { id: "ose", name: "OSE Digiflow", kind: "Facturación electrónica", status: "conectado", last: "hace 2 min", detail: "CDR promedio 1.8 s · 99.98% uptime" },
  { id: "sire", name: "SUNAT SIRE (RVIE / RCE)", kind: "Libros electrónicos", status: "conectado", last: "hoy 08:35", detail: "Propuesta 2026-09 descargada" },
  { id: "reniec", name: "RENIEC / padrón RUC", kind: "Validación de identidad", status: "conectado", last: "hace 12 min", detail: "1,248 consultas este mes" },
  { id: "niubiz", name: "Niubiz", kind: "Pasarela de tarjetas", status: "conectado", last: "hoy 10:01", detail: "Lote 0184 conciliado" },
  { id: "izipay", name: "Izipay", kind: "Pasarela de tarjetas", status: "atencion", last: "ayer 19:20", detail: "Lote 0042 con 1 transacción sin conciliar" },
  { id: "yape", name: "Yape empresas", kind: "Billetera digital", status: "conectado", last: "hace 30 s", detail: "QR dinámico habilitado" },
  { id: "culqi", name: "Culqi", kind: "Pagos en línea", status: "desconectado", last: "—", detail: "Configura las llaves API para cobrar en el enlace de cotización" },
  { id: "wa", name: "WhatsApp Business API", kind: "Mensajería", status: "conectado", last: "hace 1 min", detail: "6 plantillas aprobadas · 124 envíos hoy" },
  { id: "gcal", name: "Google Calendar", kind: "Agenda", status: "desconectado", last: "—", detail: "Sincroniza la agenda de cada profesional" },
  { id: "concar", name: "Concar / Siscont", kind: "Contabilidad", status: "conectado", last: "01/09/2026", detail: "Exportación mensual de asientos" },
];
