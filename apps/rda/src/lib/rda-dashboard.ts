import type { PoPreview, RdaPermissions } from '../api/types';
import { inboxConfig, inboxOrder, type InboxKind } from './inbox.js';
import { isApprover } from './format.js';
import { stateLabel } from './state-labels.js';

export type RdaDashboardView = 'todo' | 'mine' | 'all';
export type RdaQueueKey = InboxKind | 'own-draft' | 'requester' | 'supervision' | 'visible';
export type RdaContextType = 'inbox' | 'draft' | 'requester' | 'visibility';

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
  nextStepLabel: string;
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

const terminalStates = new Set(['APPROVED', 'CLOSED', 'REJECTED', 'SENT']);

export function parseRdaDashboardView(value: string | null): RdaDashboardView {
  if (value === 'mine' || value === 'all' || value === 'todo') return value;
  return 'todo';
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

function stateNextStep(state?: string): string {
  switch (state) {
    case 'DRAFT':
      return 'Completa bozza';
    case 'PENDING_APPROVAL':
      return 'Attendi approvazione';
    case 'PENDING_APPROVAL_PAYMENT_METHOD':
      return 'Attendi metodo';
    case 'PENDING_LEASING':
      return 'Attendi leasing';
    case 'PENDING_LEASING_ORDER_CREATION':
      return 'Attendi ordine leasing';
    case 'PENDING_APPROVAL_NO_LEASING':
      return 'Attendi no leasing';
    case 'PENDING_BUDGET_INCREMENT':
      return 'Attendi budget';
    case 'PENDING_SEND':
      return 'Attendi invio';
    case 'PENDING_VERIFICATION':
      return 'Attendi conformità';
    case 'APPROVED':
      return 'Approvata';
    case 'REJECTED':
      return 'Richiesta rifiutata';
    case 'SENT':
      return 'Inviata al fornitore';
    case 'CLOSED':
      return 'Chiusa';
    default:
      return 'Apri richiesta';
  }
}

function nextStepLabel(row: PoPreview, primaryQueue: RdaDashboardContext): string {
  if (primaryQueue.type === 'visibility' && row.state === 'PENDING_APPROVAL') {
    return 'In approvazione';
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
    case 'requester':
      return stateNextStep(row.state);
    default:
      return stateNextStep(row.state);
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
  const isActionable = isOwnDraft || contexts.some((context) => context.type === 'inbox');

  return {
    ...row.po,
    contexts,
    primaryQueue,
    nextStepLabel: nextStepLabel(row.po, primaryQueue),
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
    toManage: rows.filter((row) => row.contexts.some((context) => context.type === 'inbox')).length,
    ownDrafts: rows.filter((row) => row.isOwnDraft).length,
    ownOpen: rows.filter((row) => row.isRequesterOwned && !row.isOwnDraft && !isTerminalPOState(row.state)).length,
    totalAccessible: rows.length,
  };

  return { rows, counts };
}

function rowInView(row: RdaDashboardRow, view: RdaDashboardView): boolean {
  if (view === 'all') return true;
  if (view === 'mine') return row.isRequesterOwned;
  return row.isOwnDraft || row.contexts.some((context) => context.type === 'inbox');
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
    row.nextStepLabel,
    row.primaryQueue.label,
    stateLabel(row.state),
  ].join(' '));

  return terms.every((term) => haystack.includes(term));
}

export function filterRdaDashboardRows(rows: RdaDashboardRow[], filters: RdaDashboardFilters): RdaDashboardRow[] {
  return rows.filter((row) => {
    if (!rowInView(row, filters.view)) return false;
    if (filters.state && row.state !== filters.state) return false;
    if (filters.queue && !row.contexts.some((context) => context.key === filters.queue)) return false;
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
