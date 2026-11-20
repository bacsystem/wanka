import { randomBytes } from "node:crypto";
import { seedLoginUsers } from "./login-seed";

/**
 * Los E2E del login corren contra la API real (`make up`; docs/local-environment.md). Si la API no responde (CI sin pila
 * local), no se siembra nada y esos specs se saltan. Si responde, se siembran los usuarios con una contraseña aleatoria de
 * esta corrida (`E2E_LOGIN_PASSWORD`, la heredan los workers): no hay credenciales en el repositorio.
 */
export default async function globalSetup(): Promise<void> {
  const apiUrl = (process.env.WANKA_API_URL ?? "http://127.0.0.1:58080").replace(/\/+$/, "");
  const ready = await fetch(`${apiUrl}/readyz`, { signal: AbortSignal.timeout(2000) }).then(
    (r) => r.ok,
    () => false,
  );
  if (!ready) {
    console.log(`E2E login: API not reachable at ${apiUrl}; login specs will be skipped.`);
    return;
  }
  const password = `E2e-${randomBytes(12).toString("base64url")}`;
  seedLoginUsers(password, process.env.E2E_DB_CONTAINER ?? "wanka-local-pg");
  process.env.WANKA_API_URL = apiUrl;
  process.env.E2E_LOGIN_PASSWORD = password;
}
