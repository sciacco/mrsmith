// Factual copy + formatting helpers for the deposited-filing surface (issue #78, Fase 9).
// Every label is a fact about the row/proposal, composed from the structured backend enums —
// never a component-internal name, never a price. The block is exception-only: normality
// (a ready filing, an effective proposal at rest) is not announced.

import { ApiError } from '@mrsmith/api-client';
import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { MAFilingRow, MANIProposalView } from '../../api/types';
import { formatDeepCompactEuro } from '../deep/DeepComponents';

// Factual, action-oriented messages for the filing-surface error codes (snake_case from the
// backend). Falls back to a plain sentence for anything unmapped. Persistent inline surfaces
// use these — never rely on the toast alone for an error the analyst must act on (§14.3).
export function filingErrorMessage(error: unknown, fallback = 'Operazione non riuscita.'): string {
  if (error instanceof ApiError) {
    const body = error.body;
    const code = body && typeof body === 'object' && 'error' in body && typeof (body as { error: unknown }).error === 'string'
      ? (body as { error: string }).error
      : undefined;
    switch (code) {
      case 'docuengine_not_configured':
        return 'Canale camerale non configurato in questo ambiente.';
      case 'filing_identity_unresolved':
        return 'Identità fiscale non risolta: manca una partita IVA o un codice fiscale valido.';
      case 'invalid_pdf':
        return 'Il file non è un PDF valido.';
      case 'empty_file':
        return 'Il file è vuoto.';
      case 'file_too_large':
        return 'Il file supera i 30 MB.';
      case 'file_required':
        return 'Seleziona un file da caricare.';
      case 'identity_override_not_applicable':
        return 'Conferma non applicabile: l’identità non è più in attesa di validazione.';
      case 'reason_required':
        return 'Il motivo è obbligatorio.';
      case 'deep_absent':
        return 'Nessuna analisi approfondita per questa azienda.';
      case 'deep_not_ready':
        return 'L’analisi approfondita non è ancora pronta.';
      case 'brief_not_regenerable':
        return 'Brief non rigenerabile: dati di base non ricostruibili.';
      case 'ratified_amount_required':
        return 'Serve un importo firmato per ratificare.';
      case 'ratified_treatment_required':
        return 'Scegli il trattamento (EBITDA o PFN) per ratificare.';
      case 'invalid_ratified_treatment':
        return 'Trattamento non valido.';
      default:
        break;
    }
    if (error.status === 401) return 'Sessione non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
  }
  if (error instanceof Error && error.message) return fallback;
  return fallback;
}

const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

// formatSignedCompactEuro renders a delta with an explicit sign (proper minus glyph), compact
// scale. 0 stays "0 €" with no sign (no variation is itself a fact, not a decorated zero).
export function formatSignedCompactEuro(value: number): string {
  if (value === 0) return formatDeepCompactEuro(0);
  const sign = value > 0 ? '+' : '−';
  return `${sign}${formatDeepCompactEuro(Math.abs(value))}`;
}

export function formatEuro(value: number): string {
  return moneyFormat.format(value);
}

// Exercise label: the closing date is a bare YYYY-MM-DD; the exercise year is the analyst's
// mental key, so we surface the year with the full date as the precise fact.
export function exerciseLabel(closingDate?: string): string {
  if (!closingDate) return 'Esercizio non determinato';
  const year = closingDate.slice(0, 4);
  return `Esercizio ${year}`;
}

// The exercise year alone (for in-sentence facts). "—" when no closing date.
export function exerciseYear(closingDate?: string): string {
  if (!closingDate) return '—';
  return closingDate.slice(0, 4);
}

export function filingTypeLabel(type?: string): string {
  return type?.trim() ? type : 'Tipo non indicato';
}

export function filingOriginLabel(origin: string): string {
  switch (origin) {
    case 'upload':
      return 'Caricato';
    case 'docuengine':
      return 'Ricerca camerale';
    default:
      return origin;
  }
}

// A filing is still being worked (the pipeline will advance it) → the block keeps polling.
export function isFilingInProgress(status: string): boolean {
  return status === 'queued' || status === 'ocr' || status === 'parse' || status === 'ni_reading';
}

