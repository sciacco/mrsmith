import type { PoPreview, RdaPermissions } from '../api/types';
import { inboxConfig, inboxOrder, type InboxKind } from './inbox.js';
import { isApprover, parseMistraMoney } from './format.js';
import { stateLabel } from './state-labels.js';

export type RdaDashboardView = 'todo' | 'mine' | 'all';
export type RdaQueueKey = InboxKind | 'own-draft' | 'requester' | 'supervision' | 'visible';
export type RdaContextType = 'inbox' | 'draft' | 'requester' | 'visibility';
export type RdaDashboardSortKey = 'request' | 'state' | 'provider' | 'requester' | 'created' | 'total';
export type RdaDashboardSortDirection = 'asc' | 'desc';

export interface RdaDashboardSort {
  key: RdaDashboardSortKey;
  direction: RdaDashboardSortDirection;
}

export interface RdaDashboardContext {
  key: RdaQueueKey;
  label: string;
  type: RdaContextType;
}

export interface RdaInboxSource {
  kind: InboxKind;
  rows: PoPreview[];
}

export interface RdaDashboardRow extends PoPreview {
  contexts: RdaDashboardContext[];
  primaryQueue: RdaDashboardContext;
  actionLabel: string;
  isRequesterOwned: boolean;
  isOwnDraft: boolean;
  isActionable: boolean;
}

export interface RdaDashboardCounts {
  toManage: number;
  ownDrafts: number;
  ownOpen: number;
  totalAccessible: number;
}

export interface RdaDashboardModel {
  rows: RdaDashboardRow[];
  counts: RdaDashboardCounts;
}

export interface RdaDashboardFilters {
  view: RdaDashboardView;
  q?: string;
  state?: string;
  queue?: string;
}

export interface RdaFilterOption {
  value: string;
  label: string;
  count: number;
}

interface MutableRow {
  po: PoPreview;
  contexts: Map<RdaQueueKey, RdaDashboardContext>;
  isRequesterOwned: boolean;
}

const terminalStates = new Set(['APPROVED', 'CLOSED', 'REJECTED', 'SENT', 'DELIVERED_AND_COMPLIANT']);
const sortKeys = new Set<RdaDashboardSortKey>(['request', 'state', 'provider', 'requester', 'created', 'total']);
const textCollator = new Intl.Collator('it', { numeric: true, sensitivity: 'base' });
const defaultSortDirections: Record<RdaDashboardSortKey, RdaDashboardSortDirection> = {
  request: 'asc',
  state: 'asc',
  provider: 'asc',
  requester: 'asc',
  created: 'desc',
  total: 'desc',
};

export function parseRdaDashboardView(value: string | null): RdaDashboardView {
  if (value === 'mine' || value === 'all' || value === 'todo') return value;
  return 'todo';
}

export function parseRdaDashboardSortKey(value: string | null): RdaDashboardSortKey | null {
  return sortKeys.has(value as RdaDashboardSortKey) ? (value as RdaDashboardSortKey) : null;
}

export function parseRdaDashboardSortDirection(value: string | null): RdaDashboardSortDirection | null {
  if (value === 'asc' || value === 'desc') return value;
  return null;
}

export function defaultRdaDashboardSortDirection(key: RdaDashboardSortKey): RdaDashboardSortDirection {
  return defaultSortDirections[key];
}

export function isTerminalPOState(state?: string | null): boolean {
  return Boolean(state && terminalStates.has(state));
}

function normalize(value: string | number | null | undefined): string {
  return String(value ?? '')
    .toLocaleLowerCase('it')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '');
}

function requesterName(po: PoPreview): string {
  const user = po.requester;
  return [user?.name, user?.first_name, user?.last_name, user?.email].filter(Boolean).join(' ');
}

export function rdaDashboardRequesterLabel(po: Pick<PoPreview, 'requester'>): string {
  const requester = po.requester;
  const name = [requester?.first_name, requester?.last_name].filter(Boolean).join(' ');
  if (requester?.name) return requester.name;
  if (name) return name;
  return requester?.email ?? '-';
}

export function rdaDashboardRequestTitle(po: Pick<PoPreview, 'object' | 'project'>): string {
  return po.object || po.project || 'Richiesta senza oggetto';
}

