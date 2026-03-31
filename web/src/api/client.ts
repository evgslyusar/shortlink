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
    return { data: null as T, meta: { request_id: "" } };
  }

  const body: unknown = await res.json();

  if (!res.ok) {
    const err = body as ApiError;

    if (res.status === 401 && window.location.pathname !== "/login") {
      window.location.href = "/login";
    }

    throw new ApiClientError(res.status, err.error);
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
