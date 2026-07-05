import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { MADeepAnalysis } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import { DeepBrief, DeepQualityFlags, DeepReconciliation, DeepScorecard, DeepValuation } from '../deep/DeepComponents';
import styles from '../Inspector.module.css';
import { dossierHref, formatDateTime } from './format';

export function DeepTab({
  deep,
  initiativeId,
  initiativeTitle,
  companyKey,
}: {
  deep?: MADeepAnalysis;
  initiativeId?: string;
  initiativeTitle?: string;
  companyKey?: string;
}) {
  const dossier = dossierHref({ initiativeId, companyKey });

  // Stato D — deep nil
  if (!deep) {
    return <DeepEmptyNil initiativeId={initiativeId} initiativeTitle={initiativeTitle} companyKey={companyKey} />;
  }

  // Stato B — queued / running
  if (deep.status === 'queued' || deep.status === 'running') {
    return (
      <div className={styles.deepEmpty}>
        <span className={`${styles.deepEmptyPill} ${styles.deepEmptyRun}`}>
          <span className={styles.pulseDot} />
          {deep.status === 'queued' ? 'In coda' : 'Analisi completa in corso…'}
        </span>
        <p className={styles.deepEmptyText}>
          Il deep cached è in produzione. Appare qui automaticamente quando è ready.
        </p>
        <span className={styles.deepEmptyMono}>deep.updatedAt · {formatDateTime(deep.updatedAt)}</span>
      </div>
    );
  }

  // Stato C — failed
  if (deep.status === 'failed') {
    return (
      <div className={styles.tabBody}>
        <div className={styles.deepEmpty}>
          <span className={`${styles.deepEmptyPill} ${styles.deepEmptyFailed}`}>Analisi non riuscita</span>
          <p className={styles.deepEmptyText}>
            Il deep è fallito in un'esecuzione precedente.
          </p>
          <span className={styles.deepEmptyMono}>deep.errorCode · {deep.errorCode ?? 'n/d'}</span>
          <span className={styles.deepEmptyMono}>deep.updatedAt · {formatDateTime(deep.updatedAt)}</span>
          {dossier ? (
            <Link className={styles.dossierLink} to={dossier}>
              Apri dossier <Icon name="external-link" size={14} />
              <small>per ritentare</small>
            </Link>
          ) : null}
        </div>
      </div>
    );
  }

  // Stato A — ready
  const scorecard = deep.scorecard;
  const valuation = deep.valuation;
  const brief = deep.brief;
  return (
    <div className={styles.tabBody}>
      {scorecard ? <DeepScorecard scorecard={scorecard} /> : null}
      <div className={styles.grid2}>
        <DeepReconciliation reconciliation={scorecard?.reconciliation} />
        <DeepQualityFlags flags={scorecard?.qualityFlags} />
      </div>
      {valuation ? <DeepValuation valuation={valuation} /> : null}
      {brief ? <DeepBrief brief={brief} /> : null}

      <div className={styles.deepFooter}>
        <span>Deep cached aggiornato il {formatDateTime(deep.updatedAt)}</span>
        <span>·</span>
        <span>costo {deep.costEur != null ? `${deep.costEur.toLocaleString('it-IT', { maximumFractionDigits: 2 })} €` : 'n/d'} <HiddenField label="deep.costEur" /></span>
        {deep.errorCode ? <><span>·</span><span>errorCode: {deep.errorCode}</span></> : null}
        {dossier ? (
          <Link className={styles.dossierLink} to={dossier} style={{ marginLeft: 'auto' }}>
            Apri dossier <Icon name="external-link" size={14} />
          </Link>
        ) : null}
      </div>
    </div>
  );
}

// Stato D — nil: percorso ramificato (initiativeId → dossier; altrimenti → ricerche)
function DeepEmptyNil({ initiativeId, initiativeTitle, companyKey }: { initiativeId?: string; initiativeTitle?: string; companyKey?: string }) {
  const dossier = dossierHref({ initiativeId, companyKey });
  return (
    <div className={styles.deepEmpty}>
      <span className={`${styles.deepEmptyPill} ${styles.deepEmptyNa}`}>Nessuna analisi completa</span>
      <p className={styles.deepEmptyText}>
        Nessuna analisi completa per questa azienda.
      </p>
      {dossier ? (
        <Link className={styles.dossierLink} to={dossier}>
          Apri dossier <Icon name="external-link" size={14} />
          {initiativeTitle ? <small>nell'iniziativa «{initiativeTitle}»</small> : null}
        </Link>
      ) : initiativeId ? (
        <Link className={styles.dossierLink} to={`/iniziative/${initiativeId}`}>
          Apri board iniziativa <Icon name="external-link" size={14} />
        </Link>
      ) : (
        <>
          <p className={styles.deepEmptyText} style={{ fontStyle: 'italic' }}>
            Associa la ricerca a un'iniziativa per abilitare l'analisi completa di questa azienda.
          </p>
          <Link className={styles.dossierLink} to="/ricerche">
            Vai alle ricerche <Icon name="external-link" size={14} />
          </Link>
        </>
      )}
    </div>
  );
}
