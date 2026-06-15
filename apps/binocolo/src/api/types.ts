export interface OpenAPIITEnvelope<T> {
  data: T;
  success: boolean;
  message: string;
  error: number | null;
}

export interface Province {
  sigla: string;
  provincia: string;
  superficie: number;
  residenti: number;
  num_comuni: number;
  istat: string;
  regione: string;
}

export interface CompanySearchRow {
  id: string;
  taxCode?: string | null;
  companyName?: string | null;
  vatCode?: string | null;
  activityStatus?: string | null;
  address?: {
    registeredOffice?: {
      town?: string | null;
      province?: string | null;
      zipCode?: string | null;
    };
  };
}
