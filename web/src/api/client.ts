import { API_BASE_URL } from "@/shared/lib/constants";
import type { ApiError, ApiResponse } from "@/shared/types/api";

class ApiClientError extends Error {
  constructor(
    public status: number,
    public apiError: ApiError["error"],
  ) {
    super(apiError.message);
    this.name = "ApiClientError";
  }
}

// onUnauthorized is called when the API returns 401. Set by the app shell
// to trigger a React Router navigation instead of a hard window.location reload.
let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(fn: () => void) {
  onUnauthorized = fn;
}

async function request<T>(path: string, init?: RequestInit): Promise<ApiResponse<T>> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (res.status === 204) {
    // 204 No Content — return a sentinel value. Callers using delete/logout
    // expect ApiResponse<null> so this is safe for those paths.
    return { data: null as unknown as T, meta: { request_id: "" } };
  }

  const body: unknown = await res.json();

  if (!res.ok) {
    // Safely extract error — guard against unexpected response shapes.
    const apiErr = body as ApiError;
    const errorObj = apiErr?.error ?? { code: "UNKNOWN", message: "unexpected error" };

    if (res.status === 401 && onUnauthorized) {
      onUnauthorized();
    }

    throw new ApiClientError(res.status, errorObj);
  }

  return body as ApiResponse<T>;
}

export const apiClient = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, { method: "POST", body: data ? JSON.stringify(data) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

export { ApiClientError };
