import type { PricingTier } from '../../api/types.ts';

export const pricingTiers: Record<PricingTier['code'], PricingTier> = {
  diretta: {
    code: 'diretta',
    label: 'Diretta',
    rates: {
      vcpu: 0.1,
      ram_vmware: 0.3,
      ram_os: 0.1,
      storage_pri: 0.001,
      storage_sec: 0.001,
      fw_std: 0,
      fw_adv: 1.8,
      priv_net: 0,
      os_windows: 1,
      ms_sql_std: 6.33,
    },
  },
  indiretta: {
    code: 'indiretta',
    label: 'Indiretta',
    rates: {
      vcpu: 0.05,
      ram_vmware: 0.2,
      ram_os: 0.08,
      storage_pri: 0.001,
      storage_sec: 0.001,
      fw_std: 0,
      fw_adv: 1.8,
      priv_net: 0,
      os_windows: 1,
      ms_sql_std: 6.33,
    },
  },
};

export const tierOptions = [pricingTiers.diretta, pricingTiers.indiretta];
