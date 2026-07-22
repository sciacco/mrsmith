import { Button, MoneyInput, Skeleton, StatusBadge, useToast } from '@mrsmith/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useApiClient } from '../../api/client';
import type {
  MAFilingProposalsResponse,
  MANIDecisionResponse,
  MANIDecisionView,
  MANIProposalView,
} from '../../api/types';
import { dateLabel, errorLabel } from '../../pages/ricerche/helpers';
import {
  directionLabel,
  filingErrorMessage,
  formatEuro,
  incertezzaLabel,
  pageCiteLabel,
  proposalNeedsTreatmentChoice,
  proposalStateBadge,
  treatmentLabel,
} from './filingFacts';
import styles from './filings.module.css';

// Deliverable C.4: ratification of the nota-integrativa proposals of a filing's ACTIVE reading
// run. Each proposal is an immutable observed fact; the analyst appends a decision (ratify /
// reject / revoke). The reducer semantics are mirrored in the form so an ineffective ratify is
// avoided (dd_only / uncertain need a treatment + signed amount).
export function ProposalsPanel({ filingId, companyKey }: { filingId: string; companyKey: string }) {
  const api = useApiClient();
  const query = useQuery({
    queryKey: ['ma-filing-proposals', filingId],
    queryFn: () => api.get<MAFilingProposalsResponse>(`/binocolo/v1/ma/filings/${encodeURIComponent(filingId)}/proposals`),
  });

  if (query.isLoading) {
    return (
      <div className={styles.proposalsWrap}>
        <Skeleton rows={3} />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className={styles.proposalsWrap}>
        <p className={styles.inlineError}>{errorLabel(query.error)}</p>
      </div>
    );
  }
  const proposals = query.data?.proposals ?? [];
  if (proposals.length === 0) {
    return (
      <div className={styles.proposalsWrap}>
        <p className={styles.subtle}>Nessuna rettifica proposta dalla nota integrativa per questo fascicolo.</p>
      </div>
    );
  }
  return (
    <div className={styles.proposalsWrap}>
      {proposals.map((p) => (
        <ProposalCard key={`${p.id}:${latestDecision(p.decisions)?.id ?? 'none'}`} proposal={p} filingId={filingId} companyKey={companyKey} />
      ))}
    </div>
  );
}

function latestDecision(decisions: MANIDecisionView[]): MANIDecisionView | undefined {
  return decisions.length > 0 ? decisions[decisions.length - 1] : undefined;
}

