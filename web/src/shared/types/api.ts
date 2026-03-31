export interface ApiResponse<T> {
  data: T;
  meta: Meta;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    details?: unknown[];
  };
  meta: Meta;
}

export interface Meta {
  request_id: string;
  page?: number;
  per_page?: number;
  total?: number;
}
