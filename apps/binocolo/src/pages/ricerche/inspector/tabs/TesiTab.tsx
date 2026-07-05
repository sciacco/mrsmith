import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { MATarget, MATargetAdjustment } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { dossierHref } from './format';

export function TesiTab({
  target,
  initiativeId,
  initiativeTitle,
}: {
  target: MATarget;
  initiativeId?: string;
  initiativeTitle?: string;
}) {
  const thesisFit = (target.adjustments ?? []).find((a: MATargetAdjustment) => a.code === 'thesis-fit');
  const legacyReading = target.deep?.brief?.thesisReading?.trim();
  const dossier = dossierHref({ initiativeId, companyKey: target.companyKey });

  return (
    <div className={styles.tabBody}>
      <div className={styles.caveat}>
        <span className={styles.tcIco}>i</span>
        <div>
          <b>La lettura di tesi è context-scoped all'iniziativa.</b> Questo target mostra solo il
          fattore di tesi applicato allo score in questa sessione. Per fit, sinergie e postura
          valutativa, apri il dossier dell'iniziativa collegata.
        </div>
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>
          Fattore thesis-fit (in questa sessione) <HiddenField label="adjustments[thesis-fit]" />
        </p>
        {thesisFit ? (
          <div className={styles.rowsList}>
            <div className={`${styles.rowline} ${styles.rowlineNeutral}`}>
              <div>
                <div className={styles.rlMain}>{thesisFit.label || thesisFit.code}</div>
                <div className={styles.rlSub}>Fattore applicato allo score in questa sessione.</div>
              </div>
              <span className={styles.rlVal}>× {thesisFit.factor.toLocaleString('it-IT', { maximumFractionDigits: 2 })}</span>
            </div>
          </div>
        ) : (
          <p className={styles.cardSub} style={{ margin: 0 }}>
            Nessun fattore thesis-fit registrato per questo target.
          </p>
        )}
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>
          Lettura legacy (pre-v3) <HiddenField label="deep.brief.thesisReading" />
        </p>
        {legacyReading ? (
          <p className={styles.rlMain} style={{ fontWeight: 400, color: 'var(--color-text-secondary)' }}>{legacyReading}</p>
        ) : (
          <p className={styles.muted} style={{ fontStyle: 'italic', margin: 0 }}>— vuota —</p>
        )}
        <p className={styles.cardSub} style={{ margin: '6px 0 0' }}>
          Le run v3 non popolano questo campo; la lettura vera vive come <code>MACardThesisReading</code> nel dossier.
        </p>
      </div>

      {dossier ? (
        <Link className={styles.dossierLink} to={dossier}>
          Apri dossier <Icon name="external-link" size={14} />
          <small>lettura di tesi context-scoped{initiativeTitle ? ` · «${initiativeTitle}»` : ''}</small>
        </Link>
      ) : null}
    </div>
  );
}
