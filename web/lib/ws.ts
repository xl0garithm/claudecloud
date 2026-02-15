/**
 * WebSocket URL builder with auth token from session cookie.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/**
 * Builds a WebSocket URL with ?token= from the session cookie.
 * Converts http:// to ws:// and https:// to wss://.
 */
export function createAuthWsUrl(path: string): string {
  const wsBase = API_BASE.replace(/^http/, "ws");
  const token = getSessionToken();
  const separator = path.includes("?") ? "&" : "?";
  return `${wsBase}${path}${separator}token=${token}`;
}

function getSessionToken(): string {
  if (typeof document === "undefined") return "";
  const match = document.cookie.match(/(?:^|;\s*)session=([^;]*)/);
  return match ? match[1] : "";
}
