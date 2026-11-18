"use client";

import { CalendarBlank, Package, ShieldCheck } from "@phosphor-icons/react";
import * as React from "react";
import { toast } from "sonner";
import { CentroNotificaciones, type AccionNotificacion, type Notificacion } from "khipu-design-system/nav/centro-notificaciones";

const categorias = [
  { id: "sunat", etiqueta: "SUNAT / OSE" },
  { id: "stock", etiqueta: "Stock" },
  { id: "cobranzas", etiqueta: "Cobranzas" },
  { id: "seguridad", etiqueta: "Seguridad" },
];

/** An action with a destination is a link; one without it only confirms with a toast (mock-backed, no API yet). */
const action = (etiqueta: string, href?: string): AccionNotificacion => (href ? { etiqueta, href } : { etiqueta, onClick: () => toast.info(etiqueta) });

const initial: Notificacion[] = [
  { id: "n1", categoria: "sunat", tono: "ok", titulo: "Factura F001-00000842 aceptada por SUNAT", descripcion: "CDR N.º 20608941235-01-F001-00000842 · OSE Bizlinks.", fecha: "2026-09-13T13:55:00Z", leida: false, acciones: [action("Descargar CDR (XML)"), action("Ver comprobante", "/ventas/comprobantes/F001-00000842")] },
  { id: "n2", categoria: "sunat", tono: "error", titulo: "Nota de crédito FC01-00000104 observada por OSE", descripcion: "Error 2335: el comprobante de referencia no existe o no corresponde al RUC emisor.", fecha: "2026-09-13T13:18:00Z", leida: false, acciones: [action("Subsanar y reintentar", "/ventas/comprobantes/FC01-00000104"), action("Ver detalle técnico")] },
  { id: "n3", categoria: "stock", tono: "aviso", icono: Package, titulo: "Stock agotado: Resina Filtek Z350 A2 en Sede San Isidro", descripcion: "Hay disponibilidad en Almacén Miraflores (+18 tubos). Se sugiere generar GRE de transferencia.", fecha: "2026-09-13T12:00:00Z", leida: false, acciones: [action("Generar orden de reposición", "/inventario/stock")] },
  { id: "n4", categoria: "cobranzas", tono: "aviso", icono: CalendarBlank, titulo: "Vence hoy: factura F001-000835 · Consorcio Minero del Centro", descripcion: "Saldo pendiente S/ 2,800.00 (cuota 2/3).", fecha: "2026-09-13T11:00:00Z", leida: false, acciones: [action("Enviar recordatorio WhatsApp"), action("Registrar cobro", "/finanzas/cuentas-por-cobrar")] },
  { id: "n5", categoria: "seguridad", tono: "neutro", icono: ShieldCheck, titulo: "Certificado digital SUNAT: 245 días restantes", descripcion: "Caduca el 15/05/2027 · llave RSA-2048 en estado óptimo.", fecha: "2026-09-13T13:00:00Z", leida: false, acciones: [action("Ver configuración", "/configuracion/sunat")] },
];

export function Notifications() {
  const [items, setItems] = React.useState(initial);
  return (
    <CentroNotificaciones
      notificaciones={items}
      categorias={categorias}
      onLeer={(id) => setItems((prev) => prev.map((n) => (n.id === id ? { ...n, leida: true } : n)))}
      onLeerTodas={() => {
        setItems((prev) => prev.map((n) => ({ ...n, leida: true })));
        toast.success("Notificaciones marcadas como leídas");
      }}
      verTodas={{ href: "/configuracion/auditoria", etiqueta: "Ver historial completo y auditoría" }}
      textos={{ titulo: "Notificaciones operativas" }}
    />
  );
}
