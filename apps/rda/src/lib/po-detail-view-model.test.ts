import type { PoActionModel, PoDetail, ProviderSummary } from '../api/types.js';
import { buildPOReadinessItems, buildTabBadges, selectedModeID, type POHeaderState } from './po-detail-view-model.js';

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

const completeHeader: POHeaderState = {
  budget_id: 10,
  object: 'Notebook',
  project: 'IT',
  provider_id: 20,
  payment_method: 'SUP',
  currency: 'EUR',
  provider_offer_code: '',
  provider_offer_date: '',
  description: '',
  note: '',
};

function poFixture(overrides: Partial<PoDetail> = {}): PoDetail {
  return {
    id: 42,
    state: 'DRAFT',
    total_price: '3500.00',
    currency: 'EUR',
    requester: { email: 'user@example.com' },
    provider: { id: 20 },
    rows: [{ id: 1, type: 'good', qty: 1, total_price: '3500.00' }],
    attachments: [{ id: 1, attachment_type: 'quote' }],
    recipients: [],
    ...overrides,
  };
}

test('selected mode prefers current valid mode then backend primary', () => {
  const model: PoActionModel = {
    permission_status: 'available',
    primary_mode_id: 'requester_payment_update',
    workflow_stage: 'method_budget',
    summary: { row_count: 1, attachment_count: 0, quote_count: 0, recipient_count: 0 },
    modes: [
      { id: 'requester_payment_update', label: 'Richiedente', description: '', action_ids: [] },
      { id: 'afc_payment', label: 'AFC', description: '', action_ids: ['payment-method/approve'] },
    ],
    actions: [],
  };

  assertEqual(selectedModeID(model, 'afc_payment'), 'afc_payment', 'current valid mode');
  assertEqual(selectedModeID(model, 'missing'), 'requester_payment_update', 'fallback to backend primary');
});

test('readiness blocks draft submit when quote threshold is not satisfied', () => {
  const items = buildPOReadinessItems(poFixture(), completeHeader, { quoteThreshold: 3000 });
  const quotes = items.find((item) => item.id === 'quotes');

  assert(quotes != null, 'quotes readiness item should exist');
  assertEqual(quotes?.ready, false, 'one quote should not satisfy high-value rule');
});

test('readiness accepts qualification fallback for contacts', () => {
  const provider: ProviderSummary = {
    id: 20,
    ref: { id: 30, reference_type: 'QUALIFICATION_REF', email: 'qualifica@example.com' },
  };
  const items = buildPOReadinessItems(poFixture(), completeHeader, { provider, quoteThreshold: 5000 });
  const contacts = items.find((item) => item.id === 'contacts');

  assertEqual(contacts?.ready, true, 'qualification contact should make recipients ready');
});

test('tab badges expose quote progress, row total, notes dirty, and contacts fallback', () => {
  const provider: ProviderSummary = {
    id: 20,
    ref: { id: 30, reference_type: 'QUALIFICATION_REF', email: 'qualifica@example.com' },
  };
  const header = { ...completeHeader, note: 'Nuova nota' };
  const badges = buildTabBadges(poFixture(), header, completeHeader, provider, 3000);

  assertEqual(badges.attachments, '1/2 prev.', 'quote progress badge');
  assert(badges.rows.includes('1 - '), 'row badge should include count and total');
  assertEqual(badges.notesDirty, true, 'notes dirty badge');
  assertEqual(badges.contacts, 'Qualifica', 'qualification fallback badge');
});

console.log('po detail view-model tests passed');
