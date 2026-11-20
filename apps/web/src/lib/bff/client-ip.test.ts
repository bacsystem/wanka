import { describe, expect, it } from "vitest";
import { resolveClientIp } from "./client-ip";

const withXff = (value: string) => new Headers({ "x-forwarded-for": value });

describe("resolveClientIp", () => {
  it("toma la entrada más a la derecha de X-Forwarded-For", () => {
    expect(resolveClientIp(withXff("1.1.1.1, 2.2.2.2, 3.3.3.3"))).toBe("3.3.3.3");
  });

  it("ignora un X-Client-Ip que manda el navegador", () => {
    const headers = new Headers({ "x-forwarded-for": "a.b.c.d, 3.3.3.3", "x-client-ip": "9.9.9.9" });
    expect(resolveClientIp(headers)).toBe("3.3.3.3");
    expect(resolveClientIp(new Headers({ "x-client-ip": "9.9.9.9" }))).toBeNull();
  });

  it("acepta una sola dirección (local: la IP del socket que pone Next)", () => {
    expect(resolveClientIp(withXff("::1"))).toBe("::1");
    expect(resolveClientIp(withXff("203.0.113.7"))).toBe("203.0.113.7");
    expect(resolveClientIp(withXff("2001:db8::1"))).toBe("2001:db8::1");
  });

  it.each([
    ["sin cabecera", null],
    ["vacía", ""],
    ["no es una IP", "unknown"],
    ["con puerto", "1.2.3.4:5678"],
    ["IPv6 entre corchetes", "[::1]"],
    ["IPv6 con zona", "fe80::1%eth0"],
    ["termina en coma", "1.1.1.1,"],
    ["texto inyectado", "1.1.1.1\r\nX-Evil: 1"],
  ])("no devuelve nada si %s", (_name, value) => {
    const headers = value === null ? new Headers() : new Headers({ "x-forwarded-for": value.replace(/[\r\n]/g, " ") });
    expect(resolveClientIp(headers)).toBeNull();
  });
});
