export type TenantStatus = "activa" | "trial" | "suspendida" | "cuota";
export type Vertical = "odontologia" | "veterinaria" | "restaurante" | "retail";
export type OseProvider = "Bizlinks OSE" | "Nubefact" | "Efact OSE" | "SUNAT Directo";

export interface PlatformTenant {
  id: string; code: string; name: string; ruc: string; city: string; vertical: Vertical;
  plan: string; planPrice: number; status: TenantStatus; sites: number; users: number; sitesNote: string;
  cpeUsed: number; cpeLimit: number; cpeNote: string; ose: OseProvider; oseLatencyMs: number | null; oseNote?: string; lastAccess: string; tag?: string;
}

export const verticalLabel: Record<Vertical, string> = { odontologia: "Odontología", veterinaria: "Veterinaria", restaurante: "Restaurante", retail: "Retail" };
export const tenantStatusLabel: Record<TenantStatus, string> = { activa: "Activa", trial: "Trial", suspendida: "Suspendida", cuota: "Cuota límite" };

export const platformSummary = { mrr: 486520, mrrDelta: 12.4, arr: 5838240, active: 1482, activeDelta: 38, capacity: 62, newMonth: 64, newDelta: 18.5, newGoal: 55, trialConversion: 28.4, churn: 1.18, churnDelta: -0.32, churnCount: 17, cpeToday: 342890, cdrRate: 99.7, peakPerSecond: 142, trial: 64, trialExpiring: 14, suspended: 9 };

export const platformTenants: PlatformTenant[] = [
  { id: "TEN-104", code: "OD", name: "Clínica Dental OdontoSalud S.A.C.", ruc: "20601984211", city: "Miraflores, Lima", vertical: "odontologia", plan: "Plan Pro Multi-Sede", planPrice: 490, status: "activa", sites: 3, users: 12, sitesNote: "Matriz: José Pardo", cpeUsed: 4820, cpeLimit: 10000, cpeNote: "Facturas, boletas y NC", ose: "Bizlinks OSE", oseLatencyMs: 180, lastAccess: "2026-09-12T15:38:00" },
  { id: "TEN-221", code: "VT", name: "Hospital Veterinario PetMed Sur E.I.R.L.", ruc: "20554819230", city: "Cayma, Arequipa", vertical: "veterinaria", plan: "Plan Clínico Base", planPrice: 320, status: "cuota", sites: 2, users: 8, sitesNote: "Sede Cayma y Yanahuara", cpeUsed: 4710, cpeLimit: 5000, cpeNote: "Requiere upgrade a Plan Pro", ose: "Nubefact", oseLatencyMs: 210, lastAccess: "2026-09-12T15:41:00", tag: "Alta carga" },
  { id: "TEN-058", code: "RS", name: "Grupo Gastronómico Cevicherías La Mar S.A.", ruc: "20498112349", city: "6 locales, Lima", vertical: "restaurante", plan: "Plan Unlimited POS", planPrice: 850, status: "activa", sites: 6, users: 45, sitesNote: "Alta concurrencia fin de semana", cpeUsed: 32490, cpeLimit: 50000, cpeNote: "Boletas electrónicas rápidas", ose: "Efact OSE", oseLatencyMs: 245, lastAccess: "2026-09-12T15:42:00", tag: "Enterprise" },
  { id: "TEN-377", code: "RT", name: "Comercializadora Textil Gamarra Perú S.A.C.", ruc: "20114092812", city: "La Victoria, Lima", vertical: "retail", plan: "Plan Básico", planPrice: 280, status: "suspendida", sites: 1, users: 4, sitesNote: "Acceso bloqueado", cpeUsed: 1120, cpeLimit: 3000, cpeNote: "Timbrado pausado por morosidad", ose: "SUNAT Directo", oseLatencyMs: null, oseNote: "Token OAuth vencido", lastAccess: "2026-09-06T09:12:00", tag: "Mora 32 días" },
  { id: "TEN-298", code: "DN", name: "Dental Norte Especialistas Trujillo", ruc: "20481902410", city: "Trujillo, La Libertad", vertical: "odontologia", plan: "Demo Plan Pro", planPrice: 0, status: "trial", sites: 1, users: 3, sitesNote: "Sede El Golf", cpeUsed: 140, cpeLimit: 500, cpeNote: "Cuota de prueba asignada", ose: "Bizlinks OSE", oseLatencyMs: 175, lastAccess: "2026-09-12T14:15:00", tag: "Trial 7 días" },
  { id: "TEN-109", code: "VA", name: "Cadena Veterinaria Los Andes Cusco S.R.L.", ruc: "20603349182", city: "Wanchaq, Cusco", vertical: "veterinaria", plan: "Plan Pro Multi-Sede", planPrice: 490, status: "activa", sites: 4, users: 16, sitesNote: "Cusco y Valle Sagrado", cpeUsed: 6120, cpeLimit: 10000, cpeNote: "Consumo estable", ose: "Nubefact", oseLatencyMs: 195, lastAccess: "2026-09-11T19:22:00" },
  { id: "TEN-412", code: "BD", name: "Bodega Don Pepe", ruc: "10456789012", city: "San Juan de Lurigancho, Lima", vertical: "retail", plan: "Plan Básico", planPrice: 180, status: "activa", sites: 1, users: 2, sitesNote: "Minimarket", cpeUsed: 860, cpeLimit: 3000, cpeNote: "Boletas POS", ose: "SUNAT Directo", oseLatencyMs: 410, lastAccess: "2026-09-12T12:05:00" },
  { id: "TEN-233", code: "CM", name: "Cevichería El Muelle S.A.C.", ruc: "20512345670", city: "Miraflores, Lima", vertical: "restaurante", plan: "Plan Pro Multi-Sede", planPrice: 490, status: "activa", sites: 1, users: 9, sitesNote: "Salón + delivery", cpeUsed: 3980, cpeLimit: 10000, cpeNote: "Comandas y boletas", ose: "Bizlinks OSE", oseLatencyMs: 168, lastAccess: "2026-09-12T13:10:00" },
];

