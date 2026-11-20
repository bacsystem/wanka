"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Alerta } from "khipu-design-system/feedback/alerta";
import { Campo, Entrada } from "khipu-design-system/formularios/campo";
import { Contrasena } from "khipu-design-system/formularios/contrasena";
import { BotonAsync } from "khipu-design-system/patrones/boton-async";
import { Button } from "khipu-design-system/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "khipu-design-system/ui/card";
import { BOTON_PRIMARIO } from "@/lib/estilos";
import { validateLogin, type LoginError, type LoginOutcome } from "../lib/login-contract";
import { LOGIN_TEXT, loginErrorMessage } from "../lib/login-messages";

type Step = "credentials" | "mfa";
type Failure = { code: LoginError["code"]; retryAfterSeconds?: number };

/** Una respuesta del BFF que no es la esperada (cuerpo vacío, HTML de un proxy…) se trata como servicio no disponible. */
function isLoginError(body: unknown): body is LoginError {
  return typeof body === "object" && body !== null && typeof (body as { code?: unknown }).code === "string";
}

/** `/login`: correo y contraseña contra `POST /api/auth/login` (BFF). Los tokens nunca llegan a esta pantalla. */
export function LoginScreen() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("credentials");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [touched, setTouched] = useState({ email: false, password: false });
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<Failure | null>(null);
  // Espera pedida por el servidor (429 y 503): el botón sigue deshabilitado hasta que pasa.
  const [waitUntil, setWaitUntil] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const inFlight = useRef<AbortController | null>(null);

  useEffect(() => () => inFlight.current?.abort(), []);

  useEffect(() => {
    if (waitUntil === null) return;
    const id = window.setInterval(() => {
      const t = Date.now();
      setNow(t);
      if (t >= waitUntil) {
        setWaitUntil(null);
        setFailure(null);
      }
    }, 500);
    return () => window.clearInterval(id);
  }, [waitUntil]);

  const invalid = validateLogin(email, password);
  const emailError = touched.email && invalid === "email" ? LOGIN_TEXT.emailInvalid : undefined;
  const passwordError =
    touched.password && invalid === "password" ? LOGIN_TEXT.passwordRequired : touched.password && invalid === "passwordTooLong" ? LOGIN_TEXT.passwordTooLong : undefined;
  const remaining = waitUntil === null ? 0 : Math.max(0, Math.ceil((waitUntil - now) / 1000));
  const waiting = remaining > 0;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || waiting) return;
    setTouched({ email: true, password: true });
    if (invalid !== null) return;

    inFlight.current?.abort();
    const controller = new AbortController();
    inFlight.current = controller;
    setPending(true);
    setFailure(null);
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: email.trim(), password }),
        signal: controller.signal,
      });
      const body: unknown = await res.json().catch(() => null);
      if (res.ok && typeof body === "object" && body !== null) {
        const { status } = body as Partial<LoginOutcome>;
        if (status === "authenticated") {
          // El shell lee las cookies en el servidor: se navega y se refresca. `pending` queda activo hasta salir de la página.
          router.replace("/");
          router.refresh();
          return;
        }
        if (status === "mfa_required" || status === "mfa_enrollment_required") {
          setPassword("");
          setStep("mfa");
          setPending(false);
          return;
        }
      }
      const error: Failure = isLoginError(body) ? body : { code: "unavailable" };
      setFailure(error);
      if (error.retryAfterSeconds !== undefined) {
        const t = Date.now();
        setNow(t);
        setWaitUntil(t + error.retryAfterSeconds * 1000);
      }
      setPending(false);
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setFailure({ code: "unavailable" });
      setPending(false);
    }
  }

  function backToCredentials() {
    setStep("credentials");
    setFailure(null);
  }

  return (
    <main className="flex flex-1 items-center justify-center px-4 py-10">
      <Card className="w-full max-w-sm">
        {step === "mfa" ? (
          <>
            <CardHeader>
              <CardTitle>
                <h1>{LOGIN_TEXT.mfaTitle}</h1>
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <Alerta tono="info">{LOGIN_TEXT.mfaBody}</Alerta>
              <Button variant="outline" onClick={backToCredentials}>
                {LOGIN_TEXT.mfaBack}
              </Button>
            </CardContent>
          </>
        ) : (
          <>
            <CardHeader>
              <CardTitle>
                <h1>{LOGIN_TEXT.title}</h1>
              </CardTitle>
              <CardDescription>{LOGIN_TEXT.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={submit} noValidate className="flex flex-col gap-4">
                {failure ? <Alerta tono="error">{loginErrorMessage(failure.code, waiting ? remaining : failure.retryAfterSeconds)}</Alerta> : null}
                <Campo id="email" etiqueta={LOGIN_TEXT.email} error={emailError}>
                  <Entrada
                    id="email"
                    name="email"
                    type="email"
                    inputMode="email"
                    autoComplete="username"
                    autoFocus
                    value={email}
                    invalido={emailError !== undefined}
                    aria-describedby={emailError ? "email-error" : undefined}
                    onChange={(e) => setEmail(e.target.value)}
                    onBlur={() => setTouched((t) => ({ ...t, email: true }))}
                  />
                </Campo>
                <Campo id="password" etiqueta={LOGIN_TEXT.password} error={passwordError}>
                  <Contrasena
                    id="password"
                    valor={password}
                    onCambio={setPassword}
                    onBlur={() => setTouched((t) => ({ ...t, password: true }))}
                    autoComplete="current-password"
                    invalido={passwordError !== undefined}
                    describidoPor={passwordError ? "password-error" : undefined}
                    etiquetaMostrar={LOGIN_TEXT.showPassword}
                    etiquetaOcultar={LOGIN_TEXT.hidePassword}
                  />
                </Campo>
                <BotonAsync type="submit" pendiente={pending} textoPendiente={LOGIN_TEXT.submitting} disabled={waiting} className={BOTON_PRIMARIO}>
                  {LOGIN_TEXT.submit}
                </BotonAsync>
              </form>
            </CardContent>
          </>
        )}
      </Card>
    </main>
  );
}
