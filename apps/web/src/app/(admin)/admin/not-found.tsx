import { MagnifyingGlassIcon } from "@phosphor-icons/react/ssr";
import Link from "next/link";
import { buttonVariants } from "khipu-design-system/ui/button";
import { EstadoVacio } from "khipu-design-system/feedback/estado-vacio";

/** 404 of the admin console: renders inside the console layout (admin shell, indigo palette). */
export default function AdminNotFound() {
  return (
    <div className="flex flex-1 items-center justify-center p-4 md:p-6">
      <EstadoVacio
        icon={MagnifyingGlassIcon}
        titulo="No encontramos esta página de la consola"
        accion={<Link href="/admin" className={buttonVariants()}>Ir al dashboard de la consola</Link>}
      >
        La ruta no existe o fue movida. Vuelve al dashboard o usa el menú lateral.
      </EstadoVacio>
    </div>
  );
}
