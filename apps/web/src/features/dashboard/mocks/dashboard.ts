import type { DailySales, DashboardData } from "@/types/domain";

function buildSales30d(): DailySales[] {
  const base = new Date("2026-09-12T00:00:00");
  const pattern = [
    3120, 2890, 3410, 4020, 4890, 5210, 2310, 3050, 2980, 3620, 4110, 4730,
    5480, 2450, 3210, 3090, 3780, 4260, 4950, 5620, 2600, 3340, 3180, 3900,
    4390, 5080, 5810, 2720, 3460, 4892.5,
  ];
  return pattern.map((total, i) => {
    const d = new Date(base);
    d.setDate(base.getDate() - (pattern.length - 1 - i));
    return { date: d.toISOString().slice(0, 10), total };
  });
}

export const dashboardMock: DashboardData = {
  tenant: {
    id: "t_001",
    name: "Clínica Dental Sonrisa",
    ruc: "20601234567",
    industry: "odontologia",
  },
  kpis: [
    { id: "sales_today", label: "Ventas de hoy", value: 4892.5, format: "currency", deltaPercent: 14.2 },
    { id: "docs_today", label: "Comprobantes hoy", value: 38, format: "integer", deltaPercent: 5.6 },
    { id: "receivable", label: "Por cobrar", value: 2450.8, format: "currency", deltaPercent: -8.5 },
    { id: "low_stock", label: "Stock bajo", value: 5, format: "integer", deltaPercent: null, hint: "2 en desabastecimiento crítico" },
  ],
  sales30d: buildSales30d(),
  recentDocuments: [
    { id: "d1", type: "factura", series: "F001", number: 4582, customerName: "Inversiones Pardo S.A.C.", customerDocument: "20512345678", total: 1280.0, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T16:42:00" },
    { id: "d2", type: "boleta", series: "B001", number: 12903, customerName: "Juan Pérez Ramos", customerDocument: "45678912", total: 145.5, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T16:10:00" },
    { id: "d3", type: "factura", series: "F001", number: 4581, customerName: "Constructora & Minas del Sur E.I.R.L.", customerDocument: "20487654321", total: 3420.0, currency: "PEN", sunatStatus: "pendiente", issuedAt: "2026-09-12T15:35:00" },
    { id: "d4", type: "boleta", series: "B001", number: 12902, customerName: "María Elena Chávez Ríos", customerDocument: "72345678", total: 89.0, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T15:02:00" },
    { id: "d5", type: "nota_credito", series: "FC01", number: 118, customerName: "Inversiones Pardo S.A.C.", customerDocument: "20512345678", total: 180.0, currency: "PEN", sunatStatus: "rechazado", issuedAt: "2026-09-12T14:20:00" },
    { id: "d6", type: "boleta", series: "B001", number: 12901, customerName: "Cliente genérico", customerDocument: "—", total: 60.0, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T13:48:00" },
  ],
  stockAlerts: [
    { id: "s1", productName: "Kit resina compuesta 3M", warehouse: "Almacén principal · Bodega Odonto B", stock: 2, minimum: 10, unit: "un" },
    { id: "s2", productName: "Guantes quirúrgicos látex M", warehouse: "Almacén 2 · Box Esterilización 01", stock: 4, minimum: 15, unit: "cajas" },
    { id: "s3", productName: "Anestesia lidocaína 2% c/epinefrina", warehouse: "Farmacia clínica · Refrigerado", stock: 3, minimum: 12, unit: "amp" },
    { id: "s4", productName: "Mascarillas KN95 x 20", warehouse: "Consultorio 2", stock: 0, minimum: 15, unit: "cajas" },
    { id: "s5", productName: "Hilo retractor #00", warehouse: "Almacén principal", stock: 0, minimum: 3, unit: "un" },
  ],
  sunatQueue: [
    { id: "q1", documentLabel: "F001-00004581", status: "pendiente", message: "En cola de envío" },
    { id: "q2", documentLabel: "FC01-00000118", status: "rechazado", message: "Error 2335: fecha de emisión fuera de plazo" },
  ],
};

/** Shape of the future API call; returns mock data for now. */
export async function getDashboardData(): Promise<DashboardData> {
  return dashboardMock;
}
