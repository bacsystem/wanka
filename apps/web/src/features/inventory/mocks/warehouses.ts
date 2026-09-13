export type WarehouseKind = "principal" | "subalmacen" | "sucursal" | "restringido";

export interface WarehouseCard {
  id: string;
  name: string;
  kind: WarehouseKind;
  sunatAnnex?: string;
  site: string;
  address: string;
  description: string;
  valueLabel: string;
  value: number;
  skusLabel: string;
  facts: { label: string; value: string }[];
  actions: string[];
}

export const warehousesSummary = { total: 4, sites: 2, bySite: "Miraflores (3) · San Isidro (1)", occupancy: 68, occupancyDelta: 4.2, annexes: 2, custodians: 3 };

export const warehouseCards: WarehouseCard[] = [
  { id: "w1", name: "Almacén Central de Farmacia e Insumos", kind: "principal", sunatAnnex: "0000", site: "Sede Miraflores", address: "Av. José Larco 850, Piso 1 · Domicilio fiscal", description: "Depósito farmacéutico central: anestésicos, composites, instrumental de ortodoncia y resinas de laboratorio.", valueLabel: "Valorizado total kardex", value: 142800, skusLabel: "154 SKU activos", facts: [{ label: "Responsable registrado", value: "Q.F. Andrea Benavides" }, { label: "Guía de remisión remitente", value: "Serie T001 habilitada" }, { label: "Nivel de reorden", value: "Automatizado (mín/máx)" }], actions: ["Ver inventario (154)", "Transferir stock", "Configurar series GRE"] },
  { id: "w2", name: "Stock rápido quirófano y sillones 1–4", kind: "subalmacen", site: "Sede Miraflores", address: "Piso 2 · Área quirúrgica y boxes de atención", description: "Reposición diaria de carpules de mepivacaína, eyectores, agujas descartables e hilos de sutura.", valueLabel: "Valorizado", value: 28400, skusLabel: "42 SKU de alta rotación", facts: [{ label: "Responsable en turno", value: "Dra. Mariana Ramos" }, { label: "Frecuencia de cierre", value: "Fin de jornada (20:00)" }, { label: "Control de consumo", value: "Descargo por ficha clínica" }], actions: ["Ver ítems (42)", "Solicitar reabastecimiento", "Arqueo de turno"] },
  { id: "w3", name: "Almacén San Isidro – Clínica Dental Sonrisa", kind: "sucursal", sunatAnnex: "0001", site: "Sede San Isidro", address: "Av. Dos de Mayo 1420, Of. 302, Lima 27", description: "Almacén integral de odontopediatría, implantología y consumibles para sillones 5, 6 y 7.", valueLabel: "Valorizado total kardex", value: 65300, skusLabel: "98 SKU activos", facts: [{ label: "Responsable de sede", value: "Dr. Fernando Zegarra" }, { label: "Recepción de envíos", value: "Desde Sede Miraflores (GRE)" }, { label: "Sincronización POS", value: "En línea con SUNAT" }], actions: ["Ver inventario (98)", "Recepcionar traslado", "Auditoría física"] },
  { id: "w4", name: "Almacén de mermas y cuarentena", kind: "restringido", site: "Sede Miraflores", address: "Depósito B · Sótano 1 (seguridad biológica)", description: "Almacén transitorio para bajas por caducidad, lotes defectuosos para devolución y destrucción notarial.", valueLabel: "Monto en retención", value: 1240, skusLabel: "8 SKU en proceso", facts: [{ label: "Acta notarial programada", value: "28/10/2026" }, { label: "Motivo predominante", value: "Vencimiento de lotes (5)" }, { label: "Control de acceso", value: "Solo regente farmacéutico" }], actions: ["Ver ítems en cuarentena (8)", "Generar acta de destrucción"] },
];

export const sunatEstablishments = [
  { code: "0000", label: "Domicilio fiscal / principal", address: "Av. Larco 850, Int. 101, Miraflores, Lima", ubigeo: "150122", status: "HABIDO", kind: "Local clínico", linked: 3 },
  { code: "0001", label: "Sucursal San Isidro", address: "Av. Dos de Mayo 1420, Of. 302, San Isidro, Lima", ubigeo: "150131", status: "HABIDO", kind: "Sucursal", linked: 1 },
];

export const custodians = [
  { initials: "AB", name: "Q.F. Andrea Benavides", detail: "CQFP 14829 · Sede Miraflores" },
  { initials: "FZ", name: "Dr. Fernando Zegarra", detail: "COP 22410 · Sede San Isidro" },
  { initials: "MR", name: "Dra. Mariana Ramos", detail: "COP 31904 · Quirófano" },
];

export const transfersInTransit = [{ guide: "GRE T001-000428", route: "Miraflores → San Isidro", status: "En despacho", items: 14 }];
