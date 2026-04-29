import {
  buildPaymentMethodOptions,
  preferredPaymentMethodCode,
  requiresPaymentMethodVerification,
} from './payment-options.js';
import type { PaymentMethod, ProviderSummary } from '../api/types';

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
}

function test(name: string, run: () => void) {
  try {
    run();
  } catch (error) {
    throw new Error(`${name}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

const catalog: PaymentMethod[] = [
  { code: 'WIRE', description: 'Bonifico bancario', rda_available: true },
  { code: 'CARD', description: 'Carta aziendale', rda_available: true },
  { code: 'OLD', description: 'Metodo legacy', rda_available: false },
];

test('supplier default is first and selected by default', () => {
  const provider: ProviderSummary = {
    id: 7,
    default_payment_method: { code: 'WIRE', description: 'Bonifico bancario' },
  };
  const options = buildPaymentMethodOptions({
    methods: catalog,
    providerDefault: provider.default_payment_method,
    cdlanDefaultCode: 'CARD',
  });

  assertEqual(options[0]?.code, 'WIRE', 'supplier default should be first');
  assert(options[0]?.isProviderDefault === true, 'supplier default should be tagged');
  assertEqual(preferredPaymentMethodCode(provider, 'CARD'), 'WIRE', 'supplier default should be preferred');
});

test('CDLAN default is second and distinct when different', () => {
  const options = buildPaymentMethodOptions({
    methods: catalog,
    providerDefault: 'WIRE',
    cdlanDefaultCode: 'CARD',
  });

  assertEqual(options[1]?.code, 'CARD', 'CDLAN default should be second');
  assert(options[1]?.isCdlanDefault === true, 'CDLAN default should be tagged');
  assert(options[0]?.code !== options[1]?.code, 'supplier and CDLAN defaults should stay distinct');
});

test('duplicate codes merge into one option', () => {
  const options = buildPaymentMethodOptions({
    methods: [
      { code: 'CARD', description: 'Carta', rda_available: true },
      { code: 'CARD', description: 'Carta aggiornata', rda_available: true },
      { code: 'WIRE', description: 'Bonifico', rda_available: true },
    ],
    providerDefault: 'CARD',
    cdlanDefaultCode: 'CARD',
  });

  assertEqual(options.filter((option) => option.code === 'CARD').length, 1, 'duplicate code should appear once');
  assert(options[0]?.badges.includes('provider-default') === true, 'merged option should keep provider badge');
  assert(options[0]?.badges.includes('cdlan-default') === true, 'merged option should keep CDLAN badge');
});

test('non-RDA supplier default remains available', () => {
  const options = buildPaymentMethodOptions({
    methods: catalog,
    providerDefault: { code: 'OLD', description: 'Metodo legacy', rda_available: false },
    cdlanDefaultCode: 'CARD',
  });

  assertEqual(options[0]?.code, 'OLD', 'non-RDA supplier default should be included first');
  assert(options[0]?.isProviderDefault === true, 'non-RDA supplier default should be tagged');
  assertEqual(options[0]?.label, 'Metodo legacy', 'non-RDA supplier default should keep the plain label');
  assert(options[0]?.isNotPreapproved === true, 'non-RDA supplier default should be marked separately');
});

test('warning applies only outside supplier and CDLAN defaults', () => {
  assert(!requiresPaymentMethodVerification('WIRE', 'WIRE', 'CARD'), 'supplier default should not warn');
  assert(!requiresPaymentMethodVerification('CARD', 'WIRE', 'CARD'), 'CDLAN default should not warn');
  assert(requiresPaymentMethodVerification('MANUAL', 'WIRE', 'CARD'), 'non-standard method should warn');
});

console.log('payment-options tests passed');
