export const TENANT_COOKIE = "tenant_id";
export const SITE_COOKIE = "site_id";
export const COOKIE_MAX_AGE = 60 * 60 * 24 * 30;

export function setClientCookie(name: string, value: string) {
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${COOKIE_MAX_AGE}; samesite=lax`;
}
