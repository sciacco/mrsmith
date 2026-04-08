import type { Translation } from '../../api/types';

export interface KitSummary {
  id: number;
  internal_name: string;
  main_product_code: string | null;
  category_id: number;
  category_name: string;
  category_color: string;
  bundle_prefix: string | null;
  initial_subscription_months: number;
  next_subscription_months: number;
  activation_time_days: number;
  nrc: number;
  mrc: number;
  translation_uuid: string;
  ecommerce: boolean;
  is_active: boolean;
  is_main_prd_sellable: boolean | null;
  billing_period: number;
  sconto_massimo: number;
  variable_billing: boolean;
  h24_assurance: boolean;
  sla_resolution_hours: number;
  notes: string | null;
}

export interface KitDetail extends KitSummary {
  help_url: string | null;
  translations: Translation[];
  sellable_group_ids: number[];
  sellable_groups?: Array<{
    id: number;
    name: string;
  }>;
}

export interface KitWriteRequest {
  internal_name: string;
  main_product_code: string | null;
  category_id: number;
  bundle_prefix?: string | null;
  initial_subscription_months: number;
  next_subscription_months: number;
  activation_time_days: number;
  nrc: number;
  mrc: number;
  ecommerce: boolean;
  is_active: boolean;
  is_main_prd_sellable: boolean;
  billing_period: number;
  sconto_massimo: number;
  variable_billing: boolean;
  h24_assurance: boolean;
  sla_resolution_hours: number;
  notes: string | null;
  sellable_group_ids: number[];
}

export interface KitCreateRequest {
  internal_name: string;
  main_product_code: string | null;
  category_id: number;
  bundle_prefix: string;
  initial_subscription_months: number;
  next_subscription_months: number;
  activation_time_days: number;
  nrc: number;
  mrc: number;
  ecommerce: boolean;
}

export interface KitCloneRequest {
  name: string;
}

export interface KitCreateResponse {
  id?: number;
  kit_id?: number;
}

export interface KitProductItem {
  id: number;
  kit_id: number;
  product_code: string;
  name?: string | null;
  product_internal_name?: string | null;
  product_name?: string | null;
  group_name: string | null;
  minimum: number;
  maximum: number;
  required: boolean;
  nrc: number;
  mrc: number;
  position: number;
  notes: string | null;
}

export interface KitProductWriteRequest {
  product_code: string;
  group_name: string | null;
  minimum: number;
  maximum: number;
  required: boolean;
  nrc: number;
  mrc: number;
  position: number;
  notes: string | null;
}

export interface KitCustomValueItem {
  id: number;
  kit_id: number;
  key_name: string;
  value: unknown;
}

export interface KitCustomValueWriteRequest {
  key_name: string;
  value: unknown;
}
