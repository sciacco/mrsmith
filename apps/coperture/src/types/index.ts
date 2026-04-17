export interface LocationOption {
  id: number;
  name: string;
}

export interface CoverageProfile {
  name: string;
}

export interface CoverageDetail {
  type_id: number;
  type_name: string;
  value: string;
}

export interface CoverageResult {
  coverage_id: string;
  operator_id: number;
  operator_name: string;
  logo_url: string;
  tech: string;
  profiles: CoverageProfile[];
  details: CoverageDetail[];
}
