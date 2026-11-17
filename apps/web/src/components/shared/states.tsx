import { AlertTriangle, Lock, RefreshCw, SearchX, ShieldAlert, TimerOff, type LucideIcon } from "lucide-react";
import { TrayIcon } from "@phosphor-icons/react/dist/ssr";
import Link from "next/link";
import type { ReactNode } from "react";
import { EstadoVacio } from "khipu-design-system/feedback/estado-vacio";
import { Skeleton, SkeletonMetricas, SkeletonTabla } from "khipu-design-system/feedback/skeleton";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/*
 * @deprecated Temporary W5a adapters; module PRs compose the DS components directly. Deleted in W8.
 * EmptyState and LoadingState render DS components only. The four page-level states (Error, NotFound, Forbidden, SessionExpired) keep their
 * local layout because `EstadoVacio` renders its title as an `h3` and E2E/a11y need the page `h1` (DS gap: heading level), and it has no tone.
 */

function Shell({ icon: Icon, title, text, children, tone = "neutral", className }: { icon: LucideIcon; title: string; text?: string; children?: ReactNode; tone?: "neutral" | "danger" | "warning"; className?: string }) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-3 px-6 py-12 text-center", className)}>
      <span className={cn("flex size-12 items-center justify-center rounded-xl", tone === "danger" ? "bg-destructive/10 text-destructive" : tone === "warning" ? "bg-warning/10 text-warning" : "bg-accent text-accent-foreground")}><Icon className="size-6" strokeWidth={1.5} /></span>
      <div><h1 className="text-base font-semibold">{title}</h1>{text ? <p className="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">{text}</p> : null}</div>
      {children ? <div className="flex flex-wrap justify-center gap-2">{children}</div> : null}
    </div>
  );
}

export function EmptyState(p: { title: string; text?: string; action?: ReactNode; className?: string }) {
  return <EstadoVacio icon={TrayIcon} titulo={p.title} accion={p.action} className={p.className}>{p.text}</EstadoVacio>;
}
export function ErrorState(p: { title?: string; text?: string; detail?: string; onRetry?: () => void }) {
  return (
    <Shell icon={AlertTriangle} tone="danger" title={p.title ?? "Error de sincronización con SUNAT"} text={p.text ?? "No pudimos completar la operación. Reintenta; si persiste, revisa el detalle técnico."}>
      {p.onRetry ? <Button onClick={p.onRetry}><RefreshCw data-icon="inline-start" /> Reintentar</Button> : null}
      {p.detail ? <details className="w-full max-w-md text-left text-xs text-muted-foreground"><summary className="cursor-pointer text-center">Detalle técnico</summary><pre className="mt-2 overflow-x-auto rounded-md bg-muted p-2 font-mono">{p.detail}</pre></details> : null}
    </Shell>
  );
}
export function NotFoundState() {
  return <Shell icon={SearchX} title="No encontramos esta página" text="La ruta no existe o fue movida. Usa la búsqueda (Ctrl+K) o vuelve al panel."><Button render={<Link href="/" />} nativeButton={false}>Ir al panel</Button><Button variant="outline" render={<Link href="/ventas/comprobantes" />} nativeButton={false}>Comprobantes</Button><Button variant="outline" render={<Link href="/clientes" />} nativeButton={false}>Clientes</Button></Shell>;
}
export function ForbiddenState({ role = "Cajero POS", module = "Reportes tributarios" }: { role?: string; module?: string }) {
  return <Shell icon={ShieldAlert} tone="warning" title="Sin permisos para este módulo" text={`Tu rol ${role} no tiene acceso a ${module}. Solicita acceso al administrador de tu sede.`}><Button render={<Link href="/configuracion/usuarios" />} nativeButton={false}><Lock data-icon="inline-start" /> Solicitar acceso</Button><Button variant="outline" render={<Link href="/" />} nativeButton={false}>Volver al panel</Button></Shell>;
}
export function SessionExpiredState() {
  return <Shell icon={TimerOff} title="Tu sesión expiró" text="Por seguridad cerramos la sesión tras 30 minutos de inactividad."><Button render={<Link href="/login" />} nativeButton={false}>Iniciar sesión</Button></Shell>;
}

export function LoadingState() {
  return (
    <div className="flex flex-col gap-4 p-4 md:p-6" aria-busy aria-label="Cargando">
      <div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-6 w-64" /></div>
      <SkeletonMetricas />
      <SkeletonTabla filas={6} />
    </div>
  );
}
