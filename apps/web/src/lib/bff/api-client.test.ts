import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildApiHeaders, callApi } from "./api-client";

function browserRequest(headers: Record<string, string>) {
  return new Request("http://localhost:3000/api/x", { headers });
}

describe("buildApiHeaders", () => {
  it("con X-Forwarded-For a, b, c y X-Client-Ip 9.9.9.9 del navegador, la API recibe X-Client-Ip c", () => {
    const incoming = new Headers({ "x-forwarded-for": "1.1.1.1, 2.2.2.2, 3.3.3.3", "x-client-ip": "9.9.9.9" });
    expect(buildApiHeaders(incoming).get("x-client-ip")).toBe("3.3.3.3");
  });

  it("descarta el X-Client-Ip del navegador aunque no haya X-Forwarded-For", () => {
    const headers = buildApiHeaders(new Headers({ "x-client-ip": "9.9.9.9" }));
    expect(headers.has("x-client-ip")).toBe(false);
  });

  it("una cabecera X-Client-Ip pasada por el llamador tampoco sobrevive", () => {
    const headers = buildApiHeaders(new Headers({ "x-forwarded-for": "3.3.3.3" }), { "X-Client-Ip": "9.9.9.9" });
    expect(headers.get("x-client-ip")).toBe("3.3.3.3");
  });

  it("no copia las cabeceras del navegador y genera su propio X-Request-Id", () => {
    const incoming = new Headers({ cookie: "a=b", authorization: "Bearer x", "x-request-id": "browser-id" });
    const headers = buildApiHeaders(incoming);
    expect(headers.has("cookie")).toBe(false);
    expect(headers.has("authorization")).toBe(false);
    expect(headers.get("x-request-id")).not.toBe("browser-id");
    expect(headers.get("x-request-id")).toMatch(/^[0-9a-f-]{36}$/);
  });
});

describe("callApi", () => {
  const fetchMock = vi.fn(async () => new Response("{}", { status: 200 }));

  beforeEach(() => {
    vi.stubEnv("WANKA_API_URL", "http://api.internal:8080/");
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    fetchMock.mockClear();
  });

  it("envía X-Client-Ip con la entrada más a la derecha de X-Forwarded-For", async () => {
    const incoming = browserRequest({ "x-forwarded-for": "a, b, 3.3.3.3", "x-client-ip": "9.9.9.9" });
    await callApi(incoming, "/v1/me/entitlements");
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://api.internal:8080/v1/me/entitlements");
    expect(new Headers(init.headers).get("x-client-ip")).toBe("3.3.3.3");
  });

  it("serializa json y pone el Content-Type", async () => {
    await callApi(browserRequest({}), "/v1/auth/login", { method: "POST", json: { email: "a@b.pe" } });
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"email":"a@b.pe"}');
    expect(new Headers(init.headers).get("content-type")).toBe("application/json");
  });

  it.each(["/healthz", "//evil.test/v1/x", "/v1/../admin", "http://evil.test/v1/x", "/v1/a b", "v1/x"])(
    "rechaza la ruta %s",
    async (path) => {
      await expect(callApi(browserRequest({}), path)).rejects.toThrow("Invalid API path");
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it("falla claro si no hay WANKA_API_URL", async () => {
    vi.stubEnv("WANKA_API_URL", "");
    await expect(callApi(browserRequest({}), "/v1/me/entitlements")).rejects.toThrow("WANKA_API_URL");
  });
});
