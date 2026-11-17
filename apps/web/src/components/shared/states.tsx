import { LockIcon, MagnifyingGlassIcon, ShieldWarningIcon, TimerIcon, TrayIcon, WarningIcon, ArrowClockwiseIcon } from "@phosphor-icons/react/dist/ssr";
import Link from "next/link";
import type { ReactNode } from "react";
import { EstadoVacio } from "khipu-design-system/feedback/estado-vacio";
import { Skeleton, SkeletonMetricas, SkeletonTabla } from "khipu-design-system/feedback/skeleton";
import { BloqueCodigo } from "khipu-design-system/patrones/bloque-codigo";
import { Button } from "khipu-design-system/ui/button";

/*
 * @deprecated Temporary W5a adapters over `EstadoVacio` / `Skeleton`. Module PRs compose the DS components directly; deleted in W8.
 * Owner decision (ADR 0010): NotFound/Forbidden/SessionExpired/Error are composed here with `EstadoVacio`; the tone tint of the old icon box is gone.
 */

export function EmptyState(p: { title: string; text?: string; action?: ReactNode; className?: string }) {
  return <EstadoVacio icon={TrayIcon} titulo={p.title} accion={p.action} className={p.className}>{p.text}</EstadoVacio>;
}

export function ErrorState(p: { title?: string; text?: string; detail?: string; onRetry?: () => void }) {
  const accion =
    p.onRetry || p.detail ? (
      <div className="flex flex-col items-center gap-3">
        {p.onRetry ? <Button onClick={p.onRetry}><ArrowClockwiseIcon data-icon="inline-start" /> Reintentar</Button> : null}
        {p.detail ? (
          <details className="w-full max-w-md text-left">
            <summary className="cursor-pointer text-center">Detalle técnico</summary>
            <BloqueCodigo codigo={p.detail} tono="claro" alturaMax="16rem" />
          </details>
        ) : null}
      </div>
    ) : undefined;
  return (
    <EstadoVacio icon={WarningIcon} titulo={p.title ?? "Error de sincronización con SUNAT"} accion={accion}>
      {p.text ?? "No pudimos completar la operación. Reintenta; si persiste, revisa el detalle técnico."}
    </EstadoVacio>
  );
}

export function NotFoundState() {
  return (
    <EstadoVacio
      icon={MagnifyingGlassIcon}
      titulo="No encontramos esta página"
      accion={
        <div className="flex flex-wrap justify-center gap-2">
          <Button render={<Link href="/" />} nativeButton={false}>Ir al panel</Button>
          <Button variant="outline" render={<Link href="/ventas/comprobantes" />} nativeButton={false}>Comprobantes</Button>
          <Button variant="outline" render={<Link href="/clientes" />} nativeButton={false}>Clientes</Button>
        </div>
      }
    >
      La ruta no existe o fue movida. Usa la búsqueda (Ctrl+K) o vuelve al panel.
    </EstadoVacio>
  );
}

export function ForbiddenState({ role = "Cajero POS", module = "Reportes tributarios" }: { role?: string; module?: string }) {
  return (
    <EstadoVacio
      icon={ShieldWarningIcon}
      titulo="Sin permisos para este módulo"
      accion={
        <div className="flex flex-wrap justify-center gap-2">
          <Button render={<Link href="/configuracion/usuarios" />} nativeButton={false}><LockIcon data-icon="inline-start" /> Solicitar acceso</Button>
          <Button variant="outline" render={<Link href="/" />} nativeButton={false}>Volver al panel</Button>
        </div>
      }
    >
      {`Tu rol ${role} no tiene acceso a ${module}. Solicita acceso al administrador de tu sede.`}
    </EstadoVacio>
  );
}

export function SessionExpiredState() {
  return (
    <EstadoVacio icon={TimerIcon} titulo="Tu sesión expiró" accion={<Button render={<Link href="/login" />} nativeButton={false}>Iniciar sesión</Button>}>
      Por seguridad cerramos la sesión tras 30 minutos de inactividad.
    </EstadoVacio>
  );
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
