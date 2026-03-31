import { useMutation } from "@tanstack/react-query";

import { register } from "../api";
import type { RegisterRequest } from "../types";

export function useRegister() {
  return useMutation({
    mutationFn: (data: RegisterRequest) => register(data),
  });
}
