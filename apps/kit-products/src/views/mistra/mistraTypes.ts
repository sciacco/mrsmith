export interface PaginatedResponse<T> {
  total_number: number;
  current_page: number;
  total_pages: number;
  items: T[];
}

export interface Price {
  nrc: string | null;
  mrc: string | null;
}

export interface DiscountValue {
  percentage: string;
  sign: '+' | '-';
}

export interface KitBrief {
  id: number;
  title: string;
  internal_name: string;
  starter_subscription_time: number;
  regular_subscription_time: number;
  activation_time: number;
  base_price: Price;
  category: string;
  ecommerce: boolean;
}

export interface KitDiscountEntry {
  kit: {
    id: number;
  };
  customer_group: {
    id: number;
    name: string;
  };
  mrc: DiscountValue;
  nrc: DiscountValue;
  sellable: boolean;
  use_int_rounding: boolean;
}

export interface KitDiscountCreateRequest {
  kit_id: number;
  customer_group_id: number;
  sellable: boolean;
  use_int_rounding: boolean;
  mrc: DiscountValue;
  nrc: DiscountValue;
}

export interface CustomerBrief {
  id: number;
  name: string;
  language: string;
  group?: {
    id: number;
    name: string;
  };
  state?: {
    id: number;
    name: string;
  };
}

export interface RelatedProduct {
  id: string;
  title: string;
  min_qty: number;
  max_qty: number;
  price: Price;
  img_url?: string;
}

export interface RelatedProductGroup {
  group_name: string;
  required: boolean;
  products: RelatedProduct[];
}

export interface DiscountedKit extends KitBrief {}

export interface DiscountedKitDetail {
  id: number;
  internal_name: string;
  title: string;
  description: string;
  main_product_code: string;
  bundle_prefix: string;
  ecommerce: boolean;
  is_main_prd_sellable: boolean;
  is_active: boolean;
  starter_subscription_time: number;
  regular_subscription_time: number;
  activation_time: number;
  billing_period: number;
  base_price: Price;
  category: {
    id: number;
    name: string;
  };
  related_products: RelatedProductGroup[];
}