function ProposalCard({ proposal, filingId, companyKey }: { proposal: MANIProposalView; filingId: string; companyKey: string }) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const needsTreatment = proposalNeedsTreatmentChoice(proposal);
  const effective = latestRatify(proposal.decisions);
  const effTreatment = effective?.ratifiedTreatment || defaultTreatment(proposal);
  const effAmountWire = effective?.ratifiedAmount != null ? effective.ratifiedAmount.toFixed(2) : '';

  const [amount, setAmount] = useState<string>(
    effAmountWire || (proposal.importoRettifica != null ? proposal.importoRettifica.toFixed(2) : ''),
  );
  const [treatment, setTreatment] = useState<string>(effTreatment);
  const [reason, setReason] = useState<string>('');

  const decide = useMutation({
    mutationFn: (payload: { action: 'ratify' | 'reject' | 'revoke'; ratifiedAmount?: number; ratifiedTreatment?: string; reason?: string }) =>
      api.post<MANIDecisionResponse>(`/binocolo/v1/ma/proposals/${encodeURIComponent(proposal.id)}/decision`, payload),
    onSuccess: async (_res, variables) => {
      toast(
        variables.action === 'ratify' ? 'Rettifica ratificata.' : variables.action === 'reject' ? 'Proposta scartata.' : 'Ratifica revocata.',
        'success',
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['ma-filing-proposals', filingId] }),
        queryClient.invalidateQueries({ queryKey: ['ma-company-overview', companyKey] }),
      ]);
    },
    onError: (error) => toast(errorLabel(error), 'error'),
  });

  const treatmentChoiceMissing = needsTreatment && !treatment;
  const ratifyUnchanged =
    proposal.state === 'effective' && amount === effAmountWire && treatment === effTreatment;
  const ratifyDisabled = decide.isPending || !amount || treatmentChoiceMissing || ratifyUnchanged;

  const badge = proposalStateBadge(proposal.state);
  const isDDPoint = proposal.trattamentoCandidato === 'dd_only' && proposal.state !== 'effective';

  return (
    <article className={styles.proposalCard}>
      <div className={styles.proposalHead}>
        <span className={styles.factStrong}>{proposal.label || proposal.fattoOsservato}</span>
        <StatusBadge value={badge.label} variant={badge.variant} dot={false} />
      </div>
      {proposal.label && proposal.fattoOsservato !== proposal.label ? (
        <p className={styles.proposalFact}>{proposal.fattoOsservato}</p>
      ) : null}

      <dl className={styles.proposalFacts}>
        <div>
          <dt>Trattamento</dt>
          <dd>{treatmentLabel(proposal.trattamentoCandidato)}{isDDPoint ? ' · nessun effetto senza promozione' : ''}</dd>
        </div>
        <div>
          <dt>Verso</dt>
          <dd>{directionLabel(proposal.direction)}</dd>
        </div>
        {proposal.importoLordo != null ? (
          <div>
            <dt>Importo lordo</dt>
            <dd className={styles.num}>{formatEuro(proposal.importoLordo)}</dd>
          </div>
        ) : null}
        {proposal.importoRettifica != null ? (
          <div>
            <dt>Rettifica stimata</dt>
            <dd className={styles.num}>{formatEuro(proposal.importoRettifica)}</dd>
          </div>
        ) : null}
        {incertezzaLabel(proposal.incertezza) ? (
          <div>
            <dt>Incertezza</dt>
            <dd>{incertezzaLabel(proposal.incertezza)}</dd>
          </div>
        ) : null}
      </dl>

      {proposal.quote ? (
        <p className={styles.proposalQuote}>
          {pageCiteLabel(proposal.pageNo) ? <span className={styles.pageCite}>{pageCiteLabel(proposal.pageNo)}</span> : null}
          «{proposal.quote}»
        </p>
      ) : null}

      {effective ? (
        <p className={styles.subtle}>
          Ratificata{effective.actor ? ` da ${effective.actor}` : ''} il {dateLabel(effective.createdAt)}
          {effective.autoReconfirmedFrom ? ' · riconfermata automaticamente' : ''}
        </p>
      ) : null}

      <div className={styles.proposalForm}>
        <div className={styles.formRow}>
          <MoneyInput
            label="Importo rettifica (firmato)"
            value={amount}
            onChange={setAmount}
            allowNegative
          />
          {needsTreatment ? (
            <div className={styles.field}>
              <label htmlFor={`treatment-${proposal.id}`}>Trattamento</label>
              <select
                id={`treatment-${proposal.id}`}
                className={styles.select}
                value={treatment}
                onChange={(event) => setTreatment(event.target.value)}
              >
                <option value="">— scegli —</option>
                <option value="ebitda">EBITDA</option>
                <option value="pfn">PFN</option>
              </select>
            </div>
          ) : null}
        </div>
        <div className={styles.field}>
          <label htmlFor={`reason-${proposal.id}`}>Motivo (opzionale)</label>
          <input
            id={`reason-${proposal.id}`}
            type="text"
            className={styles.textInput}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={300}
            placeholder="Nota per la traccia decisionale"
          />
        </div>
        <div className={styles.formActions}>
          <Button
            size="sm"
            loading={decide.isPending}
            disabled={ratifyDisabled}
            onClick={() =>
              decide.mutate({
                action: 'ratify',
                ratifiedAmount: amount ? Number(amount) : undefined,
                ratifiedTreatment: needsTreatment ? treatment : undefined,
                reason: reason.trim() || undefined,
              })
            }
          >
            {proposal.state === 'effective' ? 'Aggiorna ratifica' : 'Ratifica'}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={decide.isPending || proposal.state === 'rejected'}
            onClick={() => decide.mutate({ action: 'reject', reason: reason.trim() || undefined })}
          >
            Scarta
          </Button>
          {proposal.state === 'effective' ? (
            <Button
              size="sm"
              variant="ghost"
              disabled={decide.isPending}
              onClick={() => decide.mutate({ action: 'revoke', reason: reason.trim() || undefined })}
            >
              Revoca
            </Button>
          ) : null}
        </div>
        {decide.isError ? <p className={styles.inlineError}>{filingErrorMessage(decide.error)}</p> : null}
      </div>
    </article>
  );
}

function latestRatify(decisions: MANIDecisionView[]): MANIDecisionView | undefined {
  // The current effective ratify is the latest decision only if it is a ratify (a later
  // reject/revoke supersedes it). Decisions are oldest-first.
  const latest = decisions.length > 0 ? decisions[decisions.length - 1] : undefined;
  return latest?.action === 'ratify' ? latest : undefined;
}

function defaultTreatment(proposal: MANIProposalView): string {
  return proposal.trattamentoCandidato === 'ebitda' || proposal.trattamentoCandidato === 'pfn'
    ? proposal.trattamentoCandidato
    : '';
}
