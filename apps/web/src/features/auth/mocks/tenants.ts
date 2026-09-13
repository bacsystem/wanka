import type { Industry, Tenant } from "@/types/domain";

export interface TenantOption extends Tenant {
  role: string;
  sites: number;
  sunat: { label: string; tone: "success" | "warning" };
  certificateDays: number;
  lastAccess: string;
  industryLabel: string;
}

export const tenantOptions: TenantOption[] = [
  { id: "t_001", name: "Clínica Dental Sonrisa S.A.C.", ruc: "20608941235", industry: "odontologia", industryLabel: "Odontología y salud", role: "Administrador general", sites: 2, sunat: { label: "OSE directo OK", tone: "success" }, certificateDays: 245, lastAccess: "Hoy, 15:42" },
  { id: "t_002", name: "Veterinaria San Borja E.I.R.L.", ruc: "20512345678", industry: "veterinaria", industryLabel: "Clínica de mascotas", role: "Cajero POS", sites: 1, sunat: { label: "CDR conectado", tone: "success" }, certificateDays: 12, lastAccess: "Ayer, 18:40" },
  { id: "t_004", name: "Cevichería El Muelle S.A.C.", ruc: "20512345670", industry: "restaurante", industryLabel: "Restaurante y bar", role: "Capitán de salón", sites: 1, sunat: { label: "OSE conectado", tone: "success" }, certificateDays: 300, lastAccess: "Hoy, 13:10" },
  { id: "t_003", name: "Bodega Don Pepe", ruc: "10456789012", industry: "retail", industryLabel: "Retail / minimarket", role: "Propietario master", sites: 1, sunat: { label: "SUNAT directo", tone: "success" }, certificateDays: 189, lastAccess: "Hace 3 días" },
];

export const tenantSites: Record<string, { id: string; code: string; name: string; address: string; facts: string[]; cashboxes: { id: string; name: string; status: "abierta" | "cerrada"; detail: string }[] }[]> = {
  t_001: [
    { id: "s1", code: "SEDE-01", name: "Sede Miraflores (principal)", address: "Av. José Larco 850, Piso 3 · Miraflores, Lima", facts: ["3 sillones", "Almacén central", "4 terminales"], cashboxes: [
      { id: "c1", name: "CAJA-01 · Recepción principal · turno mañana", status: "abierta", detail: "Abierta hoy 08:30 · apertura Claudia Ramos · fondo S/ 300.00 · efectivo S/ 850.00" },
      { id: "c2", name: "CAJA-02 · Box quirúrgico Odonto-2", status: "cerrada", detail: "Turno no iniciado · se solicitará arqueo inicial (sugerido S/ 200.00)" },
    ] },
    { id: "s2", code: "SEDE-02", name: "Sede San Isidro", address: "Av. Conquistadores 420 · San Isidro, Lima", facts: ["2 sillones", "Subalmacén", "2 terminales"], cashboxes: [
      { id: "c3", name: "CAJA-03 · Recepción San Isidro", status: "cerrada", detail: "Turno no iniciado" },
    ] },
  ],
  t_002: [{ id: "s3", code: "SEDE-01", name: "Sede San Borja", address: "Av. San Borja Norte 1120 · San Borja, Lima", facts: ["2 consultorios", "Petshop", "1 terminal"], cashboxes: [{ id: "c4", name: "CAJA-01 · Mostrador", status: "abierta", detail: "Abierta hoy 09:00 · fondo S/ 150.00" }] }],
  t_004: [{ id: "s5", code: "SEDE-01", name: "Sede Miraflores (Malecón)", address: "Malecón de la Reserva 610 · Miraflores, Lima", facts: ["22 mesas", "Cocina y barra", "3 terminales"], cashboxes: [{ id: "c6", name: "Caja salón 01", status: "abierta", detail: "Turno almuerzo 12:00–17:00 · fondo S/ 200.00" }] }],
  t_003: [{ id: "s4", code: "SEDE-01", name: "Tienda principal", address: "Jr. Los Pinos 455 · Surquillo, Lima", facts: ["1 caja", "Almacén trastienda"], cashboxes: [{ id: "c5", name: "CAJA-01 · Mostrador", status: "cerrada", detail: "Turno no iniciado" }] }],
};

