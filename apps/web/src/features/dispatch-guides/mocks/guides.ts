import type { SunatStatus } from "@/types/domain";

export type GuideReason = "venta" | "traslado_sedes" | "devolucion";
export type GuideStatus = SunatStatus | "en_transito" | "entregado";

export interface DispatchGuide {
  id: string;
  number: string;
  issuedAt: string;
  transferAt: string;
  reason: GuideReason;
  modality: "privado" | "publico";
  origin: string;
  destination: string;
  plate: string;
  carrier: string; // driver or company
  carrierDoc: string;
  weightKg: number;
  status: GuideStatus;
}

export const guidesSummary = { monthCount: 48, monthDelta: 12.4, betweenSites: 18, inTransit: 6, cdrRate: 100 };

export const guidesMock: DispatchGuide[] = [
  { id: "g1", number: "T001-000145", issuedAt: "2026-09-12", transferAt: "2026-09-12", reason: "traslado_sedes", modality: "privado", origin: "Av. Larco 743, Miraflores", destination: "Av. Conquistadores 410, San Isidro", plate: "BXY-842", carrier: "R. Medina", carrierDoc: "Q45892101", weightKg: 42.5, status: "en_transito" },
  { id: "g2", number: "T001-000144", issuedAt: "2026-09-12", transferAt: "2026-09-12", reason: "venta", modality: "publico", origin: "Almacén Surquillo #120", destination: "Av. Javier Prado Este 3240, San Borja", plate: "A9L-901", carrier: "Carga Express S.A.C.", carrierDoc: "RUC 20554120984", weightKg: 185, status: "aceptado" },
  { id: "g3", number: "T001-000143", issuedAt: "2026-09-11", transferAt: "2026-09-11", reason: "devolucion", modality: "publico", origin: "Av. Larco 743, Miraflores", destination: "Av. Argentina 2020, Callao (Dental Import)", plate: "C4T-118", carrier: "Transportes Lima Norte S.R.L.", carrierDoc: "RUC 20487120933", weightKg: 12, status: "entregado" },
  { id: "g4", number: "T001-000142", issuedAt: "2026-09-10", transferAt: "2026-09-10", reason: "traslado_sedes", modality: "privado", origin: "Almacén Surquillo #120", destination: "Av. Larco 743, Miraflores", plate: "BXY-842", carrier: "R. Medina", carrierDoc: "Q45892101", weightKg: 68, status: "entregado" },
  { id: "g5", number: "T001-000141", issuedAt: "2026-09-09", transferAt: "2026-09-10", reason: "venta", modality: "publico", origin: "Av. Larco 743, Miraflores", destination: "Jr. Huallaga 250, Cercado (Colegio San Agustín)", plate: "F2K-330", carrier: "Olva Courier S.A.C.", carrierDoc: "RUC 20100096341", weightKg: 9.5, status: "pendiente" },
  { id: "g6", number: "T001-000140", issuedAt: "2026-09-08", transferAt: "2026-09-08", reason: "traslado_sedes", modality: "privado", origin: "Av. Conquistadores 410, San Isidro", destination: "Almacén Surquillo #120", plate: "BXY-842", carrier: "J. Torres", carrierDoc: "Q41230098", weightKg: 30, status: "rechazado" },
];
