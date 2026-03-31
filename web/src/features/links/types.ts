export interface Link {
  slug: string;
  short_url: string;
  original_url: string;
  expires_at: string | null;
  created_at: string;
}

export interface CreateLinkRequest {
  url: string;
  slug?: string;
  expires_at?: string;
}

export interface CreateLinkResponse {
  slug: string;
  short_url: string;
  original_url: string;
  expires_at: string | null;
}

export interface PaginatedLinks {
  items: Link[];
}

export interface DayStat {
  date: string;
  count: number;
}

export interface CountryStat {
  country: string;
  count: number;
}

export interface LinkStatsResponse {
  total_clicks: number;
  by_day: DayStat[];
  by_country: CountryStat[];
}
