import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildDashboardProviderAttention,
  buildDetailProviderAttention,
} from './providerAttention.ts';
import { legacyProviderDetailPath } from './providerRoutes.ts';
import type {
  DashboardCategory,
  DashboardDocument,
  DashboardDraft,
  Provider,
  ProviderCategory,
  ProviderDocument,
} from '../api/types.ts';

test('legacy provider URL maps to detail data section', () => {
  assert.equal(legacyProviderDetailPath('42', null), '/fornitori/42?section=dati');
  assert.equal(legacyProviderDetailPath('42', 'Dati'), '/fornitori/42?section=dati');
});

test('legacy qualifica tab maps to detail qualification section', () => {
  assert.equal(legacyProviderDetailPath('42', 'Qualifica'), '/fornitori/42?section=qualifica');
});

test('dashboard draft is blocking and opens data section', () => {
  const drafts: DashboardDraft[] = [{
    id: 7,
    company_name: 'Alfa',
    state: 'DRAFT',
    vat_number: null,
    cf: null,
    erp_id: null,
    updated_at: null,
  }];

  const [attention] = buildDashboardProviderAttention({ drafts, documents: [], categories: [] });

  assert.equal(attention?.severity, 'blocking');
  assert.equal(attention?.href, '/fornitori/7?section=dati');
  assert.equal(attention?.actionLabel, 'Completa dati');
});

test('dashboard expired document opens document focus', () => {
  const documents: DashboardDocument[] = [{
    id: 91,
    provider_id: 11,
    company_name: 'Beta',
    file_id: null,
    expire_date: '2026-04-20',
    state: 'OK',
    document_type: 'DURC',
    days_remaining: -7,
  }];

  const [attention] = buildDashboardProviderAttention({ drafts: [], documents, categories: [] });

  assert.equal(attention?.severity, 'expired');
  assert.equal(attention?.href, '/fornitori/11?section=documenti&focus=document-91');
  assert.equal(attention?.counts.expiredDocuments, 1);
});

test('dashboard expiring document is warning severity', () => {
  const documents: DashboardDocument[] = [{
    id: 92,
    provider_id: 12,
    company_name: 'Gamma',
    file_id: null,
    expire_date: '2026-05-05',
    state: 'OK',
    document_type: 'Visura',
    days_remaining: 8,
  }];

  const [attention] = buildDashboardProviderAttention({ drafts: [], documents, categories: [] });

  assert.equal(attention?.severity, 'expiring');
  assert.equal(attention?.counts.expiringDocuments, 1);
});

test('critical category is blocking and opens qualification focus', () => {
  const categories: DashboardCategory[] = [{
    provider_id: 13,
    company_name: 'Delta',
    category_id: 5,
    category_name: 'Network',
    state: 'NOT_QUALIFIED',
    critical: true,
  }];

  const [attention] = buildDashboardProviderAttention({ drafts: [], documents: [], categories });

  assert.equal(attention?.severity, 'blocking');
  assert.equal(attention?.href, '/fornitori/13?section=qualifica&focus=category-5');
  assert.equal(attention?.counts.criticalCategories, 1);
});

test('detail provider without open work has no anomalies', () => {
  const provider: Provider = {
    id: 21,
    company_name: 'Epsilon',
    state: 'ACTIVE',
    default_payment_method: '401',
    address: 'Via Roma',
    city: 'Milano',
    postal_code: '20100',
    country: 'IT',
    erp_id: 300,
    refs: [{ email: 'ops@example.test' }],
  };
  const providerCategories: ProviderCategory[] = [{
    category: { id: 1, name: 'Core' },
    state: 'QUALIFIED',
    critical: false,
  }];
  const providerDocuments: ProviderDocument[] = [{
    id: 200,
    expire_date: '2026-09-01',
    document_type: { id: 9, name: 'DURC' },
    state: 'OK',
  }];

  const attention = buildDetailProviderAttention({
    provider,
    providerCategories,
    providerDocuments,
    today: new Date('2026-04-27T10:00:00'),
  });

  assert.equal(attention.severity, 'none');
  assert.equal(attention.openCount, 0);
  assert.deepEqual(attention.actions, []);
});