export const oseProviders = [
  { name: "SUNAT Directo", status: "warning" as const, label: "Demorado", latencyMs: 1420, availability: 98.2, load: "Fallos: 0.8%" },
  { name: "Bizlinks OSE", status: "ok" as const, label: "Óptimo", latencyMs: 185, availability: 99.99, load: "Carga: 41%" },
  { name: "Nubefact", status: "ok" as const, label: "Estable", latencyMs: 210, availability: 99.95, load: "Carga: 38%" },
  { name: "Efact OSE", status: "ok" as const, label: "Estable", latencyMs: 245, availability: 99.89, load: "Carga: 26%" },
];

export const incidents = [
  { id: "INC-8421", priority: "alta" as const, title: "Latencia anómala en endpoint CDR SUNAT", text: "Colas de reintento superaron 4,200 comprobantes pendientes de confirmación. Rebalanceo automático a OSE Bizlinks activo para 240 tenants.", affects: "318 empresas", openedAt: "2026-09-12T13:10:00" },
  { id: "INC-8419", priority: "bloqueante" as const, title: "Certificado digital tributario expirado", text: "Tenant VET-SAN-MARTIN con certificado de firma caducado. Comprobantes retenidos en cola de contingencia local.", affects: "1 tenant", openedAt: "2026-09-12T10:45:00" },
  { id: "INC-8408", priority: "moderada" as const, title: "Límite de tasa en envío de PDF por WhatsApp", text: "Meta Cloud API aplicó throttling de mensajes por hora a restaurantes de alta rotación en Miraflores y San Isidro. Se amplió el pool de remitentes.", affects: "42 restaurantes", openedAt: "2026-09-11T18:22:00" },
];

export const mrrSeries = ["Oct 25", "Nov 25", "Dic 25", "Ene 26", "Feb 26", "Mar 26", "Abr 26", "May 26", "Jun 26", "Jul 26", "Ago 26", "Set 26"].map((m, i) => ({ month: m, mrr: 245100 + i * 21950 + (i % 3) * 1800, goal: 250000 + i * 21000 }));
