export const sunatConfig = {
  company: { name: "CLÍNICA DENTAL SONRISA S.A.C.", ruc: "20608941235", address: "Av. Larco 743, Of. 402 – Miraflores, Lima", ubigeo: "150122", regime: "Régimen MYPE Tributario" },
  certificate: { issuer: "Llama.pe Trust CA (acreditada por INDECOPI)", serial: "5A:4F:98:C1:20:EE:99:B7", validUntil: "2027-05-14", holderRuc: "20608941235", sha256: "7E:3B:5A:F1:C9:82:4D:B0:EA:5C:31:12:F4:79:AD:04:1E:58:8D:19:6F:02:44:A8:DC:12:87:CC:01:FE:A9:63", storage: "HSM cloud" },
  connection: { channel: "ose", oseProvider: "Bizlinks", solUser: "MODDATOS", environment: "produccion", lastHandshake: "2026-09-12T11:42:00", latencyMs: 240 },
  series: [
    { type: "01 Factura", series: "F001", site: "Miraflores", last: 842 },
    { type: "03 Boleta", series: "B001", site: "Miraflores", last: 4291 },
    { type: "07 Nota de crédito", series: "FC01", site: "Miraflores", last: 104 },
    { type: "09 Guía de remisión", series: "T001", site: "Miraflores", last: 145 },
    { type: "01 Factura", series: "F002", site: "San Isidro", last: 118 },
    { type: "03 Boleta", series: "B002", site: "San Isidro", last: 1290 },
  ],
  print: { format: "a4", legend: "Representación impresa de la factura electrónica. Consulte su comprobante en sunat.gob.pe" },
};

export interface Collaborator {
  id: string;
  name: string;
  email: string;
  identity: string;
  role: string;
  sites: string;
  twoFa: "authenticator" | "sms" | "pendiente";
  lastAccess: string;
  ip: string;
  status: "activo" | "suspendido";
}

export const securitySummary = { total: 18, active: 16, pending2fa: 3, sessions: 7, maxSessions: 12, sites: 3, incidents48h: 0, failedAttempts: 4 };

export const roles = ["Super Admin", "Médico odontólogo", "Cajero POS / Recepción", "Administrador de sede", "Contador externo / Auditor"];

export const collaboratorsMock: Collaborator[] = [
  { id: "c1", name: "Dra. Valeria Mendoza Rivas", email: "valeria.mendoza@sonrisaperu.pe", identity: "COP 41829", role: "Super Admin", sites: "Todas las sedes (3)", twoFa: "authenticator", lastAccess: "Hoy 10:42", ip: "190.237.44.12 · San Isidro", status: "activo" },
  { id: "c2", name: "Lucía Espinoza Salcedo", email: "caja.miraflores@sonrisaperu.pe", identity: "DNI 72449102", role: "Cajero POS / Recepción", sites: "Sede Miraflores · Caja 01", twoFa: "sms", lastAccess: "Hoy 08:15", ip: "181.67.112.4 · Miraflores", status: "activo" },
  { id: "c3", name: "Lic. Roberto Benavides Paucar", email: "roberto.benavides@sonrisaperu.pe", identity: "DNI 41209983", role: "Administrador de sede", sites: "Sede San Isidro", twoFa: "authenticator", lastAccess: "Ayer 19:02", ip: "200.48.10.77 · San Isidro", status: "activo" },
  { id: "c4", name: "Dr. Miguel Flores Tapia", email: "miguel.flores@sonrisaperu.pe", identity: "COP 39012", role: "Médico odontólogo", sites: "Miraflores · San Isidro", twoFa: "pendiente", lastAccess: "Hoy 09:30", ip: "190.237.44.12 · Miraflores", status: "activo" },
  { id: "c5", name: "CPC Ana María Ríos Castro", email: "contabilidad@estudiorios.pe", identity: "DNI 09877123", role: "Contador externo / Auditor", sites: "Solo lectura (3)", twoFa: "authenticator", lastAccess: "08/09/2026", ip: "38.25.19.140 · Lima", status: "activo" },
  { id: "c6", name: "Renato Cano Vega", email: "higiene@sonrisaperu.pe", identity: "DNI 74561209", role: "Médico odontólogo", sites: "Sede Miraflores", twoFa: "pendiente", lastAccess: "Hoy 07:58", ip: "181.67.112.4 · Miraflores", status: "activo" },
  { id: "c7", name: "Karla Huamán Soto", email: "recepcion.si@sonrisaperu.pe", identity: "DNI 70998812", role: "Cajero POS / Recepción", sites: "Sede San Isidro · Caja 02", twoFa: "pendiente", lastAccess: "03/09/2026", ip: "200.48.10.77 · San Isidro", status: "suspendido" },
];

export const permissionModules = ["Ventas y POS", "Comprobantes SUNAT", "Clientes e historia clínica", "Inventario y almacenes", "Compras y proveedores", "Caja y arqueo", "Reportes tributarios", "Configuración y seguridad"];
export const permissionActions = ["Ver", "Crear", "Editar", "Anular", "Exportar"] as const;

/** matrix[role][module] = set of allowed actions */
export const permissionMatrix: Record<string, Record<string, ReadonlyArray<(typeof permissionActions)[number]>>> = {
  "Super Admin": Object.fromEntries(permissionModules.map((m) => [m, permissionActions])),
  "Médico odontólogo": { "Ventas y POS": ["Ver", "Crear"], "Comprobantes SUNAT": ["Ver"], "Clientes e historia clínica": ["Ver", "Crear", "Editar"], "Inventario y almacenes": ["Ver"], "Compras y proveedores": [], "Caja y arqueo": [], "Reportes tributarios": [], "Configuración y seguridad": [] },
  "Cajero POS / Recepción": { "Ventas y POS": ["Ver", "Crear", "Editar"], "Comprobantes SUNAT": ["Ver", "Crear"], "Clientes e historia clínica": ["Ver", "Crear"], "Inventario y almacenes": ["Ver"], "Compras y proveedores": [], "Caja y arqueo": ["Ver", "Crear"], "Reportes tributarios": [], "Configuración y seguridad": [] },
  "Administrador de sede": { "Ventas y POS": ["Ver", "Crear", "Editar", "Anular", "Exportar"], "Comprobantes SUNAT": ["Ver", "Crear", "Anular", "Exportar"], "Clientes e historia clínica": ["Ver", "Crear", "Editar", "Exportar"], "Inventario y almacenes": ["Ver", "Crear", "Editar", "Exportar"], "Compras y proveedores": ["Ver", "Crear", "Editar"], "Caja y arqueo": ["Ver", "Crear", "Editar", "Exportar"], "Reportes tributarios": ["Ver", "Exportar"], "Configuración y seguridad": ["Ver"] },
  "Contador externo / Auditor": { "Ventas y POS": ["Ver"], "Comprobantes SUNAT": ["Ver", "Exportar"], "Clientes e historia clínica": [], "Inventario y almacenes": ["Ver", "Exportar"], "Compras y proveedores": ["Ver", "Exportar"], "Caja y arqueo": ["Ver", "Exportar"], "Reportes tributarios": ["Ver", "Exportar"], "Configuración y seguridad": [] },
};
