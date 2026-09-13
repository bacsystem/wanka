export interface QuoteLine { id: string; code: string; description: string; detail: string; qty: number; unit: string; unitValue: number; discountPct: number; affectation: "10" | "20" }

export const quoteDetailMock = {
  code: "COT-2026-00128",
  status: "aprobada" as const,
  customer: { name: "Rimac Seguros y Reaseguros S.A.", ruc: "20100041953", tags: ["Habido", "Agente de retención"], site: "Sede Central Lima", contact: "Juan Rojas · Jefe de Compras" },
  seller: "Dr. Carlos Mendoza",
  issuedAt: "2026-09-10T09:15:00",
  expiresAt: "2026-09-25",
  currency: "PEN",
  paymentTerms: "Crédito 30 días",
  globalDiscount: 1350,
  detraction: { rate: 12, code: "022", label: "Otros servicios empresariales", account: "00-068-124859" },
  retention: { rate: 3 },
  timeline: [
    { label: "Borrador", at: "10/09 09:15", done: true },
    { label: "Enviada", at: "10/09 11:30 · mail / WA", done: true },
    { label: "Vista por cliente", at: "11/09 16:45 · 3 vistas", done: true },
    { label: "Aprobada", at: "12/09 10:20 · J. Rojas", done: true, current: true },
    { label: "Facturada", at: "Emisión F001", done: false },
  ],
  history: [
    { title: "Cotización aprobada", at: "12/09 10:20", text: "Aprobada por Juan Rojas (Jefe de Compras Rimac EPS). Adjunto O/C interna OC-RIMAC-8841.pdf" },
    { title: "Visualizada por cliente", at: "11/09 16:45", text: "Abierta por 3.ª vez desde IP corporativa Rimac (San Isidro)." },
    { title: "Envío multicanal", at: "10/09 11:30", text: "Enviada a compras@rimac.com.pe y WhatsApp corporativo." },
    { title: "Creación de proforma", at: "10/09 09:15", text: "Elaborada por Dr. Carlos Mendoza (rehabilitación oral)." },
  ],
  notes: "Instrucciones para el equipo clínico: coordinar con RR. HH. de Rimac el cronograma de atención de los 40 colaboradores (2 grupos de 20). Facturar tras la segunda jornada.",
  lines: [
    { id: "l1", code: "TRAT-0089", description: "Endodoncia mecanizada multirradicular", detail: "Piezas 1.6 y 2.6 con obturación termoplástica y sellador biocerámico", qty: 2, unit: "ZZ", unitValue: 450, discountPct: 0, affectation: "10" },
    { id: "l2", code: "TRAT-0042", description: "Restauración estética resina nanohíbrida 3M Z350 XT", detail: "Reconstrucción anatómica en 3 superficies posteriores con garantía", qty: 8, unit: "ZZ", unitValue: 120, discountPct: 5, affectation: "10" },
    { id: "l3", code: "MAT-0104", description: "Perno de fibra de vidrio + reconstrucción de muñón", detail: "Poste anatómico radiopaco y cementación dual autograbante", qty: 2, unit: "NIU", unitValue: 280, discountPct: 0, affectation: "10" },
    { id: "l4", code: "SERV-0012", description: "Paquete corporativo odontograma y profilaxis ultrasónica", detail: "Atención dental preventiva para 40 colaboradores de Rimac Seguros", qty: 40, unit: "ZZ", unitValue: 281.25, discountPct: 10, affectation: "10" },
  ] as QuoteLine[],
};
