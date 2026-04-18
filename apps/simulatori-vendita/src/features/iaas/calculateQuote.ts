import type {
  CalculationResult,
  QuantityFormValues,
  ResourceKey,
  ResourceValues,
} from '../../api/types.ts';
import { createQuantityFormValues, resourceCatalog, resourceOrder } from './resourceCatalog.ts';

const MONTHLY_MULTIPLIER = 30;

function clampValue(key: ResourceKey, value: number): number {
  const config = resourceCatalog[key];
  const min = config.min;
  const max = config.max;

  let nextValue = Number.isFinite(value) ? value : config.defaultValue;
  nextValue = Math.trunc(nextValue);
  if (nextValue < min) nextValue = min;
  if (max !== undefined && nextValue > max) nextValue = max;
  return nextValue;
}

export function normalizeQuantityValue(key: ResourceKey, rawValue: string): string {
  const trimmedValue = rawValue.trim();
  if (trimmedValue === '') {
    return String(resourceCatalog[key].min);
  }

  const parsedValue = Number(trimmedValue);
  return String(clampValue(key, parsedValue));
}

export function normalizeQuantityForm(formValues: QuantityFormValues): QuantityFormValues {
  const normalizedValues = {} as QuantityFormValues;

  for (const key of resourceOrder) {
    normalizedValues[key] = normalizeQuantityValue(key, formValues[key]);
  }

  return normalizedValues;
}

export function parseQuantityForm(formValues: QuantityFormValues): ResourceValues {
  return {
    vcpu: clampValue('vcpu', Number(formValues.vcpu)),
    ram_vmware: clampValue('ram_vmware', Number(formValues.ram_vmware)),
    ram_os: clampValue('ram_os', Number(formValues.ram_os)),
    storage_pri: clampValue('storage_pri', Number(formValues.storage_pri)),
    storage_sec: clampValue('storage_sec', Number(formValues.storage_sec)),
    fw_std: clampValue('fw_std', Number(formValues.fw_std)),
    fw_adv: clampValue('fw_adv', Number(formValues.fw_adv)),
    priv_net: clampValue('priv_net', Number(formValues.priv_net)),
    os_windows: clampValue('os_windows', Number(formValues.os_windows)),
    ms_sql_std: clampValue('ms_sql_std', Number(formValues.ms_sql_std)),
  };
}

export function calculateQuote(
  formValues: QuantityFormValues,
  rates: ResourceValues,
): CalculationResult {
  const normalizedFormValues = normalizeQuantityForm(formValues);
  const normalizedQuantities = parseQuantityForm(normalizedFormValues);
  const lineTotals: ResourceValues = {
    vcpu: normalizedQuantities.vcpu * rates.vcpu,
    ram_vmware: normalizedQuantities.ram_vmware * rates.ram_vmware,
    ram_os: normalizedQuantities.ram_os * rates.ram_os,
    storage_pri: normalizedQuantities.storage_pri * rates.storage_pri,
    storage_sec: normalizedQuantities.storage_sec * rates.storage_sec,
    fw_std: normalizedQuantities.fw_std * rates.fw_std,
    fw_adv: normalizedQuantities.fw_adv * rates.fw_adv,
    priv_net: normalizedQuantities.priv_net * rates.priv_net,
    os_windows: normalizedQuantities.os_windows * rates.os_windows,
    ms_sql_std: normalizedQuantities.ms_sql_std * rates.ms_sql_std,
  };

  const computing = lineTotals.vcpu + lineTotals.ram_vmware + lineTotals.ram_os;
  const storage = lineTotals.storage_pri + lineTotals.storage_sec;
  const sicurezza = lineTotals.fw_std + lineTotals.fw_adv + lineTotals.priv_net;
  const addon = lineTotals.os_windows + lineTotals.ms_sql_std;
  const totale = computing + storage + sicurezza + addon;

  return {
    normalizedFormValues,
    normalizedQuantities,
    lineTotals,
    dailyTotals: {
      computing,
      storage,
      sicurezza,
      addon,
      totale,
    },
    monthlyTotal: totale * MONTHLY_MULTIPLIER,
  };
}

export function resetQuoteForm(): QuantityFormValues {
  return createQuantityFormValues();
}