function requesterMatches(po: PoPreview, currentEmail?: string | null): boolean {
  const requesterEmail = po.requester?.email?.trim();
  const email = currentEmail?.trim();
  return Boolean(
    email &&
      requesterEmail &&
      requesterEmail.toLocaleLowerCase('it') === email.toLocaleLowerCase('it'),
  );
}

function addContext(row: MutableRow, context: RdaDashboardContext) {
  row.contexts.set(context.key, context);
}

function ownDraftContext(): RdaDashboardContext {
  return { key: 'own-draft', label: 'Bozze proprie', type: 'draft' };
}

function requesterContext(): RdaDashboardContext {
  return { key: 'requester', label: 'Richiedente', type: 'requester' };
}

function visibilityContext(permissions?: RdaPermissions): RdaDashboardContext {
  if (permissions?.can_see_all_po) return { key: 'supervision', label: 'Supervisione', type: 'visibility' };
  return { key: 'visible', label: 'Visibile', type: 'visibility' };
}

function inboxContext(kind: InboxKind): RdaDashboardContext {
  return { key: kind, label: inboxConfig[kind].shortTitle, type: 'inbox' };
}

function queuePriority(context: RdaDashboardContext): number {
  if (context.type === 'inbox') return inboxOrder.indexOf(context.key as InboxKind);
  if (context.key === 'own-draft') return 20;
  if (context.key === 'supervision') return 25;
  if (context.key === 'visible') return 26;
  return 30;
}

function sortContexts(contexts: Iterable<RdaDashboardContext>): RdaDashboardContext[] {
  return Array.from(contexts).sort((a, b) => queuePriority(a) - queuePriority(b));
}

function isVisibilityQueueKey(value: string): boolean {
  return value === 'supervision' || value === 'visible';
}

function stateActionLabel(state?: string): string {
  switch (state) {
    case 'PENDING_SEND':
      return 'Invia al fornitore';
    case 'PENDING_VERIFICATION':
      return 'Verifica fornitura';
    default:
      return '';
  }
}

function isRequesterActionState(state?: string | null): boolean {
  return stateActionLabel(state ?? undefined) !== '';
}

function hasInboxContext(contexts: RdaDashboardContext[]): boolean {
  return contexts.some((context) => context.type === 'inbox');
}

function isAssignedDashboardWork({
  isOwnDraft,
  isRequesterOwned,
  state,
  contexts,
}: {
  isOwnDraft: boolean;
  isRequesterOwned: boolean;
  state?: string | null;
  contexts: RdaDashboardContext[];
}): boolean {
  return isOwnDraft || hasInboxContext(contexts) || (isRequesterOwned && isRequesterActionState(state));
}

function actionLabel(row: PoPreview, primaryQueue: RdaDashboardContext, isAssignedWork: boolean): string {
  if (isTerminalPOState(row.state)) {
    return '';
  }

  switch (primaryQueue.key) {
    case 'own-draft':
      return 'Completa bozza';
    case 'level1-2':
      return 'Valuta approvazione';
    case 'leasing':
      return 'Valuta leasing';
    case 'no-leasing':
      return 'Valuta no leasing';
    case 'payment-method':
      return 'Conferma metodo';
    case 'budget-increment':
      return 'Valuta budget';
    default:
      return isAssignedWork ? stateActionLabel(row.state) : '';
  }
}

function mergePreview(current: PoPreview, next: PoPreview): PoPreview {
  return {
    ...current,
    ...next,
    requester: next.requester ?? current.requester,
    provider: next.provider ?? current.provider,
    payment_method: next.payment_method ?? current.payment_method,
    approvers: next.approvers ?? current.approvers,
  };
}

