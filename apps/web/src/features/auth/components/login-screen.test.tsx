import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoginScreen } from "./login-screen";

const replace = vi.fn();
const refresh = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, refresh }) }));

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

function fill(email: string, password: string) {
  fireEvent.change(screen.getByLabelText("Correo electrónico"), { target: { value: email } });
  fireEvent.change(screen.getByLabelText("Contraseña"), { target: { value: password } });
}
const submit = () => fireEvent.click(screen.getByRole("button", { name: "Ingresar" }));

describe("LoginScreen", () => {
  const fetchMock = vi.fn<typeof fetch>();
  beforeEach(() => vi.stubGlobal("fetch", fetchMock));
  afterEach(() => {
    fetchMock.mockReset();
    replace.mockReset();
    refresh.mockReset();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("valida el correo antes de enviar", async () => {
    render(<LoginScreen />);
    fill("ana", "secreta");
    submit();
    expect(await screen.findByText(/Ingresa un correo válido/)).toBeInTheDocument();
    expect(screen.getByLabelText("Correo electrónico")).toHaveAttribute("aria-invalid", "true");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("login correcto: envía al BFF y entra al shell", async () => {
    fetchMock.mockResolvedValue(json(200, { status: "authenticated" }));
    render(<LoginScreen />);
    fill(" ana@empresa.pe ", "secreta");
    submit();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/auth/login");
    expect(JSON.parse(String(init?.body))).toEqual({ email: "ana@empresa.pe", password: "secreta" });
    expect(refresh).toHaveBeenCalled();
  });

  it("un doble clic envía una sola petición", async () => {
    fetchMock.mockImplementation(() => new Promise(() => undefined));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "secreta");
    submit();
    fireEvent.submit(screen.getByRole("button", { name: /Ingresando/ }).closest("form") as HTMLFormElement);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: /Ingresando/ })).toBeDisabled();
  });

  it("401: un único mensaje genérico", async () => {
    fetchMock.mockResolvedValue(json(401, { code: "invalid_credentials" }));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "mala");
    submit();
    expect(await screen.findByRole("alert")).toHaveTextContent("Correo o contraseña incorrectos.");
    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Ingresar" })).toBeEnabled();
  });

  it("403: tu cuenta no tiene acceso a ninguna empresa", async () => {
    fetchMock.mockResolvedValue(json(403, { code: "tenant_access_denied" }));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "secreta");
    submit();
    expect(await screen.findByRole("alert")).toHaveTextContent("Tu cuenta no tiene acceso a ninguna empresa");
  });

  it("503 con Retry-After: pide esperar y mantiene el botón deshabilitado hasta que pasa", async () => {
    // Solo se falsean el reloj y los intervalos: el paso del tiempo es el que el test avanza, y `waitFor` sigue con timers reales.
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] });
    vi.setSystemTime(new Date("2026-11-20T10:00:00Z"));
    fetchMock.mockResolvedValue(json(503, { code: "overloaded", retryAfterSeconds: 3 }));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "secreta");
    submit();
    // Deja correr la promesa del fetch y los efectos de React sin mover el reloj.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Inténtalo de nuevo en 3 segundos"));
    expect(screen.getByRole("button", { name: "Ingresar" })).toBeDisabled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(screen.getByRole("button", { name: "Ingresar" })).toBeDisabled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    await waitFor(() => expect(screen.getByRole("button", { name: "Ingresar" })).toBeEnabled());
  });

  it("respuesta ilegible o red caída: servicio no disponible", async () => {
    fetchMock.mockRejectedValue(new TypeError("fetch failed"));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "secreta");
    submit();
    expect(await screen.findByRole("alert")).toHaveTextContent("Inténtalo de nuevo en unos segundos");
  });

  it("mfa_required: pantalla de verificación en dos pasos y limpia la contraseña", async () => {
    fetchMock.mockResolvedValue(json(200, { status: "mfa_required" }));
    render(<LoginScreen />);
    fill("ana@empresa.pe", "secreta");
    submit();
    expect(await screen.findByRole("heading", { name: "Verificación en dos pasos" })).toBeInTheDocument();
    expect(screen.getByText(/disponible próximamente/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Volver al inicio de sesión" }));
    expect((screen.getByLabelText("Contraseña") as HTMLInputElement).value).toBe("");
  });
});
