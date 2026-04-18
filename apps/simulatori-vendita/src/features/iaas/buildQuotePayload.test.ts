import assert from 'node:assert/strict';
import test from 'node:test';
import { buildQuotePayload } from './buildQuotePayload.ts';
import { calculateQuote } from './calculateQuote.ts';
import { pricingTiers } from './pricing.ts';

test('buildQuotePayload preserves Appsmith-compatible keys', () => {
  const calculation = calculateQuote(
    {
      vcpu: '2',
      ram_vmware: '4',
      ram_os: '8',
      storage_pri: '100',
      storage_sec: '50',
      fw_std: '1',
      fw_adv: '0',
      priv_net: '1',
      os_windows: '1',
      ms_sql_std: '0',
    },
    pricingTiers.indiretta.rates,
  );

  const payload = buildQuotePayload(calculation, pricingTiers.indiretta.rates);

  assert.deepEqual(payload.qta, {
    vcpu: 2,
    ram_vmware: 4,
    ram_os: 8,
    storage_pri: 100,
    storage_sec: 50,
    fw_std: 1,
    fw_adv: 0,
    priv_net: 1,
    os_windows: 1,
    ms_sql_std: 0,
  });
  assert.deepEqual(payload.prezzi, {
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
  });
  assert.equal(payload.totale_giornaliero.computing, 1.54);
  assert.ok(Math.abs(payload.totale_giornaliero.storage - 0.15) < 1e-9);
  assert.equal(payload.totale_giornaliero.sicurezza, 0);
  assert.equal(payload.totale_giornaliero.addon, 1);
  assert.equal(payload.totale_giornaliero.totale, 2.69);
});
