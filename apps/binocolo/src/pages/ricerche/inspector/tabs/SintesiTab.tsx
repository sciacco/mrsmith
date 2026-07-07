import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { MATarget } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import {
  dossierHref,
  formatCompactEuro,
  freshnessClass,
  freshnessLabel,
  numberFormat,
  ragClass,
} from './format';

export function SintesiTab({
  target,
  initiativeId,
  initiativeTitle,
}: {
  target: MATarget;
  initiativeId?: string;
  initiativeTitle?: string;
}) {
  const wv = target.webValidation;
  const deep = target.deep;
  const rationale = target.rationale || wv?.finalDecision?.reason || 'Nessun razionale disponibile.';
  const dossier = dossierHref({ initiativeId, companyKey: target.companyKey });
  const equityRange =
    deep?.status === 'ready' &&
    deep?.valuation &&
    deep.valuation.equityLow != null &&
    deep.valuation.equityHigh != null
      ? `${formatCompactEuro(deep.valuation.equityLow)} – ${formatCompactEuro(deep.valuation.equityHigh)}`
      : '—';

  return (
    <div className={styles.tabBody}>
      <div className={styles.rationale}>
        <b>Rationale del gate.</b> {rationale}
      </div>

      <div className={styles.grid3}>
        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Numeri struttura</p>
          <dl className={styles.kv}>
            <dt>Fatturato</dt>
            <dd>
              {target.turnover != null ? `${formatCompactEuro(target.turnover)}` : <span className={styles.absent}>—</span>}
              {target.turnoverYear ? <small className={styles.muted}> ({target.turnoverYear})</small> : null}
            </dd>
            <dt>Dipendenti</dt>
            <dd>{target.employees != null ? numberFormat.format(target.employees) : <span className={styles.absent}>—</span>}</dd>
            <dt>Stato attività</dt>
            <dd>
              {target.activityStatus ?? '—'} <HiddenField label="activityStatus" />
            </dd>
          </dl>
        </div>

        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Web validation</p>
          <dl className={styles.kv}>
            <dt>Dominio</dt>
            <dd className={styles.mono}>{wv?.selectedDomain ?? <span className={styles.absent}>—</span>}</dd>
            <dt>Freshness</dt>
            <dd>
              <span className={`${styles.fresh} ${freshnessClass(wv?.freshness)}`}>{freshnessLabel(wv?.freshness)}</span>{' '}
              <HiddenField label="freshness" />
            </dd>
            <dt>Stato</dt>
            <dd>{wv?.webValidationState ?? '—'}</dd>
          </dl>
        </div>

        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Analisi approfondita</p>
          <dl className={styles.kv}>
            <dt>Stato</dt>
            <dd>
              {deep ? (
                <span className={`${styles.rag} ${deep.status === 'ready' ? (styles.ragGreen ?? '') : deep.status === 'failed' ? (styles.ragRed ?? '') : (styles.ragNa ?? '')}`}>
                  {deep.status}
                </span>
              ) : (
                <span className={styles.absent}>—</span>
              )}
            </dd>
            <dt>RAG overall</dt>
            <dd>
              {deep?.scorecard?.overallRag ? (
                <span className={`${styles.rag} ${ragClass(deep.scorecard.overallRag)}`}>{deep.scorecard.overallRag}</span>
              ) : (
                <span className={styles.absent}>—</span>
              )}
            </dd>
            <dt>Banda equity</dt>
            <dd>{equityRange}</dd>
          </dl>
        </div>
      </div>

      <div className={styles.linkRow}>
        {dossier ? (
          <Link className={styles.dossierLink} to={dossier}>
            Apri dossier <Icon name="external-link" size={14} />
            {initiativeTitle ? <small>nell'iniziativa «{initiativeTitle}»</small> : null}
          </Link>
        ) : null}
        <span className={styles.muted}>La lettura di tesi context-scoped vive nel dossier.</span>
      </div>
    </div>
  );
}
