import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";
import { config, proxy } from "./proxy";

function call(path: string) {
  return proxy(new NextRequest(`http://localhost:3000${path}`));
}

describe("proxy", () => {
  it.each(["/ventas", "/clientes", "/configuracion/usuarios", "/inventario/stock", "/ventas/comprobantes/F001-00000001", "/inventario/recepcion/g.1", "/catalogo/variantes/s.k", "/clientes/abc.def", "/clientes/a.b/historia", "/ventas.txt", "/api/x", "/api/v1/ventas"])(
    "%s se reescribe al 404",
    (path) => {
      const res = call(path);
      expect(res.status).toBe(404);
      expect(res.headers.get("x-middleware-rewrite")).toBe("http://localhost:3000/no-habilitada");
    },
  );

  it.each(["/", "/login", "/seleccionar-empresa", "/admin/empresas"])("%s sigue su camino", (path) => {
    const res = call(path);
    expect(res.status).toBe(200);
    expect(res.headers.get("x-middleware-rewrite")).toBeNull();
  });
});

describe("matcher", () => {
  const matches = (path: string) => new RegExp(`^${config.matcher[0]}$`).test(path);

  it.each(["/inventario/recepcion/g.1", "/catalogo/variantes/s.k", "/clientes/abc.def", "/ventas.txt", "/api/x", "/ventas", "/"])(
    "%s pasa por el proxy",
    (path) => expect(matches(path)).toBe(true),
  );

  it.each(["/_next/static/chunks/a.js", "/_next/image", "/favicon.ico"])("%s no pasa por el proxy (recurso de Next)", (path) =>
    expect(matches(path)).toBe(false),
  );
});
