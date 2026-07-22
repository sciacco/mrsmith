import { Icon, StatusBadge } from '@mrsmith/ui';
import type { DomainIdentityPresentation } from '../../lib/domainIdentity';
import type { MATarget, PipelineWebValidationState } from '../../api/types';
import styles from './CompanyPanels.module.css';

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyStateIcon} aria-hidden="true"><Icon name="network" size={28} /></span>
      <h2>{title}</h2><p>{text}</p>
    </div>
  );
}

export function hasWebVerificationDetail(target?: MATarget): boolean {
  return Boolean(target?.webValidation);
}

function validationStateLabel(state: PipelineWebValidationState): string {
  switch (state) {
    case 'confirmed': return 'Attività coerente';
    case 'deprioritized': return 'Coerenza debole';
    case 'rejected': return 'Attività non coerente';
    case 'unclear': return 'Attività da verificare';
    case 'analysis_unavailable': return 'Valutazione non disponibile';
    case 'domain_unresolved': return 'Dominio da identificare';
    case 'no_website_declared': return 'Nessun sito dichiarato';
  }
}

function validationStateVariant(state: PipelineWebValidationState): 'success' | 'warning' | 'danger' | 'neutral' {
  if (state === 'confirmed') return 'success';
  if (state === 'rejected') return 'danger';
  if (state === 'deprioritized' || state === 'unclear' || state === 'domain_unresolved') return 'warning';
  return 'neutral';
}

function candidateVerdictLabel(verdict: string): string {
  switch (verdict) {
    case 'strong_match': return 'Molto in tesi';
    case 'match': return 'In tesi';
    case 'weak_match': return 'Debole';
    case 'no_match': return 'Fuori tesi';
    case 'unclear': return 'Da chiarire';
    default: return verdict || 'n.d.';
  }
}

function EvidenceList({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) return null;
  return <div className={styles.webListBlock}><span>{title}</span><ul>{items.map((item) => <li key={item}>{item}</li>)}</ul></div>;
}

export function WebVerificationDetail({ target, domainIdentity }: { target: MATarget; domainIdentity: DomainIdentityPresentation }) {
  const validation = target.webValidation;
  if (!validation) return <EmptyState title="Verifica sito non disponibile" text="Nessuna verifica web registrata per questo target." />;

  const analysis = validation.candidateMatchAnalysis;
  const negativeOrMissing = analysis ? [...analysis.evidenceAgainst, ...analysis.missingEvidence] : [];

  return (
    <div className={styles.webStack}>
      <article className={styles.webCard}>
        <div className={styles.webCardHeader}>
          <div><h3>Identità del dominio</h3><p>{domainIdentity.detail || domainIdentity.historicalNote || 'Stato corrente del collegamento tra azienda e dominio.'}</p></div>
          <StatusBadge value={domainIdentity.label} variant={domainIdentity.variant} dot={false} />
        </div>
        <div className={styles.webFactsGrid}>
          <div className={styles.webFact}><span>{domainIdentity.source === 'registry' ? 'Dominio corrente' : 'Dominio analizzato'}</span><p>{domainIdentity.domain || 'n.d.'}</p></div>
          <div className={styles.webFact}><span>Provenienza</span><p>{domainIdentity.sourceLabel || 'n.d.'}</p></div>
          <div className={styles.webFact}><span>Stato identità</span><p>{domainIdentity.label}</p></div>
          <div className={styles.webFact}><span>Aggiornamento</span><p>{new Date(validation.updatedAt).toLocaleString('it-IT')}</p></div>
        </div>
        {domainIdentity.historicalNote ? <div className={styles.webNote}><span>Contesto della ricerca</span><p>{domainIdentity.historicalNote}</p></div> : null}
      </article>

      <article className={styles.webCard}>
        <div className={styles.webCardHeader}>
          <div><h3>Coerenza dell’attività con la tesi</h3><p>{validation.finalDecision.reason}</p></div>
          <StatusBadge value={validationStateLabel(validation.webValidationState)} variant={validationStateVariant(validation.webValidationState)} dot={false} />
        </div>
        <div className={styles.webFactsGrid}>
          <div className={styles.webFact}><span>Verdetto</span><p>{validationStateLabel(validation.webValidationState)}</p></div>
          <div className={styles.webFact}><span>Lettura attività</span><p>{validation.finalDecision.analystVerdict ? candidateVerdictLabel(validation.finalDecision.analystVerdict) : 'n.d.'}</p></div>
        </div>
        {validation.finalDecision.reasons.length ? <EvidenceList title="Motivi dell’esito" items={validation.finalDecision.reasons.slice(0, 6)} /> : null}
      </article>

      {analysis ? (
        <article className={styles.webCard}>
          <div className={styles.webCardHeader}>
            <div><h3>Lettura estesa del sito</h3><p>{analysis.rationale}</p></div>
            <StatusBadge value={candidateVerdictLabel(analysis.verdict)} variant="neutral" dot={false} />
          </div>
          <div className={styles.webFactsGrid}>
            <div className={styles.webFact}><span>Settore</span><p>{analysis.sectorFit || 'n.d.'}</p></div>
            <div className={styles.webFact}><span>Attività rilevante</span><p>{analysis.businessFit || 'n.d.'}</p></div>
          </div>
          <div className={styles.webListGrid}><EvidenceList title="Elementi a favore" items={analysis.evidenceFor} /><EvidenceList title="Elementi da chiarire" items={negativeOrMissing} /></div>
          {analysis.negativeSignals.length ? <div className={styles.webNote}><span>Segnali contrari</span><p>{analysis.negativeSignals.join(' · ')}</p></div> : null}
          {analysis.conceptAliases.length ? <div className={styles.webNote}><span>Termini ricondotti alla tesi</span><div className={styles.webAliasList}>{analysis.conceptAliases.map((alias) => <p key={`${alias.term}-${alias.matchedConcept}`}><strong>{alias.term}</strong> → {alias.matchedConcept}{alias.evidence ? ` · ${alias.evidence}` : ''}</p>)}</div></div> : null}
        </article>
      ) : validation.candidateMatchError ? <div className={styles.webWarning}>Lettura estesa non disponibile.</div> : null}
    </div>
  );
}
