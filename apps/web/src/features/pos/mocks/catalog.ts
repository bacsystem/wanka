import type { CatalogItem } from "@/types/domain";

export const catalogCategories = ["Todos", "Tratamientos", "Insumos clínicos", "Medicamentos", "Consultas"] as const;

export const catalogMock: CatalogItem[] = [
  { id: "c1", sku: "SRV-0012", name: "Curación con resina fotocurable", kind: "servicio", category: "Tratamientos", price: 85, taxable: true, note: "Disponibilidad clínica inmediata" },
  { id: "c2", sku: "SRV-0004", name: "Profilaxis dental profunda", kind: "servicio", category: "Tratamientos", price: 70, taxable: true, note: "Sesión 40 min" },
  { id: "c3", sku: "INS-0102", name: "Kit instrumental descartable", kind: "producto", category: "Insumos clínicos", price: 15, taxable: true, stock: 48, unit: "un" },
  { id: "c4", sku: "MED-0441", name: "Anestesia lidocaína 2% c/epi", kind: "producto", category: "Medicamentos", price: 22.5, taxable: true, stock: 82, unit: "ampolla" },
  { id: "c5", sku: "SRV-0089", name: "Blanqueamiento dental LED", kind: "servicio", category: "Tratamientos", price: 320, taxable: true, note: "Sesión clínica 60 min" },
  { id: "c6", sku: "SRV-0019", name: "Radiografía periapical digital", kind: "servicio", category: "Consultas", price: 30, taxable: true, note: "Entrega digital inmediata" },
  { id: "c7", sku: "INS-0492", name: "Brackets metálicos set Roth", kind: "producto", category: "Insumos clínicos", price: 180, taxable: true, stock: 14, unit: "set" },
  { id: "c8", sku: "MED-0120", name: "Clorhexidina 0.12% x 500 ml", kind: "producto", category: "Medicamentos", price: 28, taxable: true, stock: 26, unit: "frasco" },
  { id: "c9", sku: "SRV-0001", name: "Consulta odontológica general", kind: "servicio", category: "Consultas", price: 60, taxable: true },
  { id: "c10", sku: "SRV-0033", name: "Exodoncia simple", kind: "servicio", category: "Tratamientos", price: 150, taxable: true },
  { id: "c11", sku: "INS-0210", name: "Guantes de nitrilo talla M x 100", kind: "producto", category: "Insumos clínicos", price: 45, taxable: true, stock: 3, unit: "caja" },
  { id: "c12", sku: "MED-0077", name: "Ibuprofeno 400 mg x 10", kind: "producto", category: "Medicamentos", price: 8, taxable: false, stock: 120, unit: "blíster", note: "Inafecto" },
];
