import { e2eEmail } from "./login-users";
import { expect, test } from "./fixtures";
import type { Page } from "./fixtures";

// Contra la API real (make up). Sin ella, el globalSetup no siembra y todo este archivo se salta.
test.skip(!process.env.E2E_LOGIN_PASSWORD, "needs the local API (make up): see docs/local-environment.md");

const password = () => process.env.E2E_LOGIN_PASSWORD ?? "";

async function fillAndSubmit(page: Page, email: string, pass: string) {
  await page.getByLabel("Correo electrónico").fill(email);
  await page.getByLabel("Contraseña", { exact: true }).fill(pass);
  await page.getByRole("button", { name: "Ingresar" }).click();
}

test.beforeEach(async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Iniciar sesión" })).toBeVisible();
});

test("valid credentials land on the empty shell, with the tokens only in httpOnly cookies", async ({ page, context }, testInfo) => {
  await fillAndSubmit(page, e2eEmail("ok", testInfo.project.name), password());
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("navigation", { name: "Menú principal" }).or(page.getByRole("button", { name: "Más" })).first()).toBeVisible();

  const cookies = await context.cookies();
  const access = cookies.find((c) => c.name === "wanka_at");
  const refresh = cookies.find((c) => c.name === "wanka_rt");
  expect(access).toMatchObject({ httpOnly: true, sameSite: "Lax", path: "/" });
  expect(refresh).toMatchObject({ httpOnly: true, sameSite: "Lax", path: "/api/auth" });
  expect(access?.value).toMatch(/^wk_at_[A-Za-z0-9_-]{43}$/);
  // El JavaScript de la página no ve los tokens: ni en document.cookie ni en el almacenamiento.
  const visible = await page.evaluate(() => JSON.stringify([document.cookie, { ...localStorage }, { ...sessionStorage }]));
  expect(visible).not.toContain("wk_");
});

test("wrong password: one generic message, no redirect, no cookies", async ({ page, context }, testInfo) => {
  await fillAndSubmit(page, e2eEmail("bad", testInfo.project.name), "no-es-la-clave");
  await expect(page.getByRole("alert").filter({ hasText: "Correo o contraseña" })).toHaveText("Correo o contraseña incorrectos.");
  await expect(page).toHaveURL(/\/login$/);
  expect((await context.cookies()).filter((c) => c.name.startsWith("wanka_"))).toEqual([]);
});

test("an unknown email gets the same message as a wrong password", async ({ page }, testInfo) => {
  await fillAndSubmit(page, `nadie-${testInfo.project.name}@e2e.wanka.test`, "no-es-la-clave");
  await expect(page.getByRole("alert").filter({ hasText: "Correo o contraseña" })).toHaveText("Correo o contraseña incorrectos.");
});

test("correct password but no active company: access denied message, no session", async ({ page, context }, testInfo) => {
  await fillAndSubmit(page, e2eEmail("nomembership", testInfo.project.name), password());
  await expect(page.getByRole("alert").filter({ hasText: "Tu cuenta no tiene acceso" })).toContainText("a ninguna empresa");
  await expect(page).toHaveURL(/\/login$/);
  expect((await context.cookies()).filter((c) => c.name.startsWith("wanka_"))).toEqual([]);
});

test("a user with two-step verification sees the coming-soon screen, without session cookies", async ({ page, context }, testInfo) => {
  await fillAndSubmit(page, e2eEmail("mfa", testInfo.project.name), password());
  await expect(page.getByRole("heading", { name: "Verificación en dos pasos" })).toBeVisible();
  const names = (await context.cookies()).map((c) => c.name);
  expect(names).toContain("wanka_ch");
  expect(names).not.toContain("wanka_at");
});

test("an invalid email is caught in the form, before calling the API", async ({ page }) => {
  let calls = 0;
  await page.route("**/api/auth/login", (route) => {
    calls += 1;
    return route.continue();
  });
  await fillAndSubmit(page, "sin-arroba", "x");
  await expect(page.getByText(/Ingresa un correo válido/)).toBeVisible();
  expect(calls).toBe(0);
});

test("the form has no horizontal overflow", async ({ page }) => {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(overflow).toBe(false);
});
