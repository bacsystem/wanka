import { isIP } from "node:net";

/**
 * IP del navegador que el BFF le pone a la API en `X-Client-Ip` (docs/architecture/api/conventions.md §3).
 *
 * Toma la entrada MÁS A LA DERECHA de `X-Forwarded-For`: es la que agrega el borde de Railway; las anteriores las puede
 * escribir el navegador y no se usan nunca. Sin borde (local), Next pone en `X-Forwarded-For` la IP del socket cuando la
 * petición no la trae, así que la regla es la misma.
 *
 * Devuelve una sola dirección IPv4 o IPv6 literal, sin puerto, zona ni lista. Si falta o no es válida devuelve `null`:
 * es mejor no enviar la cabecera (la API cuenta la IP como «desconocida») que enviar algo que no se verificó.
 */
export function resolveClientIp(headers: Headers): string | null {
  const forwarded = headers.get("x-forwarded-for");
  if (!forwarded) return null;
  const last = forwarded.split(",").at(-1)?.trim();
  if (!last || last.includes("%")) return null;
  return isIP(last) === 0 ? null : last;
}
