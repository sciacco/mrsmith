import type { PoPreview, RdaPermissions } from '../api/types.js';
import { buildRdaDashboardModel, filterRdaDashboardRows } from './rda-dashboard.js';

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

function poFixture(overrides: Partial<PoPreview> = {}): PoPreview {
  return {
    id: 10,
    code: 'PO-10',
    state: 'PENDING_APPROVAL',
    object: 'Notebook',
    project: 'IT',
    created: '2026-04-01T09:00:00Z',
    requester: { email: 'owner@example.com' },
    provider: { id: 20, company_name: 'Acme SRL' },
    total_price: '1200',
    currency: 'EUR',
    ...overrides,
  };
}

function permissionsFixture(overrides: Partial<RdaPermissions> = {}): RdaPermissions {
  return {
    is_approver: false,
    is_afc: false,
    is_approver_no_leasing: false,
    is_approver_extra_budget: false,
    can_see_all_po: false,
    skip_approval: false,
    ...overrides,
  };
}

test('PO present in multiple inboxes appears once', () => {
  const shared = poFixture({
    current_approval_level: '1',
    approvers: [{ level: '1', user: { email: 'approver@example.com' } }],
  });
  const model = buildRdaDashboardModel({
    myRows: [],
    currentEmail: 'approver@example.com',
    permissions: permissionsFixture({ is_approver: true, is_afc: true }),
    inboxes: [
      { kind: 'level1-2', rows: [shared] },
      { kind: 'payment-method', rows: [shared] },
    ],
  });

  assertEqual(model.rows.length, 1, 'deduplicated row count');
  assertEqual(model.counts.toManage, 1, 'to-manage count should count the PO once');
  assertEqual(model.rows[0]?.contexts.length, 2, 'row should keep both queue contexts');
  assertEqual(model.rows[0]?.primaryQueue.key, 'level1-2', 'primary queue should follow operational order');
});

test('can_see_all_po keeps visible POs out of the to-do view', () => {
  const visible = poFixture({
    id: 14,
    requester: { email: 'other@example.com' },
    approvers: [{ level: '1', user: { email: 'other-approver@example.com' } }],
    current_approval_level: '1',
  });
  const model = buildRdaDashboardModel({
    myRows: [visible],
    currentEmail: 'supervisor@example.com',
    permissions: permissionsFixture({ can_see_all_po: true }),
    inboxes: [],
  });

  assertEqual(filterRdaDashboardRows(model.rows, { view: 'todo' }).length, 0, 'to-do excludes visibility-only rows');
  assertEqual(filterRdaDashboardRows(model.rows, { view: 'all' }).length, 1, 'all view includes visible rows');
  assertEqual(model.rows[0]?.primaryQueue.key, 'supervision', 'visibility queue');
  assertEqual(model.rows[0]?.nextStepLabel, 'In approvazione', 'visibility next step');
  assertEqual(model.counts.toManage, 0, 'visibility is not counted as work');
});

test('assigned pending approval enters the to-do view', () => {
  const assigned = poFixture({
    id: 15,
    current_approval_level: '2',
    approvers: [{ level: '2', user: { email: 'me@example.com' } }],
  });
  const model = buildRdaDashboardModel({
    myRows: [],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({ is_approver: true }),
    inboxes: [{ kind: 'level1-2', rows: [assigned] }],
  });

  const todo = filterRdaDashboardRows(model.rows, { view: 'todo' });

  assertEqual(todo.length, 1, 'to-do includes assigned approval');
  assertEqual(todo[0]?.primaryQueue.key, 'level1-2', 'assigned approval queue');
  assertEqual(todo[0]?.nextStepLabel, 'Valuta approvazione', 'assigned approval next step');
});

test('unassigned pending approval stays only in all', () => {
  const unassigned = poFixture({
    id: 16,
    current_approval_level: '1',
    approvers: [{ level: '1', user: { email: 'other@example.com' } }],
  });
  const model = buildRdaDashboardModel({
    myRows: [],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({ is_approver: true }),
    inboxes: [{ kind: 'level1-2', rows: [unassigned] }],
  });

  assertEqual(filterRdaDashboardRows(model.rows, { view: 'todo' }).length, 0, 'to-do excludes unassigned approval');
  assertEqual(filterRdaDashboardRows(model.rows, { view: 'all' }).length, 1, 'all view includes unassigned approval');
  assertEqual(model.rows[0]?.primaryQueue.key, 'visible', 'visible queue');
});

