export type StockState = "optimo" | "alerta" | "critico";

export interface StockItem {
  id: string;
  sku: string;
  name: string;
  presentation: string;
  category: string;
  lot?: string;
  expiresAt?: string; // mm/yyyy
  stock: number;
  unit: string;
  minimum: number;
  avgCost: number;
  warehouseId: string;
}

export interface WarehouseInfo { id: string; name: string; short: string }

export const warehouses: WarehouseInfo[] = [
  { id: "all", name: "Todos los almacenes (consolidado)", short: "Consolidado" },
  { id: "w1", name: "Sede Central – Miraflores (principal)", short: "Miraflores" },
  { id: "w2", name: "Sede San Isidro (consultorios)", short: "San Isidro" },
  { id: "w3", name: "Almacén logístico Surquillo", short: "Surquillo" },
];

export const stockCategories = ["Todos", "Insumos clínicos", "Medicamentos", "Instrumental quirúrgico", "Material dental"];

export const stockSummary = { skus: 342, available: 248, discontinued: 12, valuation: 84230.5, critical: 5, suggestedPO: 3, transfersInTransit: 2 };

export const stockMock: StockItem[] = [
  { id: "k1", sku: "MED-0441", name: "Anestesia lidocaína 2% c/epinefrina", presentation: "Carpules 1.8 ml · New Stetic", category: "Medicamentos", lot: "LT 2024-B88", expiresAt: "11/2027", stock: 82, unit: "amp", minimum: 30, avgCost: 18, warehouseId: "w1" },
  { id: "k2", sku: "MAT-0084", name: "Resina fotocurable Filtek Z350 XT", presentation: "Jeringa 4 g tono A2 · 3M ESPE", category: "Material dental", lot: "LT 7781", expiresAt: "03/2028", stock: 0, unit: "jer", minimum: 5, avgCost: 210, warehouseId: "w1" },
  { id: "k3", sku: "INS-0102", name: "Guantes de nitrilo talla M", presentation: "Caja x 100 · Medicom", category: "Insumos clínicos", lot: "LT 2025-01", expiresAt: "01/2029", stock: 3, unit: "caja", minimum: 20, avgCost: 38, warehouseId: "w1" },
  { id: "k4", sku: "INS-0210", name: "Mascarillas KN95", presentation: "Caja x 20", category: "Insumos clínicos", stock: 4, unit: "caja", minimum: 15, avgCost: 24, warehouseId: "w2" },
  { id: "k5", sku: "QUI-0031", name: "Fórceps 150 universal superior", presentation: "Acero inoxidable · Hu-Friedy", category: "Instrumental quirúrgico", stock: 6, unit: "un", minimum: 4, avgCost: 320, warehouseId: "w1" },
  { id: "k6", sku: "MED-0120", name: "Clorhexidina 0.12%", presentation: "Frasco 500 ml · Perio-Aid", category: "Medicamentos", lot: "LT 4410", expiresAt: "08/2027", stock: 26, unit: "fco", minimum: 10, avgCost: 21, warehouseId: "w3" },
  { id: "k7", sku: "MAT-0112", name: "Hilo retractor #00", presentation: "Carrete 254 cm · Ultradent", category: "Material dental", stock: 0, unit: "un", minimum: 3, avgCost: 65, warehouseId: "w2" },
  { id: "k8", sku: "INS-0492", name: "Brackets metálicos set Roth .022", presentation: "Set 20 piezas · Morelli", category: "Insumos clínicos", stock: 14, unit: "set", minimum: 5, avgCost: 140, warehouseId: "w3" },
  { id: "k9", sku: "MED-0077", name: "Ibuprofeno 400 mg", presentation: "Blíster x 10 · Genfar", category: "Medicamentos", lot: "LT 9031", expiresAt: "05/2028", stock: 120, unit: "bl", minimum: 40, avgCost: 4.5, warehouseId: "w1" },
  { id: "k10", sku: "QUI-0099", name: "Kit de exodoncia (elevadores + fórceps)", presentation: "Estuche 8 piezas", category: "Instrumental quirúrgico", stock: 2, unit: "kit", minimum: 2, avgCost: 890, warehouseId: "w2" },
];

export function stockState(i: StockItem): StockState {
  if (i.stock === 0 || i.stock < i.minimum * 0.25) return "critico";
  if (i.stock < i.minimum) return "alerta";
  return "optimo";
}
