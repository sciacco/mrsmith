import assert from 'node:assert/strict';
import test from 'node:test';
import { calculateQuote, normalizeQuantityForm } from './calculateQuote.ts';
import { pricingTiers } from './pricing.ts';

test('calculateQuote groups line totals and monthly total for Diretta', () => {
  const result = calculateQuote(
    {
      vcpu: '1',
      ram_vmware: '10',
      ram_os: '0',
      storage_pri: '100',
      storage_sec: '100',
      fw_std: '0',
      fw_adv: '1',
      priv_net: '0',
      os_windows: '0',
      ms_sql_std: '1',
    },
    pricingTiers.diretta.rates,
  );

  assert.deepEqual(result.normalizedQuantities, {
    vcpu: 1,
    ram_vmware: 10,
    ram_os: 0,
    storage_pri: 100,
    storage_sec: 100,
    fw_std: 0,
    fw_adv: 1,
    priv_net: 0,
    os_windows: 0,
    ms_sql_std: 1,
  });
  assert.equal(result.dailyTotals.computing, 3.1);
  assert.equal(result.dailyTotals.storage, 0.2);
  assert.equal(result.dailyTotals.sicurezza, 1.8);
  assert.equal(result.dailyTotals.addon, 6.33);
  assert.equal(result.dailyTotals.totale, 11.43);
  assert.equal(result.monthlyTotal, 342.9);
});

test('normalizeQuantityForm clamps required minimums and fw_adv max', () => {
  const normalized = normalizeQuantityForm({
    vcpu: '',
    ram_vmware: '',
    ram_os: '3',
    storage_pri: '',
    storage_sec: '25',
    fw_std: '',
    fw_adv: '7',
    priv_net: '',
    os_windows: '',
    ms_sql_std: '2',
  });

  assert.deepEqual(normalized, {
    vcpu: '1',
    ram_vmware: '0',
    ram_os: '3',
    storage_pri: '10',
    storage_sec: '25',
    fw_std: '0',
    fw_adv: '1',
    priv_net: '0',
    os_windows: '0',
    ms_sql_std: '2',
  });
});
