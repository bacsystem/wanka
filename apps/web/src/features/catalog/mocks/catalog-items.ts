export type IgvAffectation = "10" | "20" | "30";
export type ItemNature = "servicio" | "producto";

export interface CatalogEntry {
  id: string;
  sku: string;
  name: string;
  description: string;
  nature: ItemNature;
  category: string;
  unspsc: string;
  price: number; // inc. IGV
  cost?: number;
  affectation: IgvAffectation;
  stock?: { qty: number; unit: string; warehouse: string; critical?: boolean };
  tariff: string; // EPS / list summary
  active: boolean;
}

export interface CatalogCategory { id: string; name: string; specialty: string; items: number; active: boolean }

export const catalogSummary = { items: 184, services: 112, products: 72, lowStock: 4, taxedShare: 82, exemptShare: 18 };

export const affectationLabel: Record<IgvAffectation, string> = { "10": "10 Gravado (18%)", "20": "20 Exonerado", "30": "30 Inafecto" };

export const catalogEntries: CatalogEntry[] = [
  { id: "e1", sku: "SRV-0012", name: "Curación con resina fotocurable 3M Z350", description: "Restauración estética por pieza dental anterior/posterior, incluye pulido", nature: "servicio", category: "Odontología general", unspsc: "85122001", price: 85, cost: 33, affectation: "10", tariff: "Rimac S/ 72.25 · copago S/ 15.00", active: true },
  { id: "e2", sku: "MED-0441", name: "Lidocaína 2% con epinefrina 1:100,000 (caja x 50)", description: "Anestésico local dental inyectable, cartucho de vidrio 1.8 ml", nature: "producto", category: "Farmacia e insumos", unspsc: "51101500", price: 115, cost: 78.5, affectation: "10", stock: { qty: 34, unit: "cajas", warehouse: "Almacén Central" }, tariff: "Tarifa directa farmacia", active: true },
  { id: "e3", sku: "SRV-0008", name: "Profilaxis dental profunda con ultrasonido", description: "Destartraje supragingival, remoción de placa y fluorización en gel", nature: "servicio", category: "Periodoncia", unspsc: "85122002", price: 70, cost: 18, affectation: "20", tariff: "Rimac S/ 55.00 · Pacífico S/ 58.00", active: true },
  { id: "e4", sku: "INS-0102", name: "Brackets metálicos set Roth 0.022 con ganchos", description: "Juego de 20 brackets prescripción Roth slot .022 Morelli", nature: "producto", category: "Ortodoncia", unspsc: "42151600", price: 180, cost: 140, affectation: "10", stock: { qty: 4, unit: "sets", warehouse: "Almacén Central", critical: true }, tariff: "Particular / paquete orto", active: true },
  { id: "e5", sku: "SRV-0035", name: "Blanqueamiento dental láser LED (2 sesiones)", description: "Peróxido de hidrógeno al 35%, aislamiento gingival fotoactivado", nature: "servicio", category: "Estética y blanqueamiento", unspsc: "85122003", price: 380, cost: 95, affectation: "10", tariff: "Promo web S/ 320.00 · sin cobertura EPS", active: true },
  { id: "e6", sku: "SRV-0001", name: "Consulta odontológica general", description: "Evaluación clínica, diagnóstico y plan de tratamiento", nature: "servicio", category: "Odontología general", unspsc: "85121800", price: 60, cost: 12, affectation: "20", tariff: "Rimac S/ 48.00 · Pacífico S/ 50.00", active: true },
  { id: "e7", sku: "MED-0120", name: "Clorhexidina 0.12% x 500 ml", description: "Enjuague bucal antiséptico post quirúrgico", nature: "producto", category: "Farmacia e insumos", unspsc: "51102700", price: 28, cost: 21, affectation: "10", stock: { qty: 26, unit: "frascos", warehouse: "Surquillo" }, tariff: "Tarifa directa farmacia", active: true },
  { id: "e8", sku: "SRV-0044", name: "Sellante de fosas y fisuras (por pieza)", description: "Prevención de caries en molares permanentes, odontopediatría", nature: "servicio", category: "Odontopediatría", unspsc: "85122004", price: 45, cost: 9, affectation: "20", tariff: "Convenio escolar S/ 35.00", active: false },
];

export const catalogCategoriesMock: CatalogCategory[] = [
  { id: "c1", name: "Odontología general", specialty: "Operatoria y diagnóstico", items: 28, active: true },
  { id: "c2", name: "Ortodoncia", specialty: "Ortodoncia y ortopedia", items: 19, active: true },
  { id: "c3", name: "Periodoncia", specialty: "Periodoncia e implantes", items: 14, active: true },
  { id: "c4", name: "Cirugía bucal", specialty: "Cirugía bucal y maxilofacial", items: 11, active: true },
  { id: "c5", name: "Estética y blanqueamiento", specialty: "Estética dental", items: 9, active: true },
  { id: "c6", name: "Odontopediatría", specialty: "Atención infantil", items: 12, active: true },
  { id: "c7", name: "Endodoncia", specialty: "Tratamiento de conductos", items: 8, active: true },
  { id: "c8", name: "Farmacia e insumos", specialty: "Fármacos y consumibles", items: 72, active: true },
  { id: "c9", name: "Prótesis", specialty: "Laboratorio dental", items: 11, active: false },
];
