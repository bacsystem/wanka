import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";
import { proxy } from "./proxy";

function call(path: string) {
  return proxy(new NextRequest(`http://localhost:3000${path}`));
}

describe("proxy", () => {
  it.each(["/ventas", "/clientes", "/configuracion/usuarios", "/inventario/stock", "/ventas/comprobantes/F001-00000001"])(
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
