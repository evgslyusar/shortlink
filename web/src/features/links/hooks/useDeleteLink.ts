import { useMutation, useQueryClient } from "@tanstack/react-query";

import { deleteLink, linkKeys } from "../api";

export function useDeleteLink() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slug: string) => deleteLink(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: linkKeys.all });
    },
  });
}
