import { CompassIcon } from "@phosphor-icons/react/ssr";
import Link from "next/link";
import { EstadoVacio } from "khipu-design-system/feedback/estado-vacio";
import { buttonVariants } from "khipu-design-system/ui/button";

export default function NotFound() {
  return (
    <EstadoVacio
      icon={CompassIcon}
      titulo="Página no encontrada"
      accion={
        <Link href="/" className={buttonVariants()}>
          Ir al inicio
        </Link>
      }
    >
      La dirección no existe o todavía no está disponible.
    </EstadoVacio>
  );
}
