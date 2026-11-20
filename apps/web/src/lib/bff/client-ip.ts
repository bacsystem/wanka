import { isIP } from "node:net";

/**
 * IP del navegador que el BFF le pone a la API en `X-Client-Ip` (docs/architecture/api/conventions.md §3).
 *
 * Toma la entrada MÁS A LA DERECHA de `X-Forwarded-For`: es la que agrega el borde de Railway; las anteriores las puede
 * escribir el navegador y no se usan nunca. Sin borde (local), Next pone en `X-Forwarded-For` la IP del socket cuando la
 * petición no la trae, así que la regla es la misma.
 *
 * Límite conocido: en local Next hace `x-forwarded-for ??= <socket>`, es decir, solo pone la IP del socket cuando la
 * petición NO trae la cabecera. Si el navegador manda su propio `X-Forwarded-For`, ese valor queda como IP (no se puede
 * distinguir sin el socket, que un route handler no ve). Detrás del borde de Railway no pasa: el borde agrega la IP real
 * como última entrada y aquí solo se lee la última. En local eso no es un riesgo de seguridad: la API ignora la
 * cabecera salvo con `API_TRUST_CLIENT_IP=true`, que solo se fija donde el BFF es su único cliente (conventions.md §3).
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