export const industryOptions: { id: Industry; label: string; description: string; tagline: string; recommended?: boolean }[] = [
  { id: "odontologia", label: "Odontología y salud", description: "Gestión clínica con historia médica, odontograma interactivo, convenios EPS, liquidación médica y recetas.", tagline: "Plantillas dentales activas", recommended: true },
  { id: "veterinaria", label: "Veterinaria y petshop", description: "Historias clínicas veterinarias, control de vacunas y desparasitaciones, citas de grooming y venta por peso y código.", tagline: "Control de mascotas" },
  { id: "restaurante", label: "Restaurante, café y bar", description: "Control de salones y mesas, comandas a cocina y barra, delivery, división de cuentas y mermas de ingredientes.", tagline: "Comanderas e inventario" },
  { id: "retail", label: "Retail y servicios generales", description: "Venta rápida POS con lector de barras, inventario con variantes (talla/color), listas de precios y cotizaciones B2B.", tagline: "Venta mostrador rápida" },
];

export const modulesByIndustry: Record<Industry, { id: string; name: string; description: string; tag?: string; on: boolean }[]> = {
  odontologia: [
    { id: "agenda", name: "Agenda de citas y turnos médicos", description: "Calendario por sillón, confirmación por WhatsApp y triaje previo.", tag: "Recomendado", on: true },
    { id: "hc", name: "Historias clínicas y odontograma digital", description: "Mapeo FDI con evolución de tratamientos, presupuesto clínico y consentimientos.", tag: "Esencial", on: true },
    { id: "cpe", name: "Facturación electrónica directa SUNAT / OSE", description: "Boletas, facturas y notas con CDR automático, QR tributario y reportes SIRE.", tag: "SUNAT UBL 2.1", on: true },
    { id: "inv", name: "Inventario de insumos médicos, lotes y vencimientos", description: "Kardex de resinas, anestésicos e instrumental con alerta temprana de expiración.", on: true },
    { id: "eps", name: "Convenios corporativos y copago EPS", description: "Liquidación diferenciada para Rímac, Pacífico, Sanitas y Mapfre.", on: true },
    { id: "cxc", name: "Cuentas por cobrar y cobranza fraccionada", description: "Cuotas para ortodoncia e implantes con registro múltiple (Yape, Plin, POS).", on: true },
    { id: "mesas", name: "Control de mesas y comandas gastronómicas", description: "No aplica comúnmente a clínicas (actívalo si ofreces cafetería).", tag: "Inactivo", on: false },
  ],
  veterinaria: [
    { id: "agenda", name: "Agenda de citas y grooming", description: "Calendario por consultorio y sala de baño.", tag: "Recomendado", on: true },
    { id: "mascotas", name: "Historia clínica veterinaria y carné de vacunas", description: "Ficha por mascota, vacunas, desparasitaciones y recordatorios.", tag: "Esencial", on: true },
    { id: "cpe", name: "Facturación electrónica SUNAT / OSE", description: "Boletas y facturas con CDR automático.", on: true },
    { id: "inv", name: "Inventario de alimentos, fármacos y accesorios", description: "Venta por peso y código de barras, lotes y vencimientos.", on: true },
    { id: "cxc", name: "Cuentas por cobrar", description: "Planes de tratamiento en cuotas.", on: false },
  ],
  restaurante: [
    { id: "mesas", name: "Control de mesas y comandas", description: "Salones, mesas, comandas a cocina y barra, división de cuentas.", tag: "Esencial", on: true },
    { id: "cpe", name: "Facturación electrónica SUNAT / OSE", description: "Boletas y facturas desde la mesa o el mostrador.", on: true },
    { id: "inv", name: "Inventario de insumos y recetas", description: "Descarga de ingredientes por plato y control de mermas.", on: true },
    { id: "delivery", name: "Delivery y pedidos", description: "Pedidos para llevar, repartidores y seguimiento.", on: true },
    { id: "agenda", name: "Reservas", description: "Reservas de mesas por horario.", on: false },
  ],
  retail: [
    { id: "pos", name: "Venta rápida POS con lector de barras", description: "Mostrador, variantes de talla y color, promociones.", tag: "Esencial", on: true },
    { id: "cpe", name: "Facturación electrónica SUNAT / OSE", description: "Boletas y facturas con CDR automático.", on: true },
    { id: "inv", name: "Inventario multialmacén y kardex", description: "Existencias, transferencias con GRE y kardex 13.1.", on: true },
    { id: "b2b", name: "Cotizaciones B2B y listas de precios", description: "Precios mayoristas y proformas.", on: true },
    { id: "cxc", name: "Cuentas por cobrar", description: "Crédito a clientes mayoristas.", on: false },
  ],
};
