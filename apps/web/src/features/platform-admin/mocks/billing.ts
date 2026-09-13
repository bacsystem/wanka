export interface Plan { id: string; name: string; tagline: string; audience: string; price: number; annual: number; annualNote: string; cpe: string; users: string; sites: string; modules: string[]; support: string; tenants: number; featured?: boolean }
export const plans: Plan[] = [
  { id: "basico", name: "Básico", tagline: "Nivel entrada", audience: "Ideal para consultorios individuales, veterinarias y micro-comercios.", price: 180, annual: 1800, annualNote: "S/ 1,800 anual (−16%)", cpe: "1,000 / mes", users: "3 usuarios", sites: "1 sede física", modules: ["Facturación OSE", "Inventario básico", "Agenda de citas"], support: "Soporte por ticket (24h)", tenants: 426 },
  { id: "pro", name: "Pro", tagline: "Profesional multi-sede", audience: "Para clínicas dentales, centros médicos y cadenas medianas con cajas POS.", price: 490, annual: 4900, annualNote: "S/ 4,900 anual (2 meses gratis)", cpe: "10,000 / mes", users: "15 usuarios", sites: "Hasta 5 sedes", modules: ["Todo lo de Básico", "Multi-almacén y kardex 13.1", "Odontograma / historia clínica", "Salón y mesas / mascotas"], support: "Soporte prioritario (4h) + WhatsApp", tenants: 859, featured: true },
  { id: "enterprise", name: "Enterprise", tagline: "Cadenas y franquicias", audience: "Grupos gastronómicos, cadenas veterinarias y retail con alta concurrencia.", price: 850, annual: 8500, annualNote: "S/ 8,500 anual + SLA dedicado", cpe: "50,000 / mes (ampliable)", users: "Ilimitados", sites: "Ilimitadas", modules: ["Todo lo de Pro", "API y webhooks", "SSO / SCIM", "Entorno de pruebas dedicado"], support: "Gerente de cuenta + SLA 99.9%", tenants: 197 },
];
export const billingSummary = { collected: 486520, collectedDelta: 12.4, niubizShare: 68, bcpShare: 32, dueSoon: 42150, dueCompanies: 84, autoDebit: 79, overdue: 14890, overdueTenants: 19, creditNotes: 2450, creditNotesCount: 4, autoReconcile: 98.2, manualPending: 3, activeSubs: 1409, onTime: 95.1, delinquent: 28, delinquentAmount: 14280, suspended: 9 };
export type InvoiceStatus = "pagado" | "pendiente" | "moroso" | "anulado";
export interface SaasInvoice { id: string; number: string; type: "01" | "03" | "07"; issuedAt: string; tenant: string; ruc: string; concept: string; total: number; gateway: string; reconciled: boolean; status: InvoiceStatus; retries?: number }
export const saasInvoices: SaasInvoice[] = [
  { id: "i1", number: "F001-0004921", type: "01", issuedAt: "2026-09-01T08:30:00", tenant: "OdontoSalud S.A.C.", ruc: "20601984211", concept: "Plan Pro Multi-Sede + 5k CPE", total: 578.2, gateway: "Niubiz débito auto", reconciled: true, status: "pagado" },
  { id: "i2", number: "F001-0004918", type: "01", issuedAt: "2026-09-01T08:30:00", tenant: "Cevicherías La Mar S.A.", ruc: "20498112349", concept: "Plan Unlimited POS", total: 1003, gateway: "Transferencia BCP", reconciled: true, status: "pagado" },
  { id: "i3", number: "F001-0004902", type: "01", issuedAt: "2026-08-15T09:10:00", tenant: "Textil Gamarra Perú S.A.C.", ruc: "20114092812", concept: "Plan Básico", total: 330.4, gateway: "Niubiz débito auto", reconciled: false, status: "moroso", retries: 3 },
  { id: "i4", number: "F001-0004930", type: "01", issuedAt: "2026-09-05T10:00:00", tenant: "PetMed Sur E.I.R.L.", ruc: "20554819230", concept: "Plan Clínico Base", total: 377.6, gateway: "Izipay checkout", reconciled: false, status: "pendiente" },
  { id: "i5", number: "FC01-0000118", type: "07", issuedAt: "2026-09-08T16:20:00", tenant: "Veterinaria Los Andes Cusco S.R.L.", ruc: "20603349182", concept: "NC por downgrade a Plan Clínico", total: -200.6, gateway: "—", reconciled: true, status: "pagado" },
  { id: "i6", number: "B001-0002210", type: "03", issuedAt: "2026-09-10T11:45:00", tenant: "Bodega Don Pepe", ruc: "10456789012", concept: "Plan Básico", total: 212.4, gateway: "PagoEfectivo", reconciled: false, status: "pendiente" },
];
export const providers = [
  { name: "SUNAT Directo", host: "ws.sunat.gob.pe", status: "warning" as const, label: "Degradado", latencyMs: 1420, queue: 4280, reject: 3.82, availability: 97.8, note: "Modo de emisión: bypass OSE activo" },
  { name: "Bizlinks OSE", host: "api.bizlinks.com.pe", status: "ok" as const, label: "Operativo", latencyMs: 185, queue: 42, reject: 0.08, availability: 99.99, note: "Capacidad absorbida: 62% (alta carga)", default: true },
  { name: "Nubefact OSE", host: "ose.nubefact.com", status: "ok" as const, label: "Operativo", latencyMs: 210, queue: 18, reject: 0.12, availability: 99.95, note: "Capacidad absorbida: 24% (normal)" },
  { name: "Efact OSE", host: "ose.efact.pe", status: "ok" as const, label: "Operativo", latencyMs: 245, queue: 5, reject: 0.05, availability: 99.89, note: "Capacidad absorbida: 14% (reserva)" },
];
export const latency24h = Array.from({ length: 24 }, (_, h) => ({ hour: `${String(h).padStart(2, "0")}:00`, sunat: h >= 12 && h <= 16 ? 900 + (h - 12) * 230 : 320 + (h % 5) * 40, bizlinks: 160 + (h % 4) * 12, nubefact: 190 + (h % 3) * 15, efact: 230 + (h % 6) * 8 }));
export const expiringCerts = [
  { tenant: "Clínica Dental OdontoSalud S.A.C.", ruc: "20601984211", days: 18, ose: "Bizlinks OSE" },
  { tenant: "Veterinaria San Martín", ruc: "20548812091", days: 0, ose: "Nubefact OSE" },
  { tenant: "Pollería El Fogón S.A.C.", ruc: "20511102934", days: 42, ose: "Efact OSE" },
  { tenant: "Farmacia Vida Sana E.I.R.L.", ruc: "20601123890", days: 67, ose: "Bizlinks OSE" },
];
export const rejectTop = [
  { tenant: "Textil Gamarra Perú S.A.C.", reject: 12.4, cause: "Token OAuth vencido" },
  { tenant: "Pollería El Fogón S.A.C.", reject: 4.1, cause: "Error 2335 fecha fuera de plazo" },
  { tenant: "Dental Norte Trujillo", reject: 2.2, cause: "RUC receptor no habido" },
];
