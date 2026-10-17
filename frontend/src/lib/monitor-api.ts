import { ApiError, getApiUrl } from "./auth-api";

export type Monitor = {
  id: string;
  url: string;
  intervalSeconds: number;
  createdAt: string;
};

type AuthorizedFetch = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

async function request<T>(authorizedFetch: AuthorizedFetch, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await authorizedFetch(`${getApiUrl()}/api/v1/monitors`, init);
  } catch {
    throw new Error("Не удалось подключиться к серверу. Повторите попытку.");
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new ApiError(response.status, payload.error ?? "Не удалось выполнить запрос.");
  }
  return response.json() as Promise<T>;
}

export function listMonitors(authorizedFetch: AuthorizedFetch) {
  return request<Monitor[]>(authorizedFetch);
}

export function createMonitor(authorizedFetch: AuthorizedFetch, url: string, intervalSeconds: number) {
  return request<Monitor>(authorizedFetch, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, intervalSeconds }),
  });
}
