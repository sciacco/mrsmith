export type TierCode = 'diretta' | 'indiretta';

export type ResourceKey =
  | 'vcpu'
  | 'ram_vmware'
  | 'ram_os'
  | 'storage_pri'
  | 'storage_sec'
  | 'fw_std'
  | 'fw_adv'
  | 'priv_net'
  | 'os_windows'
  | 'ms_sql_std';

export type ResourceGroup = 'Computing' | 'Storage' | 'Sicurezza' | 'Add On';

export type QuantityFormValues = Record<ResourceKey, string>;

export interface ResourceValues {
  vcpu: number;
  ram_vmware: number;
  ram_os: number;
  storage_pri: number;
  storage_sec: number;
  fw_std: number;
  fw_adv: number;
  priv_net: number;
  os_windows: number;
  ms_sql_std: number;
}

export interface DailyTotals {
  computing: number;
  storage: number;
  sicurezza: number;
  addon: number;
  totale: number;
}

export interface PayloadTotals extends DailyTotals {
  mese: number;
}

export interface QuotePayload {
  qta: ResourceValues;
  prezzi: ResourceValues;
  totale_giornaliero: PayloadTotals;
}

export interface CalculationResult {
  normalizedFormValues: QuantityFormValues;
  normalizedQuantities: ResourceValues;
  lineTotals: ResourceValues;
  dailyTotals: DailyTotals;
  monthlyTotal: number;
}

export interface PricingTier {
  code: TierCode;
  label: string;
  rates: ResourceValues;
}
