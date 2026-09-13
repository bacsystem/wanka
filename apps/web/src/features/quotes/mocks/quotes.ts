export type QuoteStatus = "aprobada" | "enviada" | "borrador" | "vencida";

export interface Quote {
  id: string;
  code: string;
  customerName: string;
  customerRuc: string;
  customerKind: "aseguradora" | "minera" | "colegio" | "transporte" | "otro";
  status: QuoteStatus;
  issuedAt: string;
  expiresAt: string;
  total: number;
  description: string;
  note?: string;
  seenByCustomer?: boolean;
}

export const quotesSummary = {
  active: 24, activeDelta: "+3 hoy", portfolio: 68900,
  approvedMonth: 15, approvedRate: 62.5, approvedAmount: 42500,
  expiring: 3, expiringAmount: 11200,
  closeDays: 3.2, closeDelta: "↓ 0.8 días vs. mes anterior",
};

export const quotesMock: Quote[] = [
  { id: "q1", code: "COT-2026-00128", customerName: "Rimac Seguros y Reaseguros EPS", customerRuc: "20100041953", customerKind: "aseguradora", status: "aprobada", issuedAt: "2026-09-10", expiresAt: "2026-09-15", total: 13153.46, description: "Paquete odontología integral x 25 colaboradores (profilaxis, resinas 3M, curaciones)", seenByCustomer: true },
  { id: "q2", code: "COT-2026-00127", customerName: "Compañía Minera Antamina S.A.", customerRuc: "20331898951", customerKind: "minera", status: "enviada", issuedAt: "2026-09-09", expiresAt: "2026-09-13", total: 8450, description: "Chequeos ocupacionales odontológicos Mina Huaraz x 40 trabajadores", note: "Pendiente confirmación de compras" },
  { id: "q3", code: "COT-2026-00126", customerName: "Colegio San Agustín de Lima", customerRuc: "20112938472", customerKind: "colegio", status: "borrador", issuedAt: "2026-09-07", expiresAt: "2026-09-30", total: 4120, description: "Campaña escolar de sellantes y flúor barniz (secundaria, 150 alumnos)" },
  { id: "q4", code: "COT-2026-00125", customerName: "Transportes Cruz del Sur S.A.C.", customerRuc: "20100227461", customerKind: "transporte", status: "vencida", issuedAt: "2026-08-20", expiresAt: "2026-09-05", total: 6300, description: "Evaluación odontológica para brevete A-III x 30 conductores", note: "Sin respuesta del cliente" },
  { id: "q5", code: "COT-2026-00124", customerName: "Inversiones Alpamayo S.A.C.", customerRuc: "20554182910", customerKind: "otro", status: "aprobada", issuedAt: "2026-09-04", expiresAt: "2026-09-18", total: 2980, description: "Ortodoncia (brackets metálicos) plan corporativo 2 colaboradores", seenByCustomer: true },
  { id: "q6", code: "COT-2026-00123", customerName: "Pacífico Seguros EPS", customerRuc: "20332970411", customerKind: "aseguradora", status: "enviada", issuedAt: "2026-09-02", expiresAt: "2026-09-16", total: 12400, description: "Convenio anual de tarifas preferenciales – renovación 2027" },
];

export interface QuoteLine { name: string; detail: string; qty: string; unit: number; subtotal: number }
/** Lines for the side panel (the editable detail page has its own full data). */
export function quoteLinesFor(q: Quote): QuoteLine[] {
  const base = q.total / 1.18;
  const parts = [0.28, 0.33, 0.2, 0.19];
  const names = [["Profilaxis dental profunda + ultrasonido", "Código Odonto: OD-201 · Dientes libres de sarro", "25 pac."], ["Restauraciones estéticas resina Filtek 3M", "Hasta 2 superficies por colaborador", "35 pzas."], ["Paquete blanqueamiento LED clínico", "Beneficio directores ejecutivos", "5 pac."], ["Radiografías panorámicas digitales HD", "Diagnóstico previo en equipo Planmeca", "25 und."]];
  return names.map(([name, detail, qty], i) => { const subtotal = Math.round(base * parts[i] * 100) / 100; const n = Number(qty.split(" ")[0]); return { name, detail, qty, unit: Math.round((subtotal / n) * 100) / 100, subtotal }; });
}