test('pending approval without approvers is not actionable', () => {
  const unresolved = poFixture({ id: 17, approvers: undefined, current_approval_level: '1' });
  const model = buildRdaDashboardModel({
    myRows: [],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({ is_approver: true }),
    inboxes: [{ kind: 'level1-2', rows: [unresolved] }],
  });

  assertEqual(filterRdaDashboardRows(model.rows, { view: 'todo' }).length, 0, 'to-do excludes rows without approvers');
  assertEqual(model.rows[0]?.isActionable, false, 'row remains read-only until detail confirms action');
});

test('skip_approval does not create dashboard actions', () => {
  const visible = poFixture({
    id: 18,
    requester: { email: 'other@example.com' },
    current_approval_level: '1',
    approvers: [{ level: '1', user: { email: 'me@example.com' } }],
  });
  const model = buildRdaDashboardModel({
    myRows: [visible],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({ skip_approval: true, can_see_all_po: true }),
    inboxes: [],
  });

  assertEqual(filterRdaDashboardRows(model.rows, { view: 'todo' }).length, 0, 'skip approval is not an action grant');
  assertEqual(model.counts.toManage, 0, 'skip approval does not affect action count');
});

test('own draft enters the to-do view', () => {
  const model = buildRdaDashboardModel({
    myRows: [poFixture({ id: 11, state: 'DRAFT', requester: { email: 'me@example.com' } })],
    currentEmail: 'me@example.com',
    inboxes: [],
  });

  const todo = filterRdaDashboardRows(model.rows, { view: 'todo' });

  assertEqual(model.counts.ownDrafts, 1, 'own draft count');
  assertEqual(todo.length, 1, 'to-do should include own draft');
  assertEqual(todo[0]?.primaryQueue.key, 'own-draft', 'draft queue context');
});

test('own and accessible requests land in the correct views', () => {
  const draft = poFixture({ id: 11, code: 'PO-11', state: 'DRAFT', requester: { email: 'me@example.com' } });
  const mine = poFixture({ id: 12, code: 'PO-12', state: 'PENDING_SEND', requester: { email: 'me@example.com' } });
  const inbox = poFixture({ id: 13, code: 'PO-13', requester: { email: 'other@example.com' } });

  const model = buildRdaDashboardModel({
    myRows: [draft, mine],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({
      is_approver: true,
    }),
    inboxes: [{ kind: 'level1-2', rows: [inbox] }],
  });

  assertEqual(filterRdaDashboardRows(model.rows, { view: 'todo' }).length, 1, 'to-do view');
  assertEqual(filterRdaDashboardRows(model.rows, { view: 'mine' }).length, 2, 'my view');
  assertEqual(filterRdaDashboardRows(model.rows, { view: 'all' }).length, 3, 'all view');
  assertEqual(model.counts.ownOpen, 1, 'own open count excludes drafts');
});

test('rows returned by the PO list are not treated as mine when requester differs', () => {
  const otherRequester = poFixture({ id: 31, code: 'PO-31', requester: { email: 'other@example.com' } });
  const model = buildRdaDashboardModel({
    myRows: [otherRequester],
    currentEmail: 'me@example.com',
    permissions: permissionsFixture({ is_approver: true }),
    inboxes: [{ kind: 'level1-2', rows: [otherRequester] }],
  });

  assertEqual(model.counts.ownDrafts, 0, 'foreign draft count');
  assertEqual(model.counts.ownOpen, 0, 'foreign open count');
  assertEqual(filterRdaDashboardRows(model.rows, { view: 'mine' }).length, 0, 'mine view should exclude foreign requester');
  assertEqual(model.rows[0]?.contexts.some((context) => context.key === 'requester'), false, 'foreign row should not get requester context');
});

test('filters do not change base counts', () => {
  const mine = poFixture({ id: 21, code: 'PO-21', provider: { id: 21, company_name: 'Alpha' } });
  const inbox = poFixture({
    id: 22,
    code: 'PO-22',
    state: 'PENDING_BUDGET_INCREMENT',
    provider: { id: 22, company_name: 'Budget Partner' },
  });
  const model = buildRdaDashboardModel({
    myRows: [mine],
    currentEmail: 'owner@example.com',
    permissions: permissionsFixture({ is_approver_extra_budget: true }),
    inboxes: [{ kind: 'budget-increment', rows: [inbox] }],
  });

  const filtered = filterRdaDashboardRows(model.rows, {
    view: 'all',
    q: 'budget',
    state: 'PENDING_BUDGET_INCREMENT',
    queue: 'budget-increment',
  });

  assertEqual(filtered.length, 1, 'filtered row count');
  assertEqual(filtered[0]?.id, 22, 'filtered row identity');
  assertEqual(model.counts.totalAccessible, 2, 'base total remains unchanged');
  assertEqual(model.counts.toManage, 1, 'base operational count remains unchanged');
});

console.log('rda dashboard tests passed');
