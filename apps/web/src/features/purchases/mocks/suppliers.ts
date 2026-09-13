export interface Supplier {
  id: string;
  name: string;
  ruc: string;
  category: string;
  contact: string;
  phone: string;
  email: string;
  terms: string;
  yearTotal: number;
  pending: number;
  status: "activo" | "observado" | "inactivo";
  sunat: "habido" | "no_habido";
}

export const suppliersSummary = { active: 38, observed: 2, yearTotal: 312400, pendingOrders: 9 };

export const suppliersMock: Supplier[] = [
  { id: "s1", name: "Dental Import S.A.C.", ruc: "20487120933", category: "Material dental", contact: "Rosa Villanueva", phone: "+51 (01) 421-7700", email: "ventas@dentalimport.pe", terms: "Crédito 30 días", yearTotal: 84200, pending: 2537, status: "activo", sunat: "habido" },
  { id: "s2", name: "Importaciones Biomédicas E.I.R.L.", ruc: "20512389012", category: "Insumos clínicos", contact: "Luis Cárdenas", phone: "+51 987 001 002", email: "lcardenas@biomedicas.pe", terms: "Crédito 30 días · detracción 12%", yearTotal: 61800, pending: 4864.9, status: "observado", sunat: "habido" },
  { id: "s3", name: "Distribuidora Médica del Sur S.R.L.", ruc: "20489123019", category: "Medicamentos", contact: "Carla Quispe", phone: "+51 (01) 275-1180", email: "pedidos@dimsur.pe", terms: "Crédito 15 días", yearTotal: 45300, pending: 1628.4, status: "activo", sunat: "habido" },
  { id: "s4", name: "Sonrisa Lab Prótesis S.A.C.", ruc: "20601442907", category: "Laboratorio dental", contact: "Téc. Mario Paz", phone: "+51 955 222 333", email: "lab@sonrisalab.pe", terms: "Crédito 30 días · detracción 10%", yearTotal: 38900, pending: 3776, status: "activo", sunat: "habido" },
  { id: "s5", name: "Corporación Andina de Salud S.A.C.", ruc: "20605551234", category: "Servicios profesionales", contact: "Dra. Elena Ruiz", phone: "+51 (01) 640-2200", email: "admin@andinasalud.pe", terms: "Contado", yearTotal: 21600, pending: 2124, status: "activo", sunat: "habido" },
  { id: "s6", name: "Luz del Sur S.A.A.", ruc: "20331898008", category: "Servicios públicos", contact: "Atención empresas", phone: "617-5000", email: "empresas@luzdelsur.com.pe", terms: "Recibo mensual", yearTotal: 5400, pending: 0, status: "activo", sunat: "habido" },
  { id: "s7", name: "Servitec Dental E.I.R.L.", ruc: "20548812345", category: "Mantenimiento de equipos", contact: "Jorge Salas", phone: "+51 944 888 111", email: "servitec@gmail.com", terms: "Contado", yearTotal: 3200, pending: 0, status: "inactivo", sunat: "no_habido" },
];

export interface PurchaseOrder {
  id: string;
  number: string;
  supplier: string;
  issuedAt: string;
  expectedAt: string;
  total: number;
  detraction?: { rate: number; amount: number; deposited: boolean };
  status: "pendiente" | "parcial" | "recibida" | "cancelada";
}

export const purchaseOrdersMock: PurchaseOrder[] = [
  { id: "o1", number: "OC-2026-0091", supplier: "Dental Import S.A.C.", issuedAt: "2026-09-10", expectedAt: "2026-09-17", total: 6420, status: "pendiente" },
  { id: "o2", number: "OC-2026-0090", supplier: "Importaciones Biomédicas E.I.R.L.", issuedAt: "2026-09-08", expectedAt: "2026-09-15", total: 4864.9, detraction: { rate: 12, amount: 583.79, deposited: false }, status: "parcial" },
  { id: "o3", number: "OC-2026-0089", supplier: "Sonrisa Lab Prótesis S.A.C.", issuedAt: "2026-09-05", expectedAt: "2026-09-12", total: 3776, detraction: { rate: 10, amount: 377.6, deposited: true }, status: "recibida" },
  { id: "o4", number: "OC-2026-0088", supplier: "Distribuidora Médica del Sur S.R.L.", issuedAt: "2026-09-03", expectedAt: "2026-09-10", total: 1628.4, status: "recibida" },
  { id: "o5", number: "OC-2026-0087", supplier: "Servitec Dental E.I.R.L.", issuedAt: "2026-08-28", expectedAt: "2026-09-04", total: 850, status: "cancelada" },
];
