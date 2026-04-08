// ── Mistra entities ──

export interface Customer {
  id: number;
  name: string;
}

export interface Kit {
  id: number;
  internal_name: string;
  billing_period: string;
  initial_subscription_months: number;
  next_subscription_months: number;
  activation_time_days: number;
  category_id: number;
  category_name: string;
  category_color: string;
  is_main_prd_sellable: boolean;
  sconto_massimo: number;
  variable_billing: boolean;
  h24_assurance: boolean;
  sla_resolution_hours: number;
  notes: string | null;
}

export interface KitProduct {
  group_name: string | null;
  internal_name: string;
  nrc: number;
  mrc: number;
  minimum: number;
  maximum: number | null;
  required: boolean;
  position: number;
  product_code: string;
  notes: string | null;
}

export interface CustomerGroup {
  id: number;
  name: string;
}

export interface KitGroupDiscount {
  kit_id: number;
  kit_name: string;
  discount_mrc: number;
  discount_nrc: number;
}

export interface CreditBalance {
  balance: number;
}

export interface CreditTransaction {
  id: number;
  transaction_date: string;
  amount: number;
  operation_sign: '+' | '-';
  description: string;
  operated_by: string;
}

export interface TransactionRequest {
  amount: number;
  operation_sign: '+' | '-';
  description: string;
}

export interface TimooPricing {
  user_month: number;
  se_month: number;
  is_default: boolean;
}

export interface TimooPricingRequest {
  user_month: number;
  se_month: number;
}

// ── Grappa entities ──

export interface GrappaCustomer {
  id: number;
  intestazione: string;
  codice_aggancio_gest: number;
}

export interface IaaSPricing {
  charge_cpu: number;
  charge_ram_kvm: number;
  charge_ram_vmware: number;
  charge_pstor: number;
  charge_sstor: number;
  charge_ip: number;
  charge_prefix24: number | null;
  is_default: boolean;
}

export interface IaaSPricingRequest {
  charge_cpu: number;
  charge_ram_kvm: number;
  charge_ram_vmware: number;
  charge_pstor: number;
  charge_sstor: number;
  charge_ip: number;
  charge_prefix24?: number;
}

export interface IaaSAccount {
  domainuuid: string;
  id_cli_fatturazione: number;
  intestazione: string;
  abbreviazione: string;
  serialnumber: string;
  codice_ordine: string;
  data_attivazione: string;
  credito: number;
  infrastructure_platform: string;
}

export interface IaaSCreditUpdateItem {
  domainuuid: string;
  id_cli_fatturazione: number;
  credito: number;
}

export interface Rack {
  id_rack: number;
  name: string;
  building: string;
  room: string;
  floor: number | null;
  island: number | null;
  type: string | null;
  sconto: number;
}

export interface RackDiscountUpdateItem {
  id_rack: number;
  sconto: number;
}
