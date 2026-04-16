export interface AssetFlow {
  name: string;
  label: string;
}

export interface CustomFieldKey {
  key_name: string;
  key_description: string;
}

export interface LanguageOption {
  iso: string;
  name: string;
}

export interface VocabularyItem {
  label: string;
  value: string;
}

export interface ProductCategory {
  id: number;
  name: string;
  color: string;
}

export interface ProductCategoryWriteRequest {
  name: string;
  color: string;
}

export interface CustomerGroup {
  id: number;
  name: string;
  is_default: boolean;
  is_partner: boolean;
  read_only: boolean;
  base_discount: number | null;
}

export interface CustomerGroupCreateRequest {
  name: string;
  is_partner: boolean;
}

export interface CustomerGroupBatchUpdateRequest {
  items: Array<{
    id: number;
    name: string;
    is_partner: boolean;
  }>;
}

export interface Translation {
  language: 'it' | 'en';
  short: string;
  long: string;
}

export interface ProductGroupTranslation {
  language: string;
  short: string;
  long: string;
}

export interface ProductGroup {
  name: string;
  translation_uuid: string;
  usage_count: number;
  translations: ProductGroupTranslation[];
}

export interface ProductGroupWriteRequest {
  name: string;
  translations: ProductGroupTranslation[];
  confirm_propagation?: boolean;
}

export interface ProductGroupRenameConflict {
  error: 'rename_confirmation_required';
  impacted_kit_products: number;
  quotes_unchanged: boolean;
}

export interface Product {
  code: string;
  internal_name: string;
  category_id: number;
  category_name: string;
  category_color: string;
  translation_uuid: string;
  nrc: number;
  mrc: number;
  img_url: string | null;
  erp_sync: boolean;
  asset_flow: string | null;
  translations: Translation[];
}

export interface ProductCreateRequest {
  code: string;
  internal_name: string;
  category_id: number;
  nrc: number;
  mrc: number;
  img_url: string | null;
  erp_sync: boolean;
  asset_flow: string | null;
  translations: Translation[];
}

export interface ProductUpdateRequest {
  internal_name: string;
  category_id: number;
  nrc: number;
  mrc: number;
  img_url: string | null;
  erp_sync: boolean;
  asset_flow: string | null;
}

export interface TranslationUpdateResponse {
  data: Product;
  warning?: {
    code: string;
    message: string;
  };
}
