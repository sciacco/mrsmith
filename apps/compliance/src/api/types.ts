export interface BlockRequest {
  id: number;
  request_date: string;
  reference: string;
  method_id: string;
  method_description: string;
}

export interface BlockDomain {
  id: number;
  domain: string;
}

export interface ReleaseRequest {
  id: number;
  request_date: string;
  reference: string;
}

export interface ReleaseDomain {
  id: number;
  domain: string;
}

export interface Origin {
  method_id: string;
  description: string;
  is_active: boolean;
}

export interface DomainStatus {
  domain: string;
  block_count: number;
  release_count: number;
}

export interface HistoryEntry {
  domain: string;
  request_date: string;
  reference: string;
  request_type: 'block' | 'release';
}

export interface ValidationErrorResponse {
  error: 'invalid_domains';
  message: string;
  invalid: string[];
}
