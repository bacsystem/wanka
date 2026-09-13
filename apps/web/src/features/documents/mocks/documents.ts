import type { SalesDocument, SalesDocumentDetail } from "@/types/domain";

export const documentsMock: SalesDocument[] = [
  { id: "F001-00000842", type: "factura", series: "F001", number: 842, customerName: "Inversiones Alpamayo S.A.C.", customerDocument: "20554182910", total: 1280, currency: "PEN", sunatStatus: "pendiente", issuedAt: "2026-09-12T15:10:00" },
  { id: "B001-00004291", type: "boleta", series: "B001", number: 4291, customerName: "Juan Carlos Pérez Huamán", customerDocument: "45892147", total: 240, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T16:42:00" },
  { id: "B001-00004290", type: "boleta", series: "B001", number: 4290, customerName: "Mariana Quispe Flores", customerDocument: "72109843", total: 85, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-12T14:05:00" },
  { id: "F001-00000841", type: "factura", series: "F001", number: 841, customerName: "Distribuidora Médica del Sur S.R.L.", customerDocument: "20489123019", total: 3450, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-11T18:20:00" },
  { id: "FC01-00000104", type: "nota_credito", series: "FC01", number: 104, customerName: "Inversiones Alpamayo S.A.C.", customerDocument: "20554182910", total: 180, currency: "PEN", sunatStatus: "rechazado", issuedAt: "2026-09-11T17:15:00" },
  { id: "B001-00004289", type: "boleta", series: "B001", number: 4289, customerName: "Cliente genérico", customerDocument: "—", total: 60, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-11T13:48:00" },
  { id: "F001-00000840", type: "factura", series: "F001", number: 840, customerName: "Transportes & Logística Andina S.A.", customerDocument: "20601284759", total: 5480, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-10T11:30:00" },
  { id: "B001-00004288", type: "boleta", series: "B001", number: 4288, customerName: "Ana Lucía Paredes Vega", customerDocument: "41234567", total: 320, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-10T10:12:00" },
  { id: "F001-00000839", type: "factura", series: "F001", number: 839, customerName: "Consorcio Minero del Centro S.A.C.", customerDocument: "20554891204", total: 6800, currency: "PEN", sunatStatus: "aceptado", issuedAt: "2026-09-09T09:40:00" },
  { id: "B001-00004287", type: "boleta", series: "B001", number: 4287, customerName: "Carlos Raúl Benavides S.", customerDocument: "08765432", total: 150, currency: "USD", sunatStatus: "aceptado", issuedAt: "2026-09-08T17:05:00" },
];

export const documentsSummary = {
  totalIssued: 42850,
  count: 148,
  accepted: 142,
  acceptedAmount: 39550,
  pending: 4,
  pendingAmount: 3120,
  rejected: 2,
  deltaPercent: 8.4,
};

const base = documentsMock[0];
export const documentDetailMock: SalesDocumentDetail = {
  ...base,
  sunatStatus: "aceptado",
  customerAddress: "Av. Rivera Navarrete 525, Piso 8, San Isidro, Lima",
  customerEmail: "facturacion@alpamayo.pe",
  paymentTerms: "Contado (Transferencia BCP · Op. #948201)",
  operationType: "Venta interna (Catálogo 51 SUNAT)",
  lines: [
    { code: "TRAT-004", description: "Profilaxis y blanqueamiento LED premium", detail: "Higienización ultrasónica y aclaramiento en consultorio", quantity: 1, unit: "NIU", unitValue: 380, discount: 0, total: 448.4 },
    { code: "CIR-012", description: "Exodoncia simple pieza dentaria", detail: "Piezas 1.8 y 2.8 con anestesia infiltrativa local", quantity: 2, unit: "NIU", unitValue: 150, discount: 0, total: 354 },
    { code: "INS-088", description: "Férula miorrelajante digital (bruxismo)", detail: "Diseño CAD/CAM termoformado", quantity: 1, unit: "NIU", unitValue: 404.75, discount: 0, total: 477.6 },
  ],
  taxableBase: 1084.75,
  igv: 195.25,
  cdr: { hash: "7E3B5AF1C9824DB0EA5C3112F479AD04", acceptedAt: "2026-09-12T15:11:08", code: "0", message: "La Factura número F001-842 ha sido aceptada" },
  timeline: [
    { at: "2026-09-12T15:10:02", label: "Comprobante generado", detail: "POS-01 · Dra. M. Ramos", tone: "neutral" },
    { at: "2026-09-12T15:10:05", label: "XML firmado (UBL 2.1)", detail: "Certificado vigente hasta 14/05/2027", tone: "neutral" },
    { at: "2026-09-12T15:10:40", label: "Enviado a OSE", detail: "Ticket 1726150240-001", tone: "warning" },
    { at: "2026-09-12T15:11:08", label: "Aceptado por SUNAT (CDR)", detail: "Código 0 · sin observaciones", tone: "success" },
    { at: "2026-09-12T15:12:00", label: "PDF enviado por correo", detail: "facturacion@alpamayo.pe", tone: "neutral" },
  ],
  paid: 1280,
  paymentMethod: "Transferencia BCP",
};

export function getDocumentDetail(id: string): SalesDocumentDetail | undefined {
  const doc = documentsMock.find((d) => d.id === id);
  if (!doc) return undefined;
  if (doc.id === documentDetailMock.id) return documentDetailMock;
  // Other documents reuse the detail template with their own header data.
  const taxableBase = Math.round((doc.total / 1.18) * 100) / 100;
  return {
    ...documentDetailMock,
    ...doc,
    taxableBase,
    igv: Math.round((doc.total - taxableBase) * 100) / 100,
    lines: [{ code: "SRV-0001", description: "Servicios odontológicos según detalle", quantity: 1, unit: "NIU", unitValue: taxableBase, discount: 0, total: doc.total }],
    paid: doc.sunatStatus === "rechazado" ? 0 : doc.total,
    cdr: doc.sunatStatus === "aceptado" ? documentDetailMock.cdr : doc.sunatStatus === "rechazado"
      ? { hash: "—", code: "2335", message: "El documento electrónico ingresado ha sido alterado" }
      : undefined,
    timeline: documentDetailMock.timeline.slice(0, doc.sunatStatus === "aceptado" ? 5 : 3),
  };
}
