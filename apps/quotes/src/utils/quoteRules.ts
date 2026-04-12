import type { Quote } from '../api/types';

type IaaSLanguage = 'it' | 'en';

export function getLanguageCode(selection: 'ITA' | 'ENG'): IaaSLanguage {
  return selection === 'ENG' ? 'en' : 'it';
}

export function buildIaaSTrialText(value: number, lang: IaaSLanguage): string {
  if (value <= 0) {
    return '';
  }
  if (lang === 'it') {
    return `La soluzione IAAS Payperuse prevede un trial gratuito per risorse fino a ${value}€ di valore complessivo.`;
  }
  return `Free trial amount is set by Parties up to ${value}€.`;
}

export function parseReplaceOrders(value: string | null | undefined): string[] {
  return (value ?? '')
    .split(/[;,]/)
    .map(item => item.trim())
    .filter(Boolean);
}

export function parseServiceCategoryIds(value: string | null | undefined): string[] {
  return (value ?? '')
    .replace(/[\[\]]/g, '')
    .split(',')
    .map(item => item.trim())
    .filter(Boolean);
}

export function normalizeReplaceOrdersForSave(value: string | null | undefined): string {
  return parseReplaceOrders(value).join(';');
}

export function formatReplaceOrdersForDetail(value: string | null | undefined): string | null {
  const normalized = parseReplaceOrders(value);
  if (normalized.length === 0) {
    return value ?? null;
  }
  return normalized.join(',');
}

export function prepareQuoteForDetail(quote: Quote): Quote {
  return {
    ...quote,
    replace_orders: formatReplaceOrdersForDetail(quote.replace_orders),
  };
}

export function buildQuoteSavePayload(quote: Quote) {
  return {
    id: quote.id,
    quote_number: quote.quote_number,
    customer_id: quote.customer_id,
    deal_number: quote.deal_number,
    owner: quote.owner,
    document_date: quote.document_date,
    document_type: quote.document_type,
    replace_orders: normalizeReplaceOrdersForSave(quote.replace_orders),
    template: quote.template,
    services: quote.services,
    proposal_type: quote.proposal_type,
    initial_term_months: quote.initial_term_months,
    next_term_months: quote.next_term_months,
    bill_months: quote.bill_months,
    delivered_in_days: quote.delivered_in_days,
    date_sent: quote.date_sent,
    status: quote.status,
    notes: quote.notes,
    nrc_charge_time: quote.nrc_charge_time,
    description: quote.description,
    hs_deal_id: quote.hs_deal_id,
    hs_quote_id: quote.hs_quote_id,
    payment_method: quote.payment_method,
    trial: quote.trial,
    rif_ordcli: quote.rif_ordcli,
    rif_tech_nom: quote.rif_tech_nom,
    rif_tech_tel: quote.rif_tech_tel,
    rif_tech_email: quote.rif_tech_email,
    rif_altro_tech_nom: quote.rif_altro_tech_nom,
    rif_altro_tech_tel: quote.rif_altro_tech_tel,
    rif_altro_tech_email: quote.rif_altro_tech_email,
    rif_adm_nom: quote.rif_adm_nom,
    rif_adm_tech_tel: quote.rif_adm_tech_tel,
    rif_adm_tech_email: quote.rif_adm_tech_email,
  };
}

export interface ProductUpdateValues {
  id: number;
  product_name: string;
  nrc: number;
  mrc: number;
  quantity: number;
  extended_description: string | null;
  included: boolean;
}

export function buildProductUpdatePayload(values: ProductUpdateValues, isSpotQuote: boolean) {
  const nextQuantity = values.included && values.quantity <= 0 ? 1 : values.quantity;

  return {
    id: values.id,
    product_name: values.product_name,
    nrc: values.nrc,
    mrc: isSpotQuote ? 0 : values.mrc,
    quantity: nextQuantity,
    extended_description: values.extended_description,
    included: values.included,
  };
}
