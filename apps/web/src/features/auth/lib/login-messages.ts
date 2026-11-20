import type { LoginErrorCode } from "./login-contract";

/** Textos del login (español de Perú). Un solo mensaje para las credenciales malas: no dice cuál falló ni si el correo existe. */
export const LOGIN_TEXT = {
  title: "Iniciar sesión",
  description: "Ingresa con tu correo y tu contraseña.",
  email: "Correo electrónico",
  password: "Contraseña",
  submit: "Ingresar",
  submitting: "Ingresando…",
  showPassword: "Mostrar contraseña",
  hidePassword: "Ocultar contraseña",
  emailInvalid: "Ingresa un correo válido, por ejemplo nombre@empresa.pe.",
  passwordRequired: "Ingresa tu contraseña.",
  mfaTitle: "Verificación en dos pasos",
  mfaBody: "Tu contraseña es correcta, pero tu cuenta pide verificación en dos pasos y esa función estará disponible próximamente. Por ahora no podemos completar tu ingreso.",
  mfaBack: "Volver al inicio de sesión",
} as const;

const ERRORS: Record<Exclude<LoginErrorCode, "rate_limited" | "overloaded" | "dependency_unavailable">, string> = {
  invalid_credentials: "Correo o contraseña incorrectos.",
  tenant_access_denied: "Tu cuenta no tiene acceso a ninguna empresa. Pide a un administrador de tu empresa que te dé acceso.",
  validation_failed: "Revisa el correo y la contraseña e inténtalo de nuevo.",
  malformed_request: "No pudimos procesar el formulario. Recarga la página e inténtalo de nuevo.",
  forbidden_origin: "No pudimos verificar el origen de la solicitud. Recarga la página e inténtalo de nuevo.",
  unavailable: "No pudimos conectar con el servicio. Inténtalo de nuevo en unos segundos.",
};

/** Mensaje de un error de login. Con `retryAfterSeconds` (429 y 503) pide esperar ese tiempo; sin él, «unos segundos». */
export function loginErrorMessage(code: LoginErrorCode, retryAfterSeconds?: number): string {
  if (code === "rate_limited" || code === "overloaded" || code === "dependency_unavailable") {
    const wait = retryAfterSeconds === undefined ? "unos segundos" : retryAfterSeconds === 1 ? "1 segundo" : `${retryAfterSeconds} segundos`;
    if (code === "rate_limited") return `Hiciste demasiados intentos. Inténtalo de nuevo en ${wait}.`;
    return `El servicio está ocupado. Inténtalo de nuevo en ${wait}.`;
  }
  return ERRORS[code];
}
