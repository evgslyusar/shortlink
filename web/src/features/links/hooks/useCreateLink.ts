import { useMutation, useQueryClient } from "@tanstack/react-query";

import { createLink, linkKeys } from "../api";
import type { CreateLinkRequest } from "../types";

export function useCreateLink() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateLinkRequest) => createLink(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: linkKeys.all });
    },
  });
}
