/** Datos de sesión y entorno simulados hasta conectar el backend real. */
export const DEMO_TENANT_USER = {
  email: "carlos.mendoza@clinicasonrisa.pe",
  name: "Carlos Mendoza",
  initials: "CM",
  defaultRuc: "20608941235",
  defaultCompanyName: "Clínica Dental Sonrisa S.A.C.",
  defaultDomain: "dentalsonrisa.pe",
  defaultWorkspaceLabel: "Clínica Dental Sonrisa – Sede Central",
} as const;

export const DEMO_ADMIN_USER = {
  email: "c.benavides@perusaas.pe",
  domain: "@perusaas.pe",
  name: "Carlos M. Benavides",
  initials: "CB",
  role: "SRE / SuperAdmin N3",
  cluster: "prod-lima-01",
  sla: "Producción · SLA 99.98%",
  region: "PE-Lima",
  auditIp: "190.237.44.12",
} as const;
