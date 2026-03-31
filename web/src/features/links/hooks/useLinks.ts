import { useQuery } from "@tanstack/react-query";

import { linkKeys, listLinks } from "../api";

export function useLinks(page: number, perPage = 20) {
  return useQuery({
    queryKey: linkKeys.list(page),
    queryFn: () => listLinks(page, perPage),
    staleTime: 30_000,
  });
}
