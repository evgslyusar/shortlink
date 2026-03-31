export const API_BASE_URL = "/v1";

export const queryKeys = {
  links: {
    all: ["links"] as const,
    list: (page: number) => [...queryKeys.links.all, "list", page] as const,
    stats: (slug: string) => [...queryKeys.links.all, "stats", slug] as const,
  },
  auth: {
    me: ["auth", "me"] as const,
  },
} as const;
