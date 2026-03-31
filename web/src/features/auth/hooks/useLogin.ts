import { useMutation } from "@tanstack/react-query";

import { login } from "../api";
import type { LoginRequest } from "../types";

export function useLogin() {
  return useMutation({
    mutationFn: (data: LoginRequest) => login(data),
  });
}
