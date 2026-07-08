import { Icon } from '@mrsmith/ui';
import type { MATarget, PipelineFinalAction, PipelineWebValidationState } from '../../api/types';
import styles from './CompanyPanels.module.css';

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyStateIcon} aria-hidden="true">
        <Icon name="network" size={28} />
      </span>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

export function hasWebVerificationDetail(target?: MATarget): boolean {
  return Boolean(target?.webValidation);
}

function finalActionLabel(action: PipelineFinalAction): string {
  switch (action) {
    case 'confirm':
      return 'Confermata';
    case 'deprioritize':
      return 'Da mettere in secondo piano';
    case 'reject':
      return 'Scartata';
    case 'needs_domain_review':
      return 'Dominio da verificare';
    case 'needs_business_validation':
      return 'Attività da verificare';
    case 'no_website_structured':
      return 'Senza sito utile';
  }
}

function validationStateLabel(state: PipelineWebValidationState): string {
  switch (state) {
    case 'confirmed':
      return 'Sito confermato';
    case 'deprioritized':
      return 'Priorità ridotta';
    case 'domain_unresolved':
      return 'Dominio non risolto';
    case 'analysis_unavailable':
      return 'Verifica non disponibile';
    case 'no_website_declared':
      return 'Nessun sito dichiarato';
    case 'rejected':
      return 'Sito non coerente';
    case 'unclear':
      return 'Da chiarire';
  }
}

function candidateVerdictLabel(verdict: string): string {
  switch (verdict) {
    case 'strong_match':
      return 'Molto in tesi';
    case 'match':
      return 'In tesi';
    case 'weak_match':
      return 'Debole';
    case 'no_match':
      return 'Fuori tesi';
    case 'unclear':
      return 'Da chiarire';
    default:
      return verdict || 'n.d.';
  }
}

function candidateActionLabel(action: string): string {
  switch (action) {
    case 'confirm':
      return 'Confermare';
    case 'review':
      return 'Rivedere';
    case 'downgrade':
      return 'Ridurre priorità';
    case 'reject':
      return 'Scartare';
    default:
      return action || 'n.d.';
  }
}

function confidenceLabel(value?: string): string {
  if (!value) return 'n.d.';
  if (value === 'alta') return 'confidenza alta';
  if (value === 'media') return 'confidenza media';
  if (value === 'bassa') return 'confidenza bassa';
  return value;
}

function formatScore(value?: number): string {
  if (value == null) return 'n.d.';
  return value.toLocaleString('it-IT');
}

function EvidenceList({ title, items }: { title: string; items?: string[] }) {
  if (!items || items.length === 0) return null;
  return (
    <div className={styles.webListBlock}>
      <span>{title}</span>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

export function WebVerificationDetail({ target }: { target: MATarget }) {
  const validation = target.webValidation;

  if (!validation) {
    return <EmptyState title="Verifica sito non disponibile" text="Nessuna verifica web registrata per questo target." />;
  }

  const analysis = validation.candidateMatchAnalysis;
  const negativeOrMissing = analysis ? [...analysis.evidenceAgainst, ...analysis.missingEvidence] : [];

  return (
    <div className={styles.webStack}>
      <article className={styles.webCard}>
        <div className={styles.webCardHeader}>
          <div>
            <h3>Esito della verifica</h3>
            <p>{validation.finalDecision.reason}</p>
          </div>
          <div className={styles.pillRow}>
            <span className={styles.statusPill}>{finalActionLabel(validation.finalDecision.finalAction)}</span>
            <span className={`${styles.statusPill} ${styles.neutralPill}`}>{validationStateLabel(validation.webValidationState)}</span>
          </div>
        </div>

        <div className={styles.webFactsGrid}>
          <div className={styles.webFact}>
            <span>Dominio selezionato</span>
            <p>{validation.selectedDomain ? `${validation.selectedDomain} · ${confidenceLabel(validation.domainConfidence)}` : 'n.d.'}</p>
          </div>
          <div className={styles.webFact}>
            <span>Conferma dominio</span>
            <p>{validation.selectedDomainPayload?.reasons?.slice(0, 2).join(' · ') || validation.finalDecision.reason || 'n.d.'}</p>
          </div>
          <div className={styles.webFact}>
            <span>Coerenza iniziale</span>
            <p>{formatScore(validation.finalDecision.deterministicScore)}</p>
          </div>
          <div className={styles.webFact}>
            <span>Verifica sito</span>
            <p>{formatScore(validation.finalDecision.webScore)} · {validationStateLabel(validation.webValidationState)}</p>
          </div>
          <div className={styles.webFact}>
            <span>Lettura attività</span>
            <p>
              {validation.finalDecision.analystVerdict
                ? `${candidateVerdictLabel(validation.finalDecision.analystVerdict)} · ${candidateActionLabel(validation.finalDecision.analystAction ?? '')}`
                : 'n.d.'}
            </p>
          </div>
          <div className={styles.webFact}>
            <span>Aggiornamento</span>
            <p>{new Date(validation.updatedAt).toLocaleString('it-IT')}</p>
          </div>
        </div>

        {validation.finalDecision.reasons.length > 0 ? (
          <div className={styles.webListBlock}>
            <span>Motivi dell'esito</span>
            <ul>
              {validation.finalDecision.reasons.slice(0, 6).map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </article>

      {analysis ? (
        <article className={styles.webCard}>
          <div className={styles.webCardHeader}>
            <div>
              <h3>Lettura estesa del sito</h3>
              <p>{analysis.rationale}</p>
            </div>
            <div className={styles.pillRow}>
              <span className={styles.statusPill}>{candidateVerdictLabel(analysis.verdict)}</span>
              <span className={`${styles.statusPill} ${styles.neutralPill}`}>{candidateActionLabel(analysis.recommendedAction)}</span>
            </div>
          </div>

          <div className={styles.webFactsGrid}>
            <div className={styles.webFact}>
              <span>Settore</span>
              <p>{analysis.sectorFit || 'n.d.'}</p>
            </div>
            <div className={styles.webFact}>
              <span>Attività rilevante</span>
              <p>{analysis.businessFit || 'n.d.'}</p>
            </div>
          </div>

          <div className={styles.webListGrid}>
            <EvidenceList title="Elementi a favore" items={analysis.evidenceFor} />
            <EvidenceList title="Elementi da chiarire" items={negativeOrMissing} />
          </div>

          {analysis.negativeSignals.length > 0 ? (
            <div className={styles.webNote}>
              <span>Segnali contrari</span>
              <p>{analysis.negativeSignals.join(' · ')}</p>
            </div>
          ) : null}

          {analysis.conceptAliases.length > 0 ? (
            <div className={styles.webNote}>
              <span>Termini ricondotti alla tesi</span>
              <div className={styles.webAliasList}>
                {analysis.conceptAliases.map((alias) => (
                  <p key={`${alias.term}-${alias.matchedConcept}`}>
                    <strong>{alias.term}</strong> → {alias.matchedConcept}
                    {alias.evidence ? ` · ${alias.evidence}` : ''}
                  </p>
                ))}
              </div>
            </div>
          ) : null}
        </article>
      ) : validation.candidateMatchError ? (
        <div className={styles.webWarning}>Lettura estesa non disponibile.</div>
      ) : null}
    </div>
  );
}
