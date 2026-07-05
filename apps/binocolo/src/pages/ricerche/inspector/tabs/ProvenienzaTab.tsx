import { Link } from 'react-router-dom';
import type { MATarget } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { formatDateTime } from './format';

// `createdAt` è presente nel payload reale ma non nel tipo MATarget; lo si legge
// in modo difensivo (vincolo frontend-only: nessuna modifica ai tipi/backend).
type MATargetWithTimestamps = MATarget & { createdAt?: string };

export function ProvenienzaTab({
  target,
  strategyVersionId,
}: {
  target: MATarget;
  strategyVersionId?: string;
}) {
  const t = target as MATargetWithTimestamps;
  const wv = target.webValidation;
  const deep = target.deep;

  return (
    <div className={styles.tabBody}>
      <div className={styles.card}>
        <div className={styles.provList}>
          <div className={styles.provItem}>
            <span className={styles.provK}>target id</span>
            <span className={`${styles.provV} ${styles.mono}`}>{target.id}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>session id</span>
            <span className={`${styles.provV} ${styles.mono}`}>{target.sessionId}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>run id</span>
            <span className={`${styles.provV} ${styles.mono}`}>{target.runId}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>strategy version</span>
            <span className={styles.provV}>
              {strategyVersionId ? <span className={styles.mono}>{strategyVersionId}</span> : '—'}{' '}
              <HiddenField label="strategyVersionId" />
              <Link className={styles.provLink} to={`/ricerche/${target.sessionId}`} style={{ marginLeft: 8 }}>
                Apri ricerca ↗
              </Link>
            </span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>score version</span>
            <span className={styles.provV}>
              {target.scoreVersion ? `v${target.scoreVersion}` : '—'} <HiddenField label="scoreVersion" />
            </span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>enrichment level</span>
            <span className={styles.provV}>{target.enrichmentLevel ?? '—'}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>creato il</span>
            <span className={styles.provV}>{formatDateTime(t.createdAt)}</span>
          </div>
        </div>
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>Pipeline web validation</p>
        <div className={styles.provList}>
          <div className={styles.provItem}>
            <span className={styles.provK}>pipelineVersion</span>
            <span className={`${styles.provV} ${styles.mono}`}>{wv?.pipelineVersion ?? '—'}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>llm model</span>
            <span className={`${styles.provV} ${styles.mono}`}>{wv?.llmModel ?? wv?.llmModelId ?? '—'}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>llm prompt id</span>
            <span className={`${styles.provV} ${styles.mono}`}>{wv?.llmPromptId ?? '—'}</span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>inputHash / kwHash</span>
            <span className={`${styles.provV} ${styles.mono}`}>
              {wv?.inputHash ?? '—'} / {wv?.keywordSetHash ?? '—'} <HiddenField label="inputHash / keywordSetHash" />
            </span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>validation updated</span>
            <span className={styles.provV}>
              {formatDateTime(wv?.updatedAt)}{wv?.updatedByEmail ? ` · ${wv.updatedByEmail}` : ''}
            </span>
          </div>
          <div className={styles.provItem}>
            <span className={styles.provK}>deep updated</span>
            <span className={styles.provV}>{formatDateTime(deep?.updatedAt)}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
