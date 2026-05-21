import type { BudgetForUser } from '../api/types.js';
import { budgetDisplayLabel, budgetSelectionKey, findBudget, selectedBudgetID } from './budgets.js';
import { buildPatchPOPayload, type POHeaderDraft } from './po-payload.js';

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

const dataCenterBudget: BudgetForUser = {
  budget_id: 50094,
  name: 'Manutenzioni',
  year: 2026,
  cost_center: 'Data Center',
};

const officeBudget: BudgetForUser = {
  id: 50094,
  name: 'Manutenzioni',
  year: 2026,
  cost_center: 'Office - TR',
};

const header: POHeaderDraft = {
  type: 'STANDARD',
  budget_id: '',
  object: 'pulizia uffici',
  project: 'pulizia uffici',
  provider_id: 12,
  payment_method: 'RB30',
  currency: 'EUR',
  provider_offer_code: '',
  provider_offer_date: '',
  description: '',
  note: '',
};

test('duplicate budget ids resolve only with the exact cost center binding', () => {
  const selected = budgetSelectionKey(officeBudget);
  const match = findBudget([dataCenterBudget, officeBudget], selected);

  assertEqual(match?.cost_center, 'Office - TR', 'exact composite budget match');
});

test('composite budget selection does not fall back to another same-id cost center', () => {
  const selected = budgetSelectionKey(officeBudget);

  assert(findBudget([dataCenterBudget], selected) === undefined, 'same budget id with different cost center must not match');
  assertEqual(selectedBudgetID(selected, [dataCenterBudget]), 50094, 'selection still carries the numeric budget id');
});

test('plain budget id selection does not resolve ambiguous duplicate budgets', () => {
  assert(findBudget([dataCenterBudget, officeBudget], 50094) === undefined, 'duplicate plain budget id must not pick the first budget');
});

test('PO display label uses the saved budget association', () => {
  assertEqual(budgetDisplayLabel(officeBudget), 'Manutenzioni (Office - TR)', 'PO budget display');
});

test('patch payload preserves a composite cost center binding even when not in viewer budgets', () => {
  const selected = budgetSelectionKey(officeBudget);
  const payload = buildPatchPOPayload({ ...header, budget_id: selected }, [dataCenterBudget]);

  assertEqual(payload.budget_id, 50094, 'payload budget id');
  assertEqual(payload.cost_center, 'Office - TR', 'payload cost center');
  assertEqual(payload.budget_user_id, undefined, 'payload must not send budget user id for cost center budgets');
});

test('patch payload preserves a composite user binding when not in viewer budgets', () => {
  const userBudget: BudgetForUser = {
    budget_id: 42,
    name: 'Trasferte',
    year: 2026,
    user_id: 321,
    user_email: 'utente@example.com',
  };
  const selected = budgetSelectionKey(userBudget);
  const payload = buildPatchPOPayload({ ...header, budget_id: selected }, []);

  assertEqual(payload.budget_id, 42, 'payload user budget id');
  assertEqual(payload.budget_user_id, 321, 'payload budget user id');
  assertEqual(payload.cost_center, null, 'payload clears cost center when switching to user budget');
});

console.log('budget selection tests passed');
