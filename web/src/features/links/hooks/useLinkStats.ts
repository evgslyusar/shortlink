import { useQuery } from "@tanstack/react-query";

import { getLinkStats, linkKeys } from "../api";

export function useLinkStats(slug: string | null) {
  return useQuery({
    queryKey: linkKeys.stats(slug ?? ""),
    queryFn: () => getLinkStats(slug!),
    enabled: slug !== null,
    staleTime: 30_000,
  });
}
