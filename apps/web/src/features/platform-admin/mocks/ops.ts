export interface StaffUser { id: string; name: string; email: string; area: "Soporte técnico" | "Ventas y expansión" | "Finanzas y cobranzas" | "Ingeniería y DevOps" | "SuperAdmin global"; role: string; twoFa: "activo" | "pendiente" | "no"; sessions: number; lastAccess: string; ip: string; online: boolean; status: "activa" | "suspendida" }
export const staff: StaffUser[] = [
  { id: "u1", name: "Carlos M. Benavides", email: "superadmin@perusaas.pe", area: "SuperAdmin global", role: "SRE / SuperAdmin N3", twoFa: "activo", sessions: 3, lastAccess: "2026-09-12T15:40:00", ip: "190.237.44.12", online: true, status: "activa" },
  { id: "u2", name: "Lucía Paredes Ríos", email: "lucia.paredes@perusaas.pe", area: "Soporte técnico", role: "Soporte nivel 2", twoFa: "activo", sessions: 2, lastAccess: "2026-09-12T15:22:00", ip: "181.67.112.4", online: true, status: "activa" },
  { id: "u3", name: "Renzo Villanueva", email: "renzo.villanueva@perusaas.pe", area: "Ventas y expansión", role: "Ejecutivo comercial", twoFa: "pendiente", sessions: 1, lastAccess: "2026-09-12T11:05:00", ip: "200.48.10.77", online: false, status: "activa" },
  { id: "u4", name: "Ana Sofía Morales", email: "ana.morales@perusaas.pe", area: "Finanzas y cobranzas", role: "Analista de cobranzas", twoFa: "activo", sessions: 1, lastAccess: "2026-09-11T18:30:00", ip: "179.6.211.89", online: false, status: "activa" },
  { id: "u5", name: "Diego Quispe Huamán", email: "diego.quispe@perusaas.pe", area: "Ingeniería y DevOps", role: "Ingeniero de plataforma", twoFa: "activo", sessions: 4, lastAccess: "2026-09-12T15:41:00", ip: "190.237.44.12", online: true, status: "activa" },
  { id: "u6", name: "Mariana Torres", email: "mariana.torres@perusaas.pe", area: "Soporte técnico", role: "Soporte nivel 1", twoFa: "no", sessions: 0, lastAccess: "2026-08-28T09:12:00", ip: "38.25.19.140", online: false, status: "suspendida" },
];
export const staffSummary = { total: 38, newMonth: 3, areas: 4, mfa: 97.4, mfaCount: "37 / 38", sessions: 42, ips: 18, failed24h: 3 };
export const roleMatrix = { roles: ["SuperAdmin", "Soporte N1-N3", "Finanzas", "Ventas", "Ingeniería"], modules: [["Tenants: ver", [1, 1, 1, 1, 1]], ["Tenants: impersonar", [1, 1, 0, 0, 0]], ["Tenants: suspender", [1, 0, 1, 0, 0]], ["Planes y precios", [1, 0, 1, 1, 0]], ["Facturación SaaS", [1, 0, 1, 0, 0]], ["Certificados / OSE", [1, 1, 0, 0, 1]], ["Feature flags", [1, 0, 0, 0, 1]], ["Auditoría SIEM", [1, 1, 1, 0, 1]], ["Configuración núcleo", [1, 0, 0, 0, 1]]] as [string, number[]][] };

export interface Ticket { id: string; priority: "P1" | "P2" | "P3"; tenant: string; subject: string; channel: "WhatsApp" | "Correo" | "In-app"; agent: string; status: "abierto" | "en_progreso" | "esperando" | "resuelto"; slaMin: number; createdAt: string }
export const tickets: Ticket[] = [
  { id: "TK-10432", priority: "P1", tenant: "Clínica Dental OdontoSalud S.A.C.", subject: "Rechazo masivo CDR por error 1033 (SUNAT directo)", channel: "WhatsApp", agent: "Lucía Paredes", status: "en_progreso", slaMin: 18, createdAt: "2026-09-12T14:52:00" },
  { id: "TK-10431", priority: "P1", tenant: "Cevicherías La Mar S.A.", subject: "Niubiz: débito automático rechazado x3", channel: "Correo", agent: "Ana Sofía Morales", status: "abierto", slaMin: 26, createdAt: "2026-09-12T14:40:00" },
  { id: "TK-10428", priority: "P2", tenant: "PetMed Sur E.I.R.L.", subject: "Solicitud de ampliación de cuota CPE (94%)", channel: "In-app", agent: "Renzo Villanueva", status: "esperando", slaMin: 240, createdAt: "2026-09-12T12:10:00" },
  { id: "TK-10425", priority: "P3", tenant: "Bodega Don Pepe", subject: "Impresora térmica no imprime QR", channel: "WhatsApp", agent: "Mariana Torres", status: "en_progreso", slaMin: 610, createdAt: "2026-09-12T09:30:00" },
  { id: "TK-10419", priority: "P2", tenant: "Veterinaria Los Andes Cusco S.R.L.", subject: "Certificado .pfx renovado no valida en Nubefact", channel: "Correo", agent: "Diego Quispe", status: "resuelto", slaMin: 0, createdAt: "2026-09-11T16:05:00" },
];
export const supportSummary = { firstResponse: 14, firstResponseDelta: -3.2, slaMet: 96.8, mttr: "1h 48m", mttrDelta: "−18m", record: "42m", atRisk: 2, overdue: 0, csat: 4.92, positive: 98.6, surveys: 218 };