function rowDateValue(row: PoPreview): number {
  const raw = row.created ?? row.creation_date ?? row.updated;
  if (!raw) return 0;
  const parsed = new Date(raw).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

function sortableRowDateValue(row: PoPreview): number | null {
  const raw = row.created ?? row.creation_date ?? row.updated;
  if (!raw) return null;
  const parsed = new Date(raw).getTime();
  return Number.isNaN(parsed) ? null : parsed;
}

function textValue(value: string | number | null | undefined): string | null {
  const normalized = String(value ?? '').trim();
  return normalized && normalized !== '-' ? normalized : null;
}

function moneyValue(value: string | number | null | undefined): number | null {
  if (value === null || value === undefined || String(value).trim() === '') return null;
  return parseMistraMoney(value);
}

function compareTextValues(a: string | number | null | undefined, b: string | number | null | undefined, direction: RdaDashboardSortDirection): number {
  const av = textValue(a);
  const bv = textValue(b);
  if (av === null && bv === null) return 0;
  if (av === null) return 1;
  if (bv === null) return -1;
  const result = textCollator.compare(av, bv);
  return direction === 'asc' ? result : -result;
}

function compareNumberValues(a: number | null, b: number | null, direction: RdaDashboardSortDirection): number {
  if (a === null && b === null) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  const result = a - b;
  return direction === 'asc' ? result : -result;
}

function rowRequestSortValue(row: RdaDashboardRow): string {
  return [row.code ?? `PO ${row.id}`, rdaDashboardRequestTitle(row), row.project].filter(Boolean).join(' ');
}

function compareDashboardRows(a: RdaDashboardRow, b: RdaDashboardRow, sort: RdaDashboardSort): number {
  switch (sort.key) {
    case 'request':
      return compareTextValues(rowRequestSortValue(a), rowRequestSortValue(b), sort.direction);
    case 'state':
      return compareTextValues(stateLabel(a.state), stateLabel(b.state), sort.direction);
    case 'provider':
      return compareTextValues(a.provider?.company_name, b.provider?.company_name, sort.direction);
    case 'requester':
      return compareTextValues(rdaDashboardRequesterLabel(a), rdaDashboardRequesterLabel(b), sort.direction);
    case 'created':
      return compareNumberValues(sortableRowDateValue(a), sortableRowDateValue(b), sort.direction);
    case 'total':
      return compareNumberValues(moneyValue(a.total_price), moneyValue(b.total_price), sort.direction);
    default:
      return 0;
  }
}

export function sortRdaDashboardRows(rows: RdaDashboardRow[], sort: RdaDashboardSort | null): RdaDashboardRow[] {
  if (!sort) return rows;
  return rows
    .map((row, index) => ({ row, index }))
    .sort((a, b) => compareDashboardRows(a.row, b.row, sort) || a.index - b.index)
    .map((item) => item.row);
}

function rowPriority(row: RdaDashboardRow): number {
  if (row.isActionable && !row.isOwnDraft) return 0;
  if (row.isOwnDraft) return 1;
  if (!isTerminalPOState(row.state)) return 2;
  return 3;
}

function toDashboardRow(row: MutableRow): RdaDashboardRow {
  const contexts = sortContexts(row.contexts.values());
  const primaryQueue = contexts.find((context) => context.type === 'inbox') ?? contexts[0] ?? requesterContext();
  const isOwnDraft = row.isRequesterOwned && row.po.state === 'DRAFT';
  const isActionable = isAssignedDashboardWork({
    isOwnDraft,
    isRequesterOwned: row.isRequesterOwned,
    state: row.po.state,
    contexts,
  });

  return {
    ...row.po,
    contexts,
    primaryQueue,
    actionLabel: actionLabel(row.po, primaryQueue, isActionable),
    isRequesterOwned: row.isRequesterOwned,
    isOwnDraft,
    isActionable,
  };
}

function isActionableInboxPO(kind: InboxKind, po: PoPreview, currentEmail: string | null | undefined, permissions?: RdaPermissions): boolean {
  switch (kind) {
    case 'level1-2':
      return Boolean(permissions?.is_approver && isApprover(po, currentEmail));
    case 'leasing':
    case 'payment-method':
      return Boolean(permissions?.is_afc);
    case 'no-leasing':
      return Boolean(permissions?.is_approver_no_leasing);
    case 'budget-increment':
      return Boolean(permissions?.is_approver_extra_budget);
    default:
      return false;
  }
}

export function buildRdaDashboardModel({
  myRows,
  inboxes,
  currentEmail,
  permissions,
}: {
  myRows: PoPreview[];
  inboxes: RdaInboxSource[];
  currentEmail?: string | null;
  permissions?: RdaPermissions;
}): RdaDashboardModel {
  const byId = new Map<number, MutableRow>();

  function ensureRow(po: PoPreview): MutableRow {
    const existing = byId.get(po.id);
    if (existing) {
      existing.po = mergePreview(existing.po, po);
      return existing;
    }
    const next: MutableRow = {
      po,
      contexts: new Map(),
      isRequesterOwned: requesterMatches(po, currentEmail),
    };
    byId.set(po.id, next);
    return next;
  }

  for (const po of myRows) {
    const row = ensureRow(po);
    const ownedByRequester = requesterMatches(po, currentEmail);
    row.isRequesterOwned = row.isRequesterOwned || ownedByRequester;
    if (ownedByRequester) {
      addContext(row, po.state === 'DRAFT' ? ownDraftContext() : requesterContext());
    } else {
      addContext(row, visibilityContext(permissions));
    }
  }

  for (const inbox of inboxes) {
    for (const po of inbox.rows) {
      const row = ensureRow(po);
      const ownedByRequester = requesterMatches(po, currentEmail);
      row.isRequesterOwned = row.isRequesterOwned || ownedByRequester;
      if (ownedByRequester && po.state !== 'DRAFT') {
        addContext(row, requesterContext());
      }
      if (isActionableInboxPO(inbox.kind, row.po, currentEmail, permissions)) {
        addContext(row, inboxContext(inbox.kind));
      } else {
        addContext(row, visibilityContext(permissions));
      }
    }
  }

  const rows = Array.from(byId.values())
    .map(toDashboardRow)
    .sort((a, b) => rowPriority(a) - rowPriority(b) || rowDateValue(b) - rowDateValue(a) || b.id - a.id);

  const counts: RdaDashboardCounts = {
    toManage: rows.filter((row) => row.isActionable).length,
    ownDrafts: rows.filter((row) => row.isOwnDraft).length,
    ownOpen: rows.filter((row) => row.isRequesterOwned && !row.isOwnDraft && !isTerminalPOState(row.state)).length,
    totalAccessible: rows.length,
  };

  return { rows, counts };
}

function rowInView(row: RdaDashboardRow, view: RdaDashboardView): boolean {
  if (view === 'all') return true;
  if (view === 'mine') return row.isRequesterOwned;
  return row.isActionable;
}

function matchesQuery(row: RdaDashboardRow, query: string): boolean {
  const terms = normalize(query).trim().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return true;

  const haystack = normalize([
    row.id,
    row.code,
    row.object,
    row.project,
    row.provider?.company_name,
    requesterName(row),
    row.actionLabel,
    row.contexts
      .filter((context) => context.type !== 'visibility')
      .map((context) => context.label)
      .join(' '),
    stateLabel(row.state),
  ].join(' '));

  return terms.every((term) => haystack.includes(term));
}

export function filterRdaDashboardRows(rows: RdaDashboardRow[], filters: RdaDashboardFilters): RdaDashboardRow[] {
  return rows.filter((row) => {
    if (!rowInView(row, filters.view)) return false;
    if (filters.state && row.state !== filters.state) return false;
    if (
      filters.queue &&
      !isVisibilityQueueKey(filters.queue) &&
      !row.contexts.some((context) => context.type !== 'visibility' && context.key === filters.queue)
    ) return false;
    if (filters.q && !matchesQuery(row, filters.q)) return false;
    return true;
  });
}

export function rdaStateFilterOptions(rows: RdaDashboardRow[], view: RdaDashboardView): RdaFilterOption[] {
  const counts = new Map<string, number>();
  for (const row of rows) {
    if (!rowInView(row, view) || !row.state) continue;
    counts.set(row.state, (counts.get(row.state) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .map(([value, count]) => ({ value, label: stateLabel(value), count }))
    .sort((a, b) => a.label.localeCompare(b.label, 'it'));
}

export function rdaQueueFilterOptions(rows: RdaDashboardRow[], view: RdaDashboardView): RdaFilterOption[] {
  const counts = new Map<RdaQueueKey, { label: string; count: number; priority: number }>();
  for (const row of rows) {
    if (!rowInView(row, view)) continue;
    for (const context of row.contexts) {
      if (context.type === 'visibility') continue;
      const current = counts.get(context.key);
      counts.set(context.key, {
        label: context.label,
        count: (current?.count ?? 0) + 1,
        priority: queuePriority(context),
      });
    }
  }

  return Array.from(counts.entries())
    .map(([value, option]) => ({ value, label: option.label, count: option.count, priority: option.priority }))
    .sort((a, b) => a.priority - b.priority)
    .map(({ value, label, count }) => ({ value, label, count }));
}
