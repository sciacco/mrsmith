// Factual copy + formatting helpers for the deposited-filing surface (issue #78, Fase 9).
// Every label is a fact about the row/proposal, composed from the structured backend enums —
// never a component-internal name, never a price. The block is exception-only: normality
// (a ready filing, an effective proposal at rest) is not announced.

import { ApiError } from '@mrsmith/api-client';
import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { MAFilingAcquisitionView, MAFilingRow, MANIProposalView } from '../../api/types';
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
        return 'Serve un importo con segno (+ o −) diverso da zero per ratificare.';
      case 'ratified_treatment_required':
        return 'Scegli il trattamento (EBITDA o PFN) per ratificare.';
      case 'invalid_ratified_treatment':
        return 'Trattamento non valido.';
      case 'acquisition_not_retryable':
        return 'Acquisizione non ripristinabile: è già completata o in lavorazione.';
      case 'acquisition_not_found':
        return 'Acquisizione non trovata.';
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

// The candidate treatment as a pre-selected value for the ratify form: only ebitda/pfn are valid
// targets; a dd_only/uncertain candidate leaves the choice empty (the analyst must promote it).
export function defaultTreatmentChoice(p: MANIProposalView): string {
  return p.trattamentoCandidato === 'ebitda' || p.trattamentoCandidato === 'pfn' ? p.trattamentoCandidato : '';
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

// ── Proposal state (Fase N2, struttura A) ──
// The backend derives four states from the append-only decision log (ma_filing_endpoints.go:
// classifyMANIProposalState). For the year-group UI a proposal is either "decided" (spun down in
// place, never removed) or still actionable. effective/rejected are decided; pending and revoked
// (a prior decision undone) are back on the analyst's desk.
export function proposalIsDecided(state: string): boolean {
  return state === 'effective' || state === 'rejected';
}

export function proposalIsActionable(state: string): boolean {
  return !proposalIsDecided(state);
}

// Factual one-word state for a row/panel. A revoked proposal is actionable again → "in attesa".
export function proposalStateText(state: string): string {
  switch (state) {
    case 'effective':
      return 'ratificata';
    case 'rejected':
      return 'scartata';
    default:
      return 'in attesa';
  }
}

// The signed treatment amount shown on a decided proposal (the analyst's ratified figure) or, for
// an actionable one, the machine estimate. Signed with the proper minus glyph; null → no figure.
export function proposalDisplayAmount(p: MANIProposalView): number | null {
  if (p.state === 'effective') {
    const ratify = [...p.decisions].reverse().find((d) => d.action === 'ratify');
    if (ratify?.ratifiedAmount != null) return ratify.ratifiedAmount;
  }
  return p.importoRettifica ?? null;
}

const absEuroFormat = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 0 });

// "250.000 €" — magnitude only, it-IT grouping, no decimals (matches the deposited-filing surface).
function absEuro(value: number): string {
  return `${absEuroFormat.format(Math.abs(value))} €`;
}

// Signed full euro for the row amount column: "+18.000 €" / "−250.000 €" (proper minus glyph),
// tabular-nums applied by the cell. "—" when there is no figure.
export function formatSignedEuro(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—';
  if (value === 0) return absEuro(0);
  return `${value > 0 ? '+' : '−'}${absEuro(value)}`;
}

// The effect of a signed ratification, in plain words, computed live while the analyst types. The
// sign carries the direction: on EBITDA a positive amount lifts it; on PFN a negative amount cuts
// the debt (equity up) and a positive one raises it (equity down). dd_only with no target treatment
// (or a zero/empty amount) has no effect → null (the preview stays as its placeholder). Mirrors the
// reducer's sign semantics (reduceMANIEffectiveAdjustments) so the words never contradict the math.
export function effectPhrase(treatment: string, amount: number | null | undefined, year?: string): string | null {
  if (amount == null || !Number.isFinite(amount) || amount === 0) return null;
  const a = absEuro(amount);
  const y = year ? ` ${year}` : '';
  if (treatment === 'ebitda') {
    return amount > 0 ? `Aumenta l’EBITDA${y} di ${a}` : `Riduce l’EBITDA${y} di ${a}`;
  }
  if (treatment === 'pfn') {
    return amount < 0 ? `Riduce la PFN${y} di ${a} — equity su` : `Aumenta la PFN${y} di ${a} — equity giù`;
  }
  return null;
}

// Same-theme heuristic (Fase N2): group a proposal to its counterpart in other exercises WITHOUT a
// backend theme key. We normalize the human label (or the observed fact) — lowercased, accents and
// punctuation stripped, digits/amounts dropped, whitespace collapsed — so "Compensi amministratori
// (2024)" and "Compensi amministratori" collapse to the same key. Fallback to the NI section when
// the label/fact is empty. This is presentation-only: it drives the "Stesso tema" jumps, never a
// decision or a copied amount between years.
export function proposalThemeKey(p: MANIProposalView): string {
  const source = (p.label || p.fattoOsservato || '').trim();
  const base = source || (p.section ? `sezione:${p.section}` : '');
  return base
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '') // strip diacritics
    .replace(/[0-9.,]+/g, ' ') // drop digits and amount separators
    .replace(/[^a-z\s]/g, ' ') // drop punctuation
    .replace(/\s+/g, ' ')
    .trim();
}

export function exerciseYearOf(closingDate?: string): string {
  return closingDate ? closingDate.slice(0, 4) : '';
}

export function pageCiteLabel(pageNo?: number): string | null {
  if (pageNo == null) return null;
  return `p. ${pageNo}`;
}

// ── Open acquisitions (procurements not yet bound to a filing) ──
// Surface every open acquisition so an in-progress procurement — or a stalled/failed one — never
// disappears with its search results. An acquisition is "in progress" only while a live job works
// it (inflight) in a pre-completion state; unknown/failed or a lost job is an exception to resume.

// The exercise fragment of an acquisition sentence ("esercizio 2023" | "bilancio").
function acquisitionExercisePhrase(a: MAFilingAcquisitionView): string {
  const year = a.closingDate ? a.closingDate.slice(0, 4) : '';
  return year ? `esercizio ${year}` : 'bilancio';
}

// True while a live job is actively procuring the balance sheet (calm "in corso" pill).
export function acquisitionInProgress(a: MAFilingAcquisitionView): boolean {
  return a.inflight && (a.status === 'intent' || a.status === 'requested' || a.status === 'downloaded');
}

// The factual "in corso" line for an actively-procuring acquisition. No counters.
export function acquisitionProgressLabel(a: MAFilingAcquisitionView): string {
  return `Acquisizione ${acquisitionExercisePhrase(a)} in corso…`;
}

// The exception badge for a stalled/failed acquisition (danger for failed, warning otherwise).
export function acquisitionExceptionBadge(a: MAFilingAcquisitionView): { variant: StatusBadgeVariant; label: string } {
  if (a.status === 'failed') return { variant: 'danger', label: 'Acquisizione non riuscita' };
  return { variant: 'warning', label: 'Acquisizione interrotta' };
}

// The composed factual sentence for a stalled/failed acquisition, with the vendor error appended
// when present. "failed" reads as non riuscita; anything else (unknown, or a lost job) as interrotta.
export function acquisitionExceptionFact(a: MAFilingAcquisitionView): string {
  const phrase = acquisitionExercisePhrase(a);
  const base = a.status === 'failed' ? `Acquisizione ${phrase} non riuscita.` : `Acquisizione ${phrase} interrotta.`;
  const detail = a.error?.trim();
  return detail ? `${base} ${detail}` : base;
}
