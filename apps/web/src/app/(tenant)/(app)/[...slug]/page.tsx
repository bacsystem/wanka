import { notFound } from "next/navigation";

/**
 * Destino de lo no habilitado: `proxy.ts` reescribe aquí toda URL que no está en la lista de `config/navigation.ts`.
 * También atrapa cualquier ruta sin pantalla. Siempre responde 404.
 */
export default function DisabledRoute() {
  notFound();
}
