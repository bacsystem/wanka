export type Industry = "restaurante" | "veterinaria" | "odontologia" | "retail";

export interface Tenant {
  id: string;
  name: string;
  ruc: string;
  industry: Industry;
}

export type DocumentType =
  | "boleta"
  | "factura"
  | "nota_credito"
  | "proforma"
  | "guia_remision";

export type SunatStatus = "aceptado" | "pendiente" | "rechazado";

export interface SalesDocument {
  id: string;
  type: DocumentType;
  series: string;
  number: number;
  customerName: string;
  customerDocument: string;
  total: number;
  currency: "PEN" | "USD";
  sunatStatus: SunatStatus;
  issuedAt: string; // ISO date
}

export interface Kpi {
  id: string;
  label: string;
  value: number;
  format: "currency" | "integer";
  deltaPercent: number | null;
  hint?: string;
}

export interface DailySales {
  date: string; // ISO date
  total: number;
}

export interface StockAlert {
  id: string;
  productName: string;
  warehouse: string;
  stock: number;
  minimum: number;
  unit?: string;
}

export interface SunatQueueItem {
  id: string;
  documentLabel: string;
  status: Exclude<SunatStatus, "aceptado">;
  message: string;
}

export interface DashboardData {
  tenant: Tenant;
  kpis: Kpi[];
  sales30d: DailySales[];
  recentDocuments: SalesDocument[];
  stockAlerts: StockAlert[];
  sunatQueue: SunatQueueItem[];
}

// ---------- Catalog & POS ----------
export type CatalogKind = "servicio" | "producto";

export interface CatalogItem {
  id: string;
  sku: string;
  name: string;
  kind: CatalogKind;
  category: string;
  price: number; // unit price including IGV
  taxable: boolean;
  stock?: number; // products only
  unit?: string;
  note?: string;
}

export interface CartLine {
  itemId: string;
  sku: string;
  name: string;
  unitPrice: number;
  quantity: number;
  discount: number; // absolute amount per line
  taxable: boolean;
}

export type PaymentMethod = "efectivo" | "tarjeta" | "yape_plin" | "transferencia";

export interface Customer {
  id: string;
  documentType: "DNI" | "RUC" | "CE";
  documentNumber: string;
  name: string;
  phone?: string;
  email?: string;
  segment?: string;
  lastOperation?: string; // ISO date
  billedTotal: number;
  fiscalStatus: "al_dia" | "con_deuda" | "inactivo";
  clinicalRecord?: string;
}

// ---------- Comprobantes (detail) ----------
export interface DocumentLine {
  code: string;
  description: string;
  detail?: string;
  quantity: number;
  unit: string;
  unitValue: number; // sin IGV
  discount: number;
  total: number; // con IGV
}

export interface DocumentEvent {
  at: string; // ISO datetime
  label: string;
  detail?: string;
  tone: "success" | "warning" | "danger" | "neutral";
}

export interface SalesDocumentDetail extends SalesDocument {
  customerAddress?: string;
  customerEmail?: string;
  dueAt?: string;
  paymentTerms: string;
  operationType: string;
  relatedGuide?: string;
  lines: DocumentLine[];
  taxableBase: number;
  igv: number;
  cdr?: { hash: string; acceptedAt?: string; code?: string; message?: string };
  timeline: DocumentEvent[];
  paid: number;
  paymentMethod?: string;
}