// Factual phase label while the pipeline is running. null once terminal.
export function filingProcessingLabel(status: string): string | null {
  switch (status) {
    case 'queued':
      return 'In coda';
    case 'ocr':
      return 'Lettura ottica in corso';
    case 'parse':
      return 'Estrazione bilancio in corso';
    case 'ni_reading':
      return 'Lettura nota integrativa in corso';
    default:
      return null;
  }
}

// The exception badge shown on a row — ONLY for failed / identity_blocked / degraded. A ready
// filing gets no badge (normality is not announced). Label composed from the row's own facts.
export function filingExceptionBadge(row: MAFilingRow): { variant: StatusBadgeVariant; label: string } | null {
  switch (row.status) {
    case 'failed':
      return { variant: 'danger', label: 'Elaborazione non riuscita' };
    case 'degraded':
      return { variant: 'warning', label: 'Senza analisi baseline' };
    case 'identity_blocked':
      return { variant: 'warning', label: 'Identità bloccata' };
    default:
      return null;
  }
}

// Full factual sentence for an exceptional row (rendered beneath the row).
export function filingExceptionFact(row: MAFilingRow): string | null {
  if (row.status === 'failed') {
    return row.error ? `Elaborazione non riuscita: ${row.error}` : 'Elaborazione non riuscita.';
  }
  if (row.status === 'degraded') {
    return 'Bilancio acquisito, ma senza un’analisi baseline con cui confrontarlo.';
  }
  if (row.status === 'identity_blocked') {
    if (row.identityStatus === 'mismatch') {
      return 'Il documento appartiene a un’altra identità fiscale — bloccato.';
    }
    if (row.identityStatus === 'pending_validation') {
      return 'Identità di pagina 1 non leggibile.';
    }
    return 'Identità fiscale del documento non verificata.';
  }
  return null;
}

// The identity-override action is offered ONLY for the one overridable case: an identity_blocked
// filing whose identity_status is pending_validation (an unreadable page 1, not a hard mismatch).
export function filingIsIdentityOverridable(row: MAFilingRow): boolean {
  return row.status === 'identity_blocked' && row.identityStatus === 'pending_validation';
}

export function treatmentLabel(treatment: string): string {
  switch (treatment) {
    case 'ebitda':
      return 'EBITDA';
    case 'pfn':
      return 'PFN';
    case 'dd_only':
      return 'Punto DD';
    default:
      return treatment;
  }
}

export function directionLabel(direction: string): string {
  switch (direction) {
    case 'increase':
      return 'In aumento';
    case 'decrease':
      return 'In diminuzione';
    case 'uncertain':
      return 'Verso incerto';
    default:
      return direction;
  }
}

export function incertezzaLabel(incertezza?: string): string | null {
  switch (incertezza) {
    case 'low':
      return 'Incertezza bassa';
    case 'medium':
      return 'Incertezza media';
    case 'high':
      return 'Incertezza alta';
    default:
      return null;
  }
}

// Sober state badge for a proposal: effective is the only "positive" (success); everything else
// is neutral so the surface stays calm. The buttons carry the call to action, not the badge.
export function proposalStateBadge(state: string): { variant: StatusBadgeVariant; label: string } {
  switch (state) {
    case 'effective':
      return { variant: 'success', label: 'Efficace' };
    case 'rejected':
      return { variant: 'neutral', label: 'Scartata' };
    case 'revoked':
      return { variant: 'neutral', label: 'Revocata' };
    case 'pending':
    default:
      return { variant: 'neutral', label: 'In attesa' };
  }
}

// A dd_only proposal with no promotion is a DD point (no valuation effect); a promotion to
// EBITDA/PFN requires both a signed amount and a target treatment.
export function proposalNeedsTreatmentChoice(p: MANIProposalView): boolean {
  return p.trattamentoCandidato === 'dd_only' || p.direction === 'uncertain';
}

export function pageCiteLabel(pageNo?: number): string | null {
  if (pageNo == null) return null;
  return `p. ${pageNo}`;
}
