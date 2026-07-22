import { Button, useToast } from '@mrsmith/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useApiClient } from '../../api/client';
import type { MABriefRegenerateResponse, MACompanyOverview, MADeepValuation } from '../../api/types';
import { dateTimeLabel } from '../../pages/ricerche/helpers';
import { DeepValuation, formatDeepCompactEuro } from '../deep/DeepComponents';
import { exerciseYear, filingErrorMessage, formatSignedCompactEuro } from './filingFacts';
import styles from './filings.module.css';

// Block-2 extension (issue #78, Fase 9, deliverable D): the deposited-filing reading OF the
// valuation — the adjusted valuation co-present with the vendor baseline, the effective deltas,
// the doc-vs-vendor reconciliation flag, and the brief-staleness action. Facts only; every
// sentence is composed from the structured MAAdjustedView. Exception-only: a canonical filing
// with no effect and no pending point announces nothing.
export function DepositedValuationView({
  overview,
  companyKey,
  baselineValuation,
}: {
  overview: MACompanyOverview;
  companyKey: string;
  baselineValuation?: MADeepValuation;
}) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const adjusted = overview.adjusted;

  const regenerate = useMutation({
    mutationFn: () =>
      api.post<MABriefRegenerateResponse>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/brief/regenerate`, {}),
    onSuccess: () => {
      toast('Brief rigenerato.', 'success');
      void queryClient.invalidateQueries({ queryKey: ['ma-company-overview', companyKey] });
    },
    onError: (error) => toast(filingErrorMessage(error), 'error'),
  });

  // Nothing to say unless there is an adjusted view or a stale brief to flag.
  if (!adjusted && !overview.briefStale) return null;

  const staleRow = overview.briefStale ? (
    <div className={styles.staleRow} role="status">
      <div>
        <span className={styles.factStrong}>Brief non aggiornato</span>
        <p>
          Il brief non riflette le ratifiche{overview.briefGeneratedAt ? ` successive al ${dateTimeLabel(overview.briefGeneratedAt)}` : ''}.
        </p>
        {regenerate.isError ? <p className={styles.inlineError}>{filingErrorMessage(regenerate.error)}</p> : null}
      </div>
      <Button size="sm" variant="secondary" loading={regenerate.isPending} onClick={() => regenerate.mutate()}>
        Rigenera brief
      </Button>
    </div>
  ) : null;

  const adjustedBody = adjusted ? renderAdjustedBody(adjusted, baselineValuation) : null;

  // No orphan header: render the wrapper + label only when there is real content — a stale brief
  // OR an adjusted body that actually produced output (an active/ambiguous/not_aligned view or a
  // divergent reconciliation). Pure normality — including no_effects with only pending points, now
  // that the pending-count link is gone (the year-group «N in attesa» chips carry it) — shows nothing.
  if (!staleRow && !adjustedBody) return null;

  return (
    <div className={styles.deposit}>
      <p className={styles.depositLabel}>Effetto dei bilanci depositati</p>
      {staleRow}
      {adjustedBody}
    </div>
  );
}

// renderAdjustedBody returns the adjusted-view facts, or null when there is nothing to say
// (no_filing, or no_effects with no divergent reconciliation) — the wrapper gate above depends on
// this null to stay silent. Pending points are NOT surfaced here: they are visible one row below on
// the year-group headers («N in attesa»), so a link back to them would be a decorative duplicate.
function renderAdjustedBody(
  adjusted: NonNullable<MACompanyOverview['adjusted']>,
  baselineValuation: MADeepValuation | undefined,
): ReactNode {
  const year = exerciseYear(adjusted.baselineExercise);
  const recon =
    adjusted.reconciliation?.divergent ? <ReconciliationRow reconciliation={adjusted.reconciliation} /> : null;

  if (adjusted.status === 'active') {
    const ratifiche = adjusted.effects.length;
    return (
      <>
        {adjusted.adjustedValuation && baselineValuation ? (
          <div className={styles.valuationPair}>
            <div className={styles.valuationCol}>
              <p className={styles.valuationColHead}>Da bilancio vendor (esercizio {year})</p>
              <DeepValuation valuation={baselineValuation} />
            </div>
            <div className={styles.valuationCol}>
              <p className={styles.valuationColHead}>
                Aggiustata da bilanci depositati ({ratifiche} {ratifiche === 1 ? 'ratifica' : 'ratifiche'})
              </p>
              <DeepValuation valuation={adjusted.adjustedValuation} />
            </div>
          </div>
        ) : (
          <p className={styles.factLine}>
            Rettifiche ratificate presenti, ma la valutazione baseline non è disponibile per applicarle.
          </p>
        )}
        <dl className={styles.deltaRow}>
          <div>
            <dt>Δ EBITDA</dt>
            <dd className={styles.num}>{formatSignedCompactEuro(adjusted.deltaEbitda)}</dd>
          </div>
          <div>
            <dt>Δ PFN</dt>
            <dd className={styles.num}>{formatSignedCompactEuro(adjusted.deltaPfn)}</dd>
          </div>
        </dl>
        {adjusted.evSalesDeltaEbitdaNotApplied ? (
          <p className={styles.factLine}>
            Metodo EV/Sales: le rettifiche EBITDA non si applicano al ricavo; la Δ PFN è applicata nel bridge.
          </p>
        ) : null}
        {recon}
      </>
    );
  }

  if (adjusted.status === 'ambiguous') {
    const n = adjusted.ambiguousCount ?? 2;
    return (
      <p className={styles.factLine}>
        {n} depositi per l’esercizio {year}: valutazione aggiustata sospesa.
      </p>
    );
  }

  // not_aligned = fascicoli present but none on the baseline exercise → a real exception. no_filing
  // (no deposited fascicolo at all) is NOT announced here: it is normality, already covered by the
  // block-5 empty state and the "—" in the spine.
  if (adjusted.status === 'not_aligned') {
    return <p className={styles.factLine}>Nessun fascicolo allineato all’esercizio {year}.</p>;
  }

  // no_effects: normality → announce nothing here. A divergent reconciliation still surfaces; open
  // DD points are shown by the year-group «N in attesa» chips, not duplicated with a link.
  return recon;
}

function ReconciliationRow({
  reconciliation,
}: {
  reconciliation: NonNullable<NonNullable<MACompanyOverview['adjusted']>['reconciliation']>;
}) {
  const divergent = reconciliation.fields.filter((f) => f.delta != null && f.delta !== 0);
  const fields = divergent.length > 0 ? divergent : reconciliation.fields;
  return (
    <div className={styles.reconRow} role="status">
      <span className={styles.factStrong}>Il deposito diverge dal dato vendor</span>
      <ul className={styles.reconList}>
        {fields.map((f) => (
          <li key={f.name}>
            {f.name}: deposito {fmtOptEuro(f.filing)} · vendor {fmtOptEuro(f.vendor)}
            {f.delta != null ? ` (${formatSignedCompactEuro(f.delta)})` : ''}
          </li>
        ))}
      </ul>
    </div>
  );
}

function fmtOptEuro(value?: number): string {
  if (value == null) return 'n.d.';
  return formatDeepCompactEuro(value);
}
