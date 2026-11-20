// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { handleLogin, isSameOrigin, parseRetryAfter } from "./login-handler";

const AT = `wk_at_${"a".repeat(43)}`;
const RT = `wk_rt_${"b".repeat(43)}`;
const CH = `wk_ch_${"c".repeat(43)}`;
const EN = `wk_en_${"d".repeat(43)}`;
const PASSWORD = "Sup3r-secreta!";

function login(body: unknown, headers: Record<string, string> = {}) {
  return new Request("http://localhost:3000/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json", host: "localhost:3000", origin: "http://localhost:3000", "x-forwarded-for": "1.1.1.1, 203.0.113.9", ...headers },
    body: typeof body === "string" ? body : JSON.stringify(body),
  });
}

const apiJson = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });

describe("POST /api/auth/login (BFF)", () => {
  const fetchMock = vi.fn<typeof fetch>();
  const errorLog = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const infoLog = vi.spyOn(console, "log").mockImplementation(() => undefined);

  beforeEach(() => {
    vi.stubEnv("WANKA_API_URL", "http://api.internal:8080");
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    fetchMock.mockReset();
    errorLog.mockClear();
    infoLog.mockClear();
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("llama a /v1/auth/login con X-Client-Ip (última entrada de X-Forwarded-For) y sin Authorization ni Cookie", async () => {
    fetchMock.mockResolvedValue(apiJson(401, { code: "invalid_credentials" }));
    await handleLogin(login({ email: " Ana@Empresa.pe ", password: PASSWORD }, { cookie: "x=y", authorization: "Bearer z", "x-client-ip": "9.9.9.9" }));
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://api.internal:8080/v1/auth/login");
    const headers = new Headers(init?.headers);
    expect(headers.get("x-client-ip")).toBe("203.0.113.9");
    expect(headers.has("cookie")).toBe(false);
    expect(headers.has("authorization")).toBe(false);
    expect(headers.has("idempotency-key")).toBe(false);
    expect(JSON.parse(String(init?.body))).toEqual({ email: "Ana@Empresa.pe", password: PASSWORD });
  });

  it("authenticated: tokens en cookies httpOnly SameSite=Lax y nada de ellos en el cuerpo", async () => {
    const expiresAt = new Date(Date.now() + 15 * 60_000).toISOString();
    fetchMock.mockResolvedValue(apiJson(200, { status: "authenticated", accessToken: AT, refreshToken: RT, expiresAt, memberships: [] }));
    const res = await handleLogin(login({ email: "ana@empresa.pe", password: PASSWORD }));
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ status: "authenticated" });
    const at = res.cookies.get("wanka_at");
    const rt = res.cookies.get("wanka_rt");
    expect(at).toMatchObject({ value: AT, httpOnly: true, sameSite: "lax", path: "/" });
    expect(at?.maxAge).toBeGreaterThan(14 * 60);
    expect(at?.maxAge).toBeLessThanOrEqual(15 * 60);
    expect(rt).toMatchObject({ value: RT, httpOnly: true, sameSite: "lax", path: "/api/auth", maxAge: 30 * 24 * 60 * 60 });
    expect(res.headers.get("cache-control")).toBe("no-store");
  });

  it("Secure en producción y no en local", async () => {
    const ok = () => apiJson(200, { status: "authenticated", accessToken: AT, refreshToken: RT, expiresAt: new Date(Date.now() + 60_000).toISOString(), memberships: [] });
    fetchMock.mockImplementation(async () => ok());
    vi.stubEnv("NODE_ENV", "production");
    expect((await handleLogin(login({ email: "a@b.pe", password: "x" }))).cookies.get("wanka_at")?.secure).toBe(true);
    vi.stubEnv("NODE_ENV", "development");
    expect((await handleLogin(login({ email: "a@b.pe", password: "x" }))).cookies.get("wanka_rt")?.secure).toBe(false);
  });

  it("mfa_required: el desafío va en una cookie httpOnly de /api/auth y no hay cookies de sesión", async () => {
    fetchMock.mockResolvedValue(apiJson(200, { status: "mfa_required", challengeToken: CH, expiresAt: new Date(Date.now() + 300_000).toISOString() }));
    const res = await handleLogin(login({ email: "a@b.pe", password: "x" }));
    expect(await res.json()).toEqual({ status: "mfa_required" });
    expect(res.cookies.get("wanka_ch")).toMatchObject({ value: CH, httpOnly: true, path: "/api/auth" });
    expect(res.cookies.get("wanka_at")).toBeUndefined();
    expect(res.cookies.get("wanka_rt")).toBeUndefined();
  });

  it("mfa_enrollment_required: el token de enrolamiento va en su cookie", async () => {
    fetchMock.mockResolvedValue(apiJson(200, { status: "mfa_enrollment_required", enrollmentToken: EN, expiresAt: new Date(Date.now() + 600_000).toISOString(), emailCodeSent: true }));
    const res = await handleLogin(login({ email: "a@b.pe", password: "x" }));
    expect(await res.json()).toEqual({ status: "mfa_enrollment_required" });
    expect(res.cookies.get("wanka_en")).toMatchObject({ value: EN, httpOnly: true, path: "/api/auth" });
  });

  it("un token con forma inválida no se guarda: 502", async () => {
    fetchMock.mockResolvedValue(apiJson(200, { status: "authenticated", accessToken: "x", refreshToken: RT, expiresAt: "2030-01-01T00:00:00Z", memberships: [] }));
    const res = await handleLogin(login({ email: "a@b.pe", password: "x" }));
    expect(res.status).toBe(502);
    expect(res.cookies.get("wanka_at")).toBeUndefined();
  });

  describe("errores de la API", () => {
    const cases: [number, Record<string, unknown>, Record<string, string>, number, Record<string, unknown>][] = [
      [401, { code: "invalid_credentials", detail: "no such user ana@x.pe" }, { "WWW-Authenticate": "Bearer" }, 401, { code: "invalid_credentials" }],
      [403, { code: "tenant_access_denied" }, {}, 403, { code: "tenant_access_denied" }],
      [429, { code: "rate_limited" }, { "Retry-After": "30" }, 429, { code: "rate_limited", retryAfterSeconds: 30 }],
      [503, { code: "overloaded" }, { "Retry-After": "1" }, 503, { code: "overloaded", retryAfterSeconds: 1 }],
      [503, { code: "dependency_unavailable" }, { "Retry-After": "60" }, 503, { code: "dependency_unavailable", retryAfterSeconds: 60 }],
      [500, { code: "internal" }, {}, 502, { code: "unavailable" }],
      [422, { code: "validation_failed" }, {}, 502, { code: "unavailable" }],
    ];
    it.each(cases)("API %i %j → %i", async (status, body, headers, want, wantBody) => {
      fetchMock.mockResolvedValue(apiJson(status, body, headers));
      const res = await handleLogin(login({ email: "a@b.pe", password: "x" }));
      expect(res.status).toBe(want);
      expect(await res.json()).toEqual(wantBody);
      expect(res.cookies.getAll()).toEqual([]);
      expect(res.headers.has("www-authenticate")).toBe(false);
    });

    it("503 con Retry-After lo devuelve también como cabecera", async () => {
      fetchMock.mockResolvedValue(apiJson(503, { code: "dependency_unavailable" }, { "Retry-After": "60" }));
      expect((await handleLogin(login({ email: "a@b.pe", password: "x" }))).headers.get("retry-after")).toBe("60");
    });

    it("API caída o sin responder: 502 unavailable", async () => {
      fetchMock.mockRejectedValue(new TypeError("fetch failed"));
      const res = await handleLogin(login({ email: "a@b.pe", password: "x" }));
      expect(res.status).toBe(502);
      expect(await res.json()).toEqual({ code: "unavailable" });
    });
  });

  describe("entrada", () => {
    it.each([
      ["JSON roto", "{no"],
      ["no es objeto", "7"],
    ])("%s: 400 sin llamar a la API", async (_n, body) => {
      const res = await handleLogin(login(body));
      expect(res.status).toBe(400);
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("Content-Type ajeno: 400", async () => {
      expect((await handleLogin(login({ email: "a@b.pe", password: "x" }, { "Content-Type": "text/plain" }))).status).toBe(400);
    });

    it.each([
      ["correo sin arroba", { email: "ana", password: "x" }],
      ["contraseña vacía", { email: "a@b.pe", password: "" }],
      ["contraseña de 129", { email: "a@b.pe", password: "x".repeat(129) }],
      ["tipos erróneos", { email: 1, password: 2 }],
    ])("%s: 422 sin llamar a la API", async (_n, body) => {
      const res = await handleLogin(login(body));
      expect(res.status).toBe(422);
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("origen ajeno: 403 sin llamar a la API", async () => {
      const res = await handleLogin(login({ email: "a@b.pe", password: "x" }, { origin: "https://evil.example" }));
      expect(res.status).toBe(403);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  it("nunca registra la contraseña ni los tokens", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
    await handleLogin(login({ email: "a@b.pe", password: PASSWORD }));
    fetchMock.mockResolvedValueOnce(apiJson(200, { status: "authenticated", accessToken: AT, refreshToken: RT, expiresAt: new Date(Date.now() + 60_000).toISOString(), memberships: [] }));
    await handleLogin(login({ email: "a@b.pe", password: PASSWORD }));
    const logged = JSON.stringify([...errorLog.mock.calls, ...infoLog.mock.calls]);
    for (const secret of [PASSWORD, AT, RT]) expect(logged).not.toContain(secret);
  });
});

describe("isSameOrigin", () => {
  const req = (h: Record<string, string>) => new Request("http://localhost:3000/x", { method: "POST", headers: h });
  it("acepta el mismo host, también tras un proxy (X-Forwarded-Host)", () => {
    expect(isSameOrigin(req({ origin: "https://app.wanka.pe", host: "10.0.0.5:3000", "x-forwarded-host": "app.wanka.pe" }))).toBe(true);
  });
  it("rechaza otro host y un Origin ilegible", () => {
    expect(isSameOrigin(req({ origin: "https://evil.example", host: "app.wanka.pe" }))).toBe(false);
    expect(isSameOrigin(req({ origin: "null", host: "app.wanka.pe" }))).toBe(false);
  });
  it("sin Origin, rechaza Sec-Fetch-Site cross-site", () => {
    expect(isSameOrigin(req({ "sec-fetch-site": "cross-site" }))).toBe(false);
    expect(isSameOrigin(req({ "sec-fetch-site": "same-origin" }))).toBe(true);
  });
});

describe("parseRetryAfter", () => {
  it("segundos, fecha HTTP, tope y valores ilegibles", () => {
    const now = Date.parse("2026-11-20T10:00:00Z");
    expect(parseRetryAfter("60")).toBe(60);
    expect(parseRetryAfter("0")).toBe(1);
    expect(parseRetryAfter("999999")).toBe(3600);
    expect(parseRetryAfter("Fri, 20 Nov 2026 10:00:30 GMT", now)).toBe(30);
    expect(parseRetryAfter("pronto")).toBeUndefined();
    expect(parseRetryAfter(null)).toBeUndefined();
  });
});
