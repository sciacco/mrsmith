import type { PaymentMethod, ProviderSummary } from '../api/types';

export type PaymentOptionBadge = 'provider-default' | 'cdlan-default';

export interface PaymentMethodOption extends PaymentMethod {
  label: string;
  badges: PaymentOptionBadge[];
  isProviderDefault: boolean;
  isCdlanDefault: boolean;
  isNotPreapproved: boolean;
}

export interface BuildPaymentMethodOptionsInput {
  methods: PaymentMethod[];
  providerDefault?: PaymentMethod | string | number | null;
  cdlanDefaultCode?: string | number | null;
  currentCode?: string | number | null;
}

function normalizeCode(value: string | number | null | undefined): string {
  if (value == null) return '';
  return String(value).trim();
}

function normalizePaymentMethod(value: PaymentMethod | string | number | null | undefined): PaymentMethod | null {
  if (value == null) return null;
  if (typeof value === 'object') {
    const code = normalizeCode(value.code);
    if (!code) return null;
    return {
      ...value,
      code,
      description: value.description?.trim() || code,
    };
  }
  const code = normalizeCode(value);
  return code ? { code, description: code } : null;
}

function mergeMethod(existing: PaymentMethod | undefined, next: PaymentMethod): PaymentMethod {
  if (!existing) return next;
  return {
    ...existing,
    ...next,
    code: existing.code,
    description: next.description?.trim() || existing.description?.trim() || existing.code,
    rda_available: Boolean(existing.rda_available || next.rda_available),
  };
}

function methodLabel(method: PaymentMethod): string {
  return method.description?.trim() || method.code;
}

export function paymentMethodFromProvider(provider: ProviderSummary | undefined): PaymentMethod | null {
  return normalizePaymentMethod(provider?.default_payment_method);
}

export function paymentCodeFromProvider(provider: ProviderSummary | undefined): string {
  return paymentMethodFromProvider(provider)?.code ?? '';
}

export function preferredPaymentMethodCode(provider: ProviderSummary | undefined, cdlanDefaultCode: string | number | null | undefined): string {
  return paymentCodeFromProvider(provider) || normalizeCode(cdlanDefaultCode);
}

export function requiresPaymentMethodVerification(
  code: string | number | null | undefined,
  providerDefaultCode: string | number | null | undefined,
  cdlanDefaultCode: string | number | null | undefined,
): boolean {
  const selected = normalizeCode(code);
  if (!selected) return false;
  const providerDefault = normalizeCode(providerDefaultCode);
  const cdlanDefault = normalizeCode(cdlanDefaultCode);
  return selected !== providerDefault && selected !== cdlanDefault;
}

export function buildPaymentMethodOptions({
  methods,
  providerDefault,
  cdlanDefaultCode,
  currentCode,
}: BuildPaymentMethodOptionsInput): PaymentMethodOption[] {
  const catalogByCode = new Map<string, PaymentMethod>();

  for (const rawMethod of methods) {
    const method = normalizePaymentMethod(rawMethod);
    if (!method) continue;
    catalogByCode.set(method.code, mergeMethod(catalogByCode.get(method.code), method));
  }

  const providerMethod = normalizePaymentMethod(providerDefault);
  const providerDefaultCode = providerMethod?.code ?? '';
  const cdlanDefault = normalizeCode(cdlanDefaultCode);
  const current = normalizeCode(currentCode);
  const orderedCodes: string[] = [];
  const fallbackByCode = new Map<string, PaymentMethod>();

  function pushCode(code: string, fallback?: PaymentMethod | null) {
    if (!code) return;
    if (!orderedCodes.includes(code)) orderedCodes.push(code);
    if (fallback) fallbackByCode.set(code, mergeMethod(fallbackByCode.get(code), fallback));
  }

  pushCode(providerDefaultCode, providerMethod);
  pushCode(cdlanDefault, catalogByCode.get(cdlanDefault) ?? (cdlanDefault ? { code: cdlanDefault, description: cdlanDefault } : null));

  const catalogRest = Array.from(catalogByCode.values())
    .filter((method) => method.rda_available === true)
    .filter((method) => method.code !== providerDefaultCode && method.code !== cdlanDefault)
    .sort((a, b) => methodLabel(a).localeCompare(methodLabel(b), 'it'));

  for (const method of catalogRest) pushCode(method.code, method);
  pushCode(current, catalogByCode.get(current) ?? (current ? { code: current, description: current } : null));

  return orderedCodes.map((code) => {
    const method = mergeMethod(fallbackByCode.get(code), catalogByCode.get(code) ?? { code, description: code });
    const isProviderDefault = code === providerDefaultCode;
    const isCdlanDefault = code === cdlanDefault;
    const badges: PaymentOptionBadge[] = [];
    if (isProviderDefault) badges.push('provider-default');
    if (isCdlanDefault) badges.push('cdlan-default');
    const isNotPreapproved = isProviderDefault && method.rda_available !== true;
    return {
      ...method,
      label: methodLabel(method),
      badges,
      isProviderDefault,
      isCdlanDefault,
      isNotPreapproved,
    };
  });
}
