import test from 'node:test';
import assert from 'node:assert/strict';
import { providerStateValues } from './providerState.ts';

test('draft provider without ERP can only remain draft', () => {
  assert.deepEqual(providerStateValues('DRAFT', ''), ['DRAFT']);
  assert.deepEqual(providerStateValues('DRAFT', null), ['DRAFT']);
});

test('draft provider with ERP can move to Appsmith editable states', () => {
  assert.deepEqual(providerStateValues('DRAFT', '123'), ['DRAFT', 'ACTIVE', 'INACTIVE']);
});

test('active provider can only stay active or become inactive', () => {
  assert.deepEqual(providerStateValues('ACTIVE', '123'), ['ACTIVE', 'INACTIVE']);
});

test('ceased is never exposed as selectable state', () => {
  assert.deepEqual(providerStateValues('CEASED', '123'), []);
});
