import { apiClient } from "@/api/client";

import type {
  CreateLinkRequest,
  CreateLinkResponse,
  LinkStatsResponse,
  PaginatedLinks,
} from "./types";

export const linkKeys = {
  all: ["links"] as const,
  list: (page: number) => [...linkKeys.all, "list", page] as const,
  stats: (slug: string) => [...linkKeys.all, "stats", slug] as const,
};

export function createLink(data: CreateLinkRequest) {
  return apiClient.post<CreateLinkResponse>("/links", data);
}

export function listLinks(page: number, perPage: number) {
  return apiClient.get<PaginatedLinks>(`/links?page=${page}&per_page=${perPage}`);
}

export function deleteLink(slug: string) {
  return apiClient.delete<null>(`/links/${slug}`);
}

export function getLinkStats(slug: string) {
  return apiClient.get<LinkStatsResponse>(`/links/${slug}/stats`);
}
