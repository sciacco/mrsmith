import type { PoActionModel, PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import { countQuoteAttachments } from './attachments.js';
import { formatMoney, normalizeCurrency, parseMistraMoney } from './format.js';
import { QUALIFICATION_REF } from './provider-refs.js';

export interface POReadinessOptions {
  provider?: ProviderSummary;
  recipients?: ProviderReference[];
  quoteThreshold: number;
}

export interface POReadinessItem {
  id: string;
  label: string;
  detail: string;
  ready: boolean;
}

export interface POHeaderState {
  budget_id: number | '';
  object: string;
  project: string;
  provider_id: number | '';
  payment_method: string;
  currency: string;
  provider_offer_code?: string;
  provider_offer_date?: string;
  description: string;
  note: string;
}

export interface TabBadgeModel {
  attachments: string;
  rows: string;
  notesDirty: boolean;
  contacts: string;
}

export const workflowStages = [
  { id: 'draft', label: 'Bozza' },
  { id: 'approval', label: 'Approvazione' },
  { id: 'method_budget', label: 'Metodo/Leasing/Budget' },
  { id: 'send', label: 'Invio' },
  { id: 'verification', label: 'Verifica' },
  { id: 'closure', label: 'Chiusura' },
] as const;

function providerRefs(provider?: ProviderSummary): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

export function hasQualificationFallback(provider?: ProviderSummary): boolean {
  return providerRefs(provider).some((ref) => ref.reference_type === QUALIFICATION_REF && Boolean(ref.email));
}

export function selectedModeID(actionModel?: PoActionModel | null, currentModeID?: string | null): string {
  const modes = actionModel?.modes ?? [];
  if (currentModeID && modes.some((mode) => mode.id === currentModeID)) return currentModeID;
  return actionModel?.primary_mode_id && modes.some((mode) => mode.id === actionModel.primary_mode_id)
    ? actionModel.primary_mode_id
    : modes[0]?.id ?? 'read_only';
}

export function buildPOReadinessItems(po: PoDetail, header: POHeaderState, options: POReadinessOptions): POReadinessItem[] {
  const total = parseMistraMoney(po.total_price);
  const currency = normalizeCurrency(po.currency ?? header.currency);
  const quoteCount = countQuoteAttachments(po.attachments);
  const quoteRuleReady = total < options.quoteThreshold || quoteCount >= 2;
  const recipients = options.recipients ?? po.recipients ?? [];
  const contactsReady = recipients.length > 0 || hasQualificationFallback(options.provider ?? po.provider);
  const headerReady = Boolean(
    header.budget_id &&
      header.provider_id &&
      header.payment_method.trim() &&
      header.project.trim() &&
      header.object.trim(),
  );

  return [
    {
      id: 'header',
      label: 'Dati richiesta',
      ready: headerReady,
      detail: headerReady ? 'Budget, fornitore, progetto e oggetto sono compilati.' : 'Completa i dati principali della richiesta.',
    },
    {
      id: 'rows',
      label: 'Righe PO',
      ready: (po.rows ?? []).length > 0,
      detail: (po.rows ?? []).length > 0
        ? `${po.rows?.length ?? 0} rig${(po.rows?.length ?? 0) === 1 ? 'a inserita' : 'he inserite'}.`
        : 'Aggiungi almeno una riga PO.',
    },
    {
      id: 'quotes',
      label: 'Preventivi',
      ready: quoteRuleReady,
      detail: total >= options.quoteThreshold
        ? `${quoteCount}/2 preventiv${quoteCount === 1 ? 'o caricato' : 'i caricati'} per ${formatMoney(total, currency)}.`
        : 'La soglia preventivi non richiede altri allegati.',
    },
    {
      id: 'contacts',
      label: 'Destinatari',
      ready: contactsReady,
      detail: contactsReady ? 'Destinatari selezionati o referente qualifica disponibile.' : 'Seleziona un contatto fornitore.',
    },
    {
      id: 'payment',
      label: 'Pagamento',
      ready: header.payment_method.trim() !== '',
      detail: header.payment_method.trim() ? `Metodo selezionato: ${header.payment_method.trim()}.` : 'Seleziona un metodo di pagamento.',
    },
  ];
}

export function buildTabBadges(
  po: PoDetail,
  header: POHeaderState,
  initialHeader: POHeaderState,
  provider?: ProviderSummary,
  quoteThreshold = 3000,
): TabBadgeModel {
  const total = parseMistraMoney(po.total_price);
  const quoteCount = countQuoteAttachments(po.attachments);
  const attachments = total >= quoteThreshold ? `${quoteCount}/2 prev.` : `${po.attachments?.length ?? 0}`;
  const rowCount = po.rows?.length ?? 0;
  const rows = rowCount > 0 ? `${rowCount} - ${formatMoney(po.total_price, po.currency)}` : '0';
  const recipientCount = po.recipients?.length ?? 0;
  const contacts = recipientCount > 0 ? `${recipientCount}` : hasQualificationFallback(provider ?? po.provider) ? 'Qualifica' : '0';

  return {
    attachments,
    rows,
    notesDirty: header.note !== initialHeader.note || header.description !== initialHeader.description,
    contacts,
  };
}
