import { canManageProviderContacts, isValidOptionalProviderRefPhone } from './provider-refs.js';

function assertEqual(actual: unknown, expected: unknown, message: string) {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

assertEqual(isValidOptionalProviderRefPhone(''), true, 'empty phone should be valid');
assertEqual(isValidOptionalProviderRefPhone('+391234567890'), true, 'E.164-like phone should be valid');
assertEqual(isValidOptionalProviderRefPhone('+39 1234567890'), false, 'phone with spaces should be invalid');
assertEqual(isValidOptionalProviderRefPhone('391234567890'), false, 'phone without plus should be invalid');

assertEqual(
  canManageProviderContacts({ id: 42, type: 'STANDARD' }, { id: 7 }),
  true,
  'standard PO contacts should be manageable when provider detail is loaded',
);
assertEqual(
  canManageProviderContacts({ id: 42, type: 'STANDARD' }, null),
  false,
  'contacts should wait for provider detail',
);
assertEqual(
  canManageProviderContacts({ id: 42, type: 'ECOMMERCE' }, { id: 7 }),
  false,
  'e-commerce PO contacts should not show recipient management',
);
