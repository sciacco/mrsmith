import type {
  QuantityFormValues,
  ResourceGroup,
  ResourceKey,
  ResourceValues,
} from '../../api/types.ts';

export interface ResourceDefinition {
  key: ResourceKey;
  label: string;
  group: ResourceGroup;
  min: number;
  max?: number;
  step: number;
  defaultValue: number;
}

export const resourceOrder: ResourceKey[] = [
  'vcpu',
  'ram_vmware',
  'ram_os',
  'storage_pri',
  'storage_sec',
  'fw_std',
  'fw_adv',
  'priv_net',
  'os_windows',
  'ms_sql_std',
];

export const defaultQuantities: ResourceValues = {
  vcpu: 1,
  ram_vmware: 0,
  ram_os: 0,
  storage_pri: 100,
  storage_sec: 100,
  fw_std: 0,
  fw_adv: 0,
  priv_net: 0,
  os_windows: 0,
  ms_sql_std: 0,
};

export const resourceCatalog: Record<ResourceKey, ResourceDefinition> = {
  vcpu: {
    key: 'vcpu',
    label: 'vCPU (1 GHz min)',
    group: 'Computing',
    min: 1,
    step: 1,
    defaultValue: 1,
  },
  ram_vmware: {
    key: 'ram_vmware',
    label: 'RAM VMware (GB)',
    group: 'Computing',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
  ram_os: {
    key: 'ram_os',
    label: 'RAM KVM Linux (GB)',
    group: 'Computing',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
  storage_pri: {
    key: 'storage_pri',
    label: 'Primary Storage (GB)',
    group: 'Storage',
    min: 10,
    step: 1,
    defaultValue: 100,
  },
  storage_sec: {
    key: 'storage_sec',
    label: 'Secondary Storage (GB)',
    group: 'Storage',
    min: 0,
    step: 1,
    defaultValue: 100,
  },
  fw_std: {
    key: 'fw_std',
    label: 'Firewall standard',
    group: 'Sicurezza',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
  fw_adv: {
    key: 'fw_adv',
    label: 'Firewall advanced',
    group: 'Sicurezza',
    min: 0,
    max: 1,
    step: 1,
    defaultValue: 0,
  },
  priv_net: {
    key: 'priv_net',
    label: 'Private network',
    group: 'Sicurezza',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
  os_windows: {
    key: 'os_windows',
    label: 'O.S. Windows Server',
    group: 'Add On',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
  ms_sql_std: {
    key: 'ms_sql_std',
    label: 'MS SQL Server std',
    group: 'Add On',
    min: 0,
    step: 1,
    defaultValue: 0,
  },
};

export const resourceGroups: Array<{ id: ResourceGroup; title: ResourceGroup; keys: ResourceKey[] }> = [
  {
    id: 'Computing',
    title: 'Computing',
    keys: ['vcpu', 'ram_vmware', 'ram_os'],
  },
  {
    id: 'Storage',
    title: 'Storage',
    keys: ['storage_pri', 'storage_sec'],
  },
  {
    id: 'Sicurezza',
    title: 'Sicurezza',
    keys: ['fw_std', 'fw_adv', 'priv_net'],
  },
  {
    id: 'Add On',
    title: 'Add On',
    keys: ['os_windows', 'ms_sql_std'],
  },
];

export function createQuantityFormValues(values: ResourceValues = defaultQuantities): QuantityFormValues {
  return {
    vcpu: String(values.vcpu),
    ram_vmware: String(values.ram_vmware),
    ram_os: String(values.ram_os),
    storage_pri: String(values.storage_pri),
    storage_sec: String(values.storage_sec),
    fw_std: String(values.fw_std),
    fw_adv: String(values.fw_adv),
    priv_net: String(values.priv_net),
    os_windows: String(values.os_windows),
    ms_sql_std: String(values.ms_sql_std),
  };
}
