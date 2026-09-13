"use client";

import { AlertTriangle, Bell, CalendarClock, CheckCircle2, History, PackageX, ShieldCheck } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Tag } from "@/components/shared/status-badge";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";

type Cat = "sunat" | "stock" | "cobranzas" | "seguridad";
interface Notice { id: string; cat: Cat; tone: "success" | "danger" | "warning" | "info"; title: string; text: string; when: string; actions: { label: string; href?: string }[]; read?: boolean }

const initial: Notice[] = [
  { id: "n1", cat: "sunat", tone: "success", title: "Factura F001-00000842 aceptada por SUNAT", text: "CDR N.º 20608941235-01-F001-00000842 · OSE Bizlinks.", when: "hace 5 min", actions: [{ label: "Descargar CDR (XML)" }, { label: "Ver comprobante", href: "/ventas/comprobantes/F001-00000842" }] },
  { id: "n2", cat: "sunat", tone: "danger", title: "Nota de crédito FC01-00000104 observada por OSE", text: "Error 2335: el comprobante de referencia no existe o no corresponde al RUC emisor.", when: "hace 42 min", actions: [{ label: "Subsanar y reintentar", href: "/ventas/comprobantes/FC01-00000104" }, { label: "Ver detalle técnico" }] },
  { id: "n3", cat: "stock", tone: "warning", title: "Stock agotado: Resina Filtek Z350 A2 en Sede San Isidro", text: "Hay disponibilidad en Almacén Miraflores (+18 tubos). Se sugiere generar GRE de transferencia.", when: "hace 2 horas", actions: [{ label: "Generar orden de reposición", href: "/inventario/stock" }] },
  { id: "n4", cat: "cobranzas", tone: "warning", title: "Vence hoy: factura F001-000835 · Consorcio Minero del Centro", text: "Saldo pendiente S/ 2,800.00 (cuota 2/3).", when: "hace 3 horas", actions: [{ label: "Enviar recordatorio WhatsApp" }, { label: "Registrar cobro", href: "/finanzas/cuentas-por-cobrar" }] },
  { id: "n5", cat: "seguridad", tone: "info", title: "Certificado digital SUNAT: 245 días restantes", text: "Caduca el 15/05/2027 · llave RSA-2048 en estado óptimo.", when: "hoy 08:00", actions: [{ label: "Ver configuración", href: "/configuracion/sunat" }] },
];
const icon = { sunat: ShieldCheck, stock: PackageX, cobranzas: CalendarClock, seguridad: ShieldCheck } as const;
const toneClass = { success: "text-success bg-success/10", danger: "text-destructive bg-destructive/10", warning: "text-warning bg-warning/10", info: "text-primary bg-accent" } as const;

export function Notifications() {
  const [items, setItems] = React.useState(initial);
  const [tab, setTab] = React.useState("all");
  const unread = items.filter((i) => !i.read).length;
  const rows = items.filter((i) => tab === "all" || i.cat === tab);
  const count = (c: string) => items.filter((i) => c === "all" || i.cat === c).length;
  return (
    <Popover>
      <PopoverTrigger render={<Button variant="ghost" size="icon" aria-label={`Notificaciones${unread ? ` (${unread} sin leer)` : ""}`} className="relative" />}>
        <Bell className={cn(unread && "motion-safe:animate-[wiggle_1.2s_ease-in-out_1]")} />
        {unread ? <span className="absolute top-1.5 right-1.5 size-2 rounded-full bg-rose-500 ring-2 ring-background" aria-hidden /> : null}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[min(26rem,calc(100vw-2rem))] p-0">
        <div className="flex items-center justify-between gap-2 border-b px-3 py-2"><div><p className="text-sm font-semibold">Notificaciones operativas</p><p className="text-xs text-muted-foreground">{unread} alerta{unread === 1 ? "" : "s"} sin leer</p></div><Button variant="ghost" size="xs" onClick={() => setItems((it) => it.map((i) => ({ ...i, read: true })))}>Marcar leídas</Button></div>
        <div className="border-b px-2 pt-2"><Tabs value={tab} onValueChange={(v) => setTab(String(v))}><TabsList variant="line"><TabsTrigger value="all">Todas <span className="ml-1 font-mono text-[10px] opacity-70">{count("all")}</span></TabsTrigger><TabsTrigger value="sunat">SUNAT / OSE <span className="ml-1 font-mono text-[10px] opacity-70">{count("sunat")}</span></TabsTrigger><TabsTrigger value="stock">Stock <span className="ml-1 font-mono text-[10px] opacity-70">{count("stock")}</span></TabsTrigger><TabsTrigger value="cobranzas">Cobranzas <span className="ml-1 font-mono text-[10px] opacity-70">{count("cobranzas")}</span></TabsTrigger></TabsList></Tabs></div>
        <ul className="max-h-[60svh] divide-y overflow-y-auto">
          {rows.map((n) => { const Icon = n.tone === "success" ? CheckCircle2 : n.tone === "danger" ? AlertTriangle : icon[n.cat]; return (
            <li key={n.id} className={cn("flex gap-3 px-3 py-3", !n.read && "bg-accent/20")}>
              <span className={cn("mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md", toneClass[n.tone])}><Icon className="size-4" strokeWidth={1.5} /></span>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium leading-snug">{n.title}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{n.text}</p>
                <p className="mt-1 text-[11px] text-muted-foreground">{n.when}{!n.read ? <Tag tone="primary" label="Nueva" className="ml-2" /> : null}</p>
                <div className="mt-2 flex flex-wrap gap-1.5">{n.actions.map((a) => a.href ? <Button key={a.label} size="xs" variant="outline" render={<Link href={a.href} />} nativeButton={false}>{a.label}</Button> : <Button key={a.label} size="xs" variant="ghost" onClick={() => toast.info(a.label)}>{a.label}</Button>)}</div>
              </div>
            </li>
          ); })}
        </ul>
        <div className="border-t px-3 py-2"><Button variant="link" size="sm" className="px-0" render={<Link href="/configuracion/auditoria" />} nativeButton={false}><History data-icon="inline-start" /> Ver historial completo y auditoría</Button></div>
      </PopoverContent>
    </Popover>
  );
}
