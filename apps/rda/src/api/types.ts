export interface PagedEnvelope<T> {
  items?: T[];
  total_number?: number;
  total_items?: number;
  current_page?: number;
  total_pages?: number;
}

export interface RdaUser {
  id?: number;
  first_name?: string;
  last_name?: string;
  name?: string;
  email?: string;
}

export interface ProviderReference {
  id?: number;
  first_name?: string;
  last_name?: string;
  email?: string;
  phone?: string;
  reference_type?: string;
}

export interface ProviderSummary {
  id: number;
  company_name?: string;
  state?: string;
  default_payment_method?: PaymentMethod | string | number | null;
  language?: string;
  vat_number?: string;
  refs?: ProviderReference[];
  ref?: ProviderReference;
}

export interface BudgetForUser {
  budget_id?: number;
  id?: number;
  name?: string;
  year?: number;
  cost_center?: string | null;
  budget_user_id?: number | null;
  user_id?: number | null;
}

export interface PaymentMethod {
  code: string;
  description: string;
  rda_available?: boolean;
}

export interface Country {
  code: string;
  name: string;
}

export interface DefaultPaymentMethod {
  code: string;
}

export type CurrencyCode = 'EUR' | 'USD' | 'GBP';

export interface Article {
  code: string;
  description?: string;
  type: 'good' | 'service';
}

export interface RdaPermissions {
  is_approver: boolean;
  is_afc: boolean;
  is_approver_no_leasing: boolean;
  is_approver_extra_budget: boolean;
}

export interface PoApprover {
  level?: string | number;
  user?: RdaUser;
}

export interface PoAttachment {
  id: number;
  attachment_type?: string;
  file_id?: string;
  file_name?: string;
  created_at?: string;
  created?: string;
  updated_at?: string;
}

export interface PoRow {
  id: number;
  type?: 'good' | 'service' | string;
  description?: string;
  product_code?: string;
  product_description?: string;
  qty?: number | string;
  price?: number | string;
  montly_fee?: number | string;
  monthly_fee?: number | string;
  activation_fee?: number | string;
  activation_price?: number | string;
  total_price?: number | string;
  payment_detail?: {
    start_at?: string;
    start_pay_at_activation_date?: string;
    start_at_date?: string;
    month_recursion?: number | string;
  };
  renew_detail?: {
    initial_subscription_months?: number | string;
    next_subscription_months?: number | string;
    automatic_renew?: boolean;
    cancellation_advice?: string;
  };
}

export interface PoPreview {
  id: number;
  code?: string;
  state?: string;
  current_approval_level?: string | number;
  project?: string;
  object?: string;
  total_price?: string;
  currency?: CurrencyCode | string;
  created?: string;
  creation_date?: string;
  updated?: string;
  requester?: RdaUser;
  provider?: ProviderSummary;
  payment_method?: PaymentMethod | string | null;
  approvers?: PoApprover[];
  budget_increment_needed?: number | string;
}

export interface PoDetail extends PoPreview {
  type?: string;
  language?: string;
  description?: string;
  note?: string;
  provider_offer_code?: string;
  provider_offer_date?: string;
  reference_warehouse?: string;
  budget?: BudgetForUser;
  rows?: PoRow[];
  attachments?: PoAttachment[];
  recipients?: ProviderReference[];
}

export interface PoComment {
  id: number;
  user?: RdaUser;
  comment?: string;
  comment_text?: string;
  created_at?: string;
  created?: string;
  replies?: PoComment[];
}

export interface CreatePOPayload {
  type: 'STANDARD' | 'ECOMMERCE';
  budget_id: number;
  cost_center?: string;
  budget_user_id?: number;
  provider_id: number;
  payment_method: string;
  currency: CurrencyCode;
  project: string;
  object: string;
  description?: string;
  note?: string;
  provider_offer_code?: string;
  provider_offer_date?: string;
}

export interface PatchPOPayload {
  type?: string;
  budget_id?: number;
  budget_user_id?: number;
  cost_center?: string | null;
  description?: string;
  object?: string;
  note?: string;
  payment_method?: string;
  currency?: CurrencyCode;
  reference_warehouse?: string;
  provider_id?: number;
  project?: string;
  provider_offer_code?: string;
  provider_offer_date?: string;
  recipient_ids?: number[];
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
  ref?: ProviderReference;
}

export interface RowPayload {
  type: 'good' | 'service';
  description: string;
  qty: number;
  product_code: string;
  product_description: string;
  price?: number;
  monthly_fee?: number;
  montly_fee?: number;
  activation_price?: number;
  payment_detail: {
    start_at: string;
    start_at_date?: string;
    month_recursion?: number;
  };
  renew_detail?: {
    initial_subscription_months?: number;
    automatic_renew?: boolean;
    cancellation_advice?: string;
  };
}
