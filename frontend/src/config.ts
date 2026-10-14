// Runtime configuration from Vite env vars (inlined at build time).

const DEFAULT_API_URL = "http://localhost:8080";
const WS_PATH = "/ws/alerts";

function resolveApiUrl(raw: string | undefined): string {
  const value = raw?.trim() || DEFAULT_API_URL;
  // Relative values (e.g. "/api" behind a reverse proxy) resolve against the page.
  return new URL(value, window.location.origin).toString().replace(/\/+$/, "");
}

/** Derives the WebSocket URL from the API URL: http → ws, https → wss. */
export function deriveWsUrl(apiUrl: string): string {
  const url = new URL(apiUrl);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = url.pathname.replace(/\/+$/, "") + WS_PATH;
  url.search = "";
  url.hash = "";
  return url.toString();
}

const apiUrl = resolveApiUrl(import.meta.env.VITE_API_URL);

export const config = {
  apiUrl,
  wsUrl: import.meta.env.VITE_WS_URL?.trim() || deriveWsUrl(apiUrl),
  apiKey: import.meta.env.VITE_API_KEY?.trim() || undefined,
} as const;