export interface FeatureFlag { id: string; name: string; description: string; vertical: string; plans: { basico: boolean; pro: boolean; enterprise: boolean }; rollout: number; tenants: number; updatedAt: string; by: string }
export const flags: FeatureFlag[] = [
  { id: "f1", name: "Agenda de citas y turnos", description: "Programación por sillón / consultorio con recordatorios WhatsApp.", vertical: "Odontología · Veterinaria", plans: { basico: true, pro: true, enterprise: true }, rollout: 100, tenants: 675, updatedAt: "2026-08-20", by: "Diego Quispe" },
  { id: "f2", name: "Salón, mesas y KDS", description: "Mapa de mesas, comandas y tablero de cocina.", vertical: "Restaurante", plans: { basico: false, pro: true, enterprise: true }, rollout: 100, tenants: 472, updatedAt: "2026-07-02", by: "Carlos Benavides" },
  { id: "f3", name: "Mascotas y carné de vacunas", description: "Ficha de paciente, SOAP y recordatorios SENASA.", vertical: "Veterinaria", plans: { basico: true, pro: true, enterprise: true }, rollout: 100, tenants: 291, updatedAt: "2026-06-15", by: "Diego Quispe" },
  { id: "f4", name: "Variantes retail y códigos de barras", description: "Matriz talla × color, EAN-13 y venta rápida POS.", vertical: "Retail", plans: { basico: false, pro: true, enterprise: true }, rollout: 60, tenants: 201, updatedAt: "2026-09-09", by: "Carlos Benavides" },
  { id: "f5", name: "Hospitalización y grooming", description: "Kennels, constantes vitales y peluquería.", vertical: "Veterinaria", plans: { basico: false, pro: true, enterprise: true }, rollout: 35, tenants: 102, updatedAt: "2026-09-11", by: "Lucía Paredes" },
  { id: "f6", name: "Delivery y reservas", description: "Tablero de despacho, repartidores y reservas de mesa.", vertical: "Restaurante", plans: { basico: false, pro: false, enterprise: true }, rollout: 20, tenants: 39, updatedAt: "2026-09-12", by: "Diego Quispe" },
];

export interface AuditEvent { id: string; at: string; type: "impersonacion" | "exportacion" | "plan" | "acceso_fallido" | "config" | "certificado"; actor: string; tenant?: string; detail: string; ip: string; risk: "alto" | "medio" | "bajo" }
export const auditEvents: AuditEvent[] = [
  { id: "e1", at: "2026-09-12T15:38:12", type: "impersonacion", actor: "Lucía Paredes (Soporte N2)", tenant: "Clínica Dental OdontoSalud S.A.C.", detail: "Sesión de impersonación iniciada (ticket TK-10432) · 30 min", ip: "181.67.112.4", risk: "medio" },
  { id: "e2", at: "2026-09-12T15:02:44", type: "exportacion", actor: "Ana Sofía Morales (Finanzas)", detail: "Exportación masiva: facturas SaaS setiembre 2026 (1,482 registros)", ip: "179.6.211.89", risk: "medio" },
  { id: "e3", at: "2026-09-12T14:20:05", type: "plan", actor: "Renzo Villanueva (Ventas)", tenant: "PetMed Sur E.I.R.L.", detail: "Cambio de plan: Clínico Base → Pro Multi-Sede (pendiente de aprobación)", ip: "200.48.10.77", risk: "bajo" },
  { id: "e4", at: "2026-09-12T13:47:31", type: "acceso_fallido", actor: "desconocido", detail: "5 intentos fallidos consecutivos sobre superadmin@perusaas.pe · IP bloqueada 15 min", ip: "45.231.88.9", risk: "alto" },
  { id: "e5", at: "2026-09-12T11:10:00", type: "config", actor: "Carlos M. Benavides (SuperAdmin)", detail: "Regla de failover OSE: umbral de latencia 1,200 ms → 1,000 ms", ip: "190.237.44.12", risk: "medio" },
  { id: "e6", at: "2026-09-12T09:00:15", type: "certificado", actor: "sistema", tenant: "Veterinaria San Martín", detail: "Certificado digital expirado · timbrado retenido en contingencia", ip: "—", risk: "alto" },
  { id: "e7", at: "2026-09-11T18:22:40", type: "impersonacion", actor: "Diego Quispe (Ingeniería)", tenant: "Cevicherías La Mar S.A.", detail: "Impersonación finalizada · 12 min · 0 cambios", ip: "190.237.44.12", risk: "bajo" },
];
