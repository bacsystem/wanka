// @vitest-environment node
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { EMAIL_MAX, EMAIL_MIN, PASSWORD_MAX, runeLength, validateLogin } from "./login-contract";

describe("límites de LoginRequest atados al contrato", () => {
  const yaml = readFileSync(path.resolve(__dirname, "../../../../../api/openapi/paths/identidad.yaml"), "utf8");
  const block = yaml.slice(yaml.indexOf("    LoginRequest:"), yaml.indexOf("    LoginResponse:"));
  const limit = (field: "email" | "password", key: "minLength" | "maxLength") => {
    const part = block.slice(block.indexOf(`        ${field}:`));
    return Number(new RegExp(`${key}: (\\d+)`).exec(part)?.[1]);
  };

  it("las constantes coinciden con minLength y maxLength del contrato", () => {
    expect(EMAIL_MIN).toBe(limit("email", "minLength"));
    expect(EMAIL_MAX).toBe(limit("email", "maxLength"));
    expect(1).toBe(limit("password", "minLength"));
    expect(PASSWORD_MAX).toBe(limit("password", "maxLength"));
  });
});

describe("validateLogin (largos en puntos de código, como la API)", () => {
  const email = (n: number) => `${"a".repeat(n - 5)}@b.pe`;
  it("contraseña: 1 y 128 pasan; 0 y 129 no", () => {
    expect(validateLogin("a@b.pe", "x")).toBeNull();
    expect(validateLogin("a@b.pe", "x".repeat(128))).toBeNull();
    expect(validateLogin("a@b.pe", "")).toBe("password");
    expect(validateLogin("a@b.pe", "x".repeat(129))).toBe("passwordTooLong");
  });
  it("70 emojis son 70 caracteres (140 unidades UTF-16): válido", () => {
    const pass = "😀".repeat(70);
    expect(pass.length).toBe(140);
    expect(runeLength(pass)).toBe(70);
    expect(validateLogin("a@b.pe", pass)).toBeNull();
    expect(validateLogin("a@b.pe", "😀".repeat(129))).toBe("passwordTooLong");
  });
  it("correo: 254 pasa y 255 no", () => {
    expect(validateLogin(email(254), "x")).toBeNull();
    expect(validateLogin(email(255), "x")).toBe("email");
  });
});
