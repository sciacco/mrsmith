import type { Quote } from '../api/types';

type IaaSLanguage = 'it' | 'en';

interface IaaSTemplateRule {
  templateId: string;
  kitId: number;
  services: string;
  lang: IaaSLanguage;
}

const IAAS_TEMPLATE_RULES: Record<string, IaaSTemplateRule> = {
  '850825381069': { templateId: '850825381069', kitId: 62, services: '[12]', lang: 'en' },
  '853027287235': { templateId: '853027287235', kitId: 62, services: '[12]', lang: 'it' },
  '853237903587': { templateId: '853237903587', kitId: 116, services: '[14]', lang: 'it' },
  '853320143046': { templateId: '853320143046', kitId: 63, services: '[13]', lang: 'en' },
  '853500178641': { templateId: '853500178641', kitId: 63, services: '[13]', lang: 'it' },
  '853500899556': { templateId: '853500899556', kitId: 116, services: '[14]', lang: 'en' },
  '855439340792': { templateId: '855439340792', kitId: 119, services: '[15]', lang: 'en' },
  '856380863697': { templateId: '856380863697', kitId: 119, services: '[15]', lang: 'it' },
};

export function getIaaSTemplateRule(templateId: string | null | undefined): IaaSTemplateRule | null {
  if (!templateId) {
    return null;
  }
  return IAAS_TEMPLATE_RULES[templateId] ?? null;
}

export function isIaaSTemplate(templateId: string | null | undefined): boolean {
  return getIaaSTemplateRule(templateId) !== null;
}

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
  const iaasRule = getIaaSTemplateRule(quote.template);

  return {
    id: quote.id,
    quote_number: quote.quote_number,
    customer_id: quote.customer_id,
    deal_number: quote.deal_number,
    owner: quote.owner,
    document_date: quote.document_date,
    document_type: iaasRule ? 'TSC-ORDINE-RIC' : quote.document_type,
    replace_orders: normalizeReplaceOrdersForSave(quote.replace_orders),
    template: iaasRule?.templateId ?? quote.template,
    services: iaasRule?.services ?? quote.services,
    proposal_type: quote.proposal_type,
    initial_term_months: iaasRule ? 1 : quote.initial_term_months,
    next_term_months: iaasRule ? 1 : quote.next_term_months,
    bill_months: iaasRule ? 1 : quote.bill_months,
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

export function buildProductUpdatePayload(
  product: {
    id: number;
    product_name: string;
    nrc: number;
    mrc: number;
    quantity: number;
    extended_description: string | null;
  },
  included: boolean,
  quantity: number,
  isSpotQuote: boolean
) {
  const nextQuantity = included && quantity <= 0 ? 1 : quantity;

  return {
    id: product.id,
    product_name: product.product_name,
    nrc: product.nrc,
    mrc: isSpotQuote ? 0 : product.mrc,
    quantity: nextQuantity,
    extended_description: product.extended_description,
    included,
  };
}
