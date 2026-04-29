import type { BudgetForUser, CreatePOPayload, PatchPOPayload, PaymentMethod, ProviderSummary } from '../api/types';

export type POType = 'STANDARD' | 'ECOMMERCE';

export interface POHeaderDraft {
  type?: POType | string;
  budget_id: number | '';
  object: string;
  project: string;
  provider_id: number | '';
  payment_method: string;
  provider_offer_code: string;
  provider_offer_date: string;
  description: string;
  note: string;
}

export function paymentCodeFromProvider(provider: ProviderSummary | undefined): string {
  const value = provider?.default_payment_method;
  if (!value) return '';
  if (typeof value === 'object') return value.code;
  return String(value);
}

export function methodUnion(methods: PaymentMethod[], ...extraCodes: string[]): PaymentMethod[] {
  const byCode = new Map<string, PaymentMethod>();
  for (const method of methods) byCode.set(method.code, method);
  for (const code of extraCodes) {
    if (code && !byCode.has(code)) byCode.set(code, { code, description: code });
  }
  return Array.from(byCode.values()).sort((a, b) => a.description.localeCompare(b.description));
}

export function selectedBudgetBinding(budgets: BudgetForUser[], budgetId: number | '') {
  const budget = budgets.find((item) => (item.budget_id ?? item.id ?? 0) === budgetId);
  if (!budget) return {};
  if (budget.cost_center) return { cost_center: budget.cost_center, budget_user_id: null };
  return { budget_user_id: budget.budget_user_id ?? budget.user_id ?? null, cost_center: null };
}

export function buildCreatePOPayload(header: POHeaderDraft, budgets: BudgetForUser[]): CreatePOPayload {
  const binding = selectedBudgetBinding(budgets, header.budget_id);
  return {
    type: header.type === 'ECOMMERCE' ? 'ECOMMERCE' : 'STANDARD',
    budget_id: Number(header.budget_id),
    provider_id: Number(header.provider_id),
    payment_method: header.payment_method,
    project: header.project.trim(),
    object: header.object.trim(),
    description: header.description.trim() || undefined,
    note: header.note.trim() || undefined,
    provider_offer_code: header.provider_offer_code.trim() || undefined,
    provider_offer_date: header.provider_offer_date || undefined,
    ...(binding.cost_center ? { cost_center: binding.cost_center } : {}),
    ...(binding.budget_user_id ? { budget_user_id: binding.budget_user_id } : {}),
  };
}

export function buildPatchPOPayload(
  header: POHeaderDraft,
  budgets: BudgetForUser[],
  providerChanged: boolean,
  recipientIds?: number[],
): PatchPOPayload {
  return {
    ...(header.type ? { type: header.type } : {}),
    budget_id: Number(header.budget_id),
    ...selectedBudgetBinding(budgets, header.budget_id),
    object: header.object.trim(),
    project: header.project.trim(),
    provider_id: Number(header.provider_id),
    payment_method: header.payment_method,
    provider_offer_code: header.provider_offer_code.trim() || null,
    provider_offer_date: header.provider_offer_date || null,
    description: header.description.trim() || null,
    note: header.note.trim() || null,
    reference_warehouse: 'MILANO',
    ...(providerChanged ? { recipient_ids: [] } : {}),
    ...(recipientIds ? { recipient_ids: recipientIds } : {}),
  };
}
