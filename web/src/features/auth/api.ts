import { apiClient } from "@/api/client";
import type { AuthResponse, LoginRequest, RegisterRequest, UserResponse } from "./types";

export function login(data: LoginRequest) {
  return apiClient.post<AuthResponse>("/auth/login", data);
}

export function register(data: RegisterRequest) {
  return apiClient.post<UserResponse>("/auth/register", data);
}

export function refresh() {
  return apiClient.post<AuthResponse>("/auth/refresh", {});
}

export function logout() {
  return apiClient.post<null>("/auth/logout", {});
}
