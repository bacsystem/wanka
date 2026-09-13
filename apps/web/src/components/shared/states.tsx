import { AlertTriangle, Inbox, Lock, RefreshCw, SearchX, ShieldAlert, TimerOff } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

function Shell({ icon: Icon, title, text, children, tone = "neutral", className }: { icon: typeof Inbox; title: string; text?: string; children?: ReactNode; tone?: "neutral" | "danger" | "warning"; className?: string }) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-3 px-6 py-12 text-center", className)}>
      <span className={cn("flex size-12 items-center justify-center rounded-xl", tone === "danger" ? "bg-destructive/10 text-destructive" : tone === "warning" ? "bg-warning/10 text-warning" : "bg-accent text-accent-foreground")}><Icon className="size-6" strokeWidth={1.5} /></span>
      <div><h1 className="text-base font-semibold">{title}</h1>{text ? <p className="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">{text}</p> : null}</div>
      {children ? <div className="flex flex-wrap justify-center gap-2">{children}</div> : null}
    </div>
  );
}

export function EmptyState(p: { title: string; text?: string; action?: ReactNode; className?: string }) { return <Shell icon={Inbox} title={p.title} text={p.text} className={p.className}>{p.action}</Shell>; }
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
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-28 rounded-lg" />)}</div>
      <div className="rounded-lg border"><Skeleton className="w-full rounded-b-none" />{Array.from({ length: 6 }).map((_, i) => <div key={i} className="flex gap-3 border-t p-3"><Skeleton className="h-4 w-24" /><Skeleton className="h-4 flex-1" /><Skeleton className="h-4 w-16" /><Skeleton className="h-4 w-20" /></div>)}</div>
    </div>
  );
}
