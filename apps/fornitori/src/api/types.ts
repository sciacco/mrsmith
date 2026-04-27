export interface PaginatedEnvelope<T> {
  items?: T[];
  total_number?: number;
  current_page?: number;
  total_pages?: number;
}

export interface PaymentMethod {
  code: string;
  description: string;
  rda_available?: boolean;
}

export interface AlyanteSupplier {
  code: string;
  company_name: string;
}

export interface Country {
  code: string;
  name: string;
}

export interface ProviderReference {
  id?: number;
  first_name?: string;
  last_name?: string;
  email?: string;
  phone?: string;
  reference_type?: string;
}

export interface Provider {
  id: number;
  company_name?: string;
  state?: string;
  default_payment_method?: PaymentMethod | string | number | null;
  vat_number?: string;
  cf?: string;
  address?: string;
  city?: string;
  postal_code?: string;
  province?: string;
  erp_id?: number | null;
  language?: string;
  country?: string;
  ref?: ProviderReference;
  refs?: ProviderReference[];
  skip_qualification_validation?: boolean;
}

export interface DocumentType {
  id: number;
  name: string;
}

export interface CategoryDocumentType {
  document_type: DocumentType;
  required: boolean;
}

export interface Category {
  id: number;
  name: string;
  document_types?: CategoryDocumentType[];
}

export interface ProviderCategory {
  category?: Category;
  status?: string;
  state?: string;
  critical?: boolean;
}

export interface ProviderDocument {
  id: number;
  file_id?: string;
  expire_date?: string;
  provider_id?: number;
  state?: string;
  document_type?: DocumentType;
  source?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ProviderSummary {
  id: number;
  company_name: string | null;
  state: string | null;
  vat_number: string | null;
  cf: string | null;
  erp_id: number | null;
  qualified_count: number;
  total_count: number;
  has_expiring_docs: boolean;
}

export interface DashboardDraft {
  id: number;
  company_name: string | null;
  state: string | null;
  vat_number: string | null;
  cf: string | null;
  erp_id: number | null;
  updated_at: string | null;
}

export interface DashboardDocument {
  id: number;
  provider_id: number;
  company_name: string | null;
  file_id: string | null;
  expire_date: string | null;
  state: string | null;
  document_type: string | null;
  days_remaining: number;
}

export interface DashboardCategory {
  provider_id: number;
  company_name: string | null;
  category_id: number;
  category_name: string | null;
  state: string | null;
  critical: boolean;
}

export interface ArticleCategory {
  article_code: string;
  description: string | null;
  category_id: number;
  category_name: string | null;
}

export interface ProviderPayload {
  company_name: string;
  state: string;
  vat_number?: string;
  cf?: string;
  address?: string;
  city?: string;
  postal_code?: string;
  province?: string;
  erp_id?: number | null;
  language?: string;
  country?: string;
  default_payment_method?: string | number | null;
  skip_qualification_validation?: boolean;
  ref?: ProviderReference;
}

export interface CategoryPayload {
  name: string;
  required_document_type_ids?: number[];
  optional_document_type_ids?: number[];
}
