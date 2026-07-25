import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { MATarget, MATargetOutcome } from '../../../../api/types';
import { CompanyRegistrySection } from '../../../iniziative/CompanyRegistrySection';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { dossierHref, formatDateTime } from './format';

function RatingStars({ rating }: { rating: number }) {
  const excluded = rating === -1;
  return (
    <span>
      <span className={styles.stars}>
        {[1, 2, 3].map((v) => (
          <span key={v} className={v <= rating ? '' : styles.off}>★</span>
        ))}
      </span>
      {excluded ? <span className={styles.starsExcluded}>escluso</span> : null}
    </span>
  );
}

function outcomeBadgeClass(event: MATargetOutcome['event']): string {
  switch (event) {
    case 'buon_lead': return styles.outcomeBadgeGood ?? '';
    case 'no_go': return styles.outcomeBadgeNoGo ?? '';
    default: return styles.outcomeBadge ?? '';
  }
}

export function RegistroTab({
  target,
  initiativeId,
}: {
  target: MATarget;
  initiativeId?: string;
}) {
  const rating = target.rating;
  const outcomes = target.outcomes ?? [];
  const dossier = dossierHref({ initiativeId, companyKey: target.companyKey });
  // Mai la P.IVA come chiave: fatti e note finirebbero su un'identità inventata
  // dal client (issue #86). Senza chiave il pannello non si mostra — la guardia
  // c'è già più sotto.
  const companyKey = target.companyKey ?? '';

  return (
    <div className={styles.tabBody}>
      <div className={styles.grid2}>
        {/* rating in questa sessione */}
        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Rating in questa sessione <HiddenField label="rating" /></p>
          {rating != null && rating !== 0 ? (
            <RatingStars rating={rating} />
          ) : (
            <p className={styles.muted} style={{ margin: 0 }}>Nessun rating assegnato in questa sessione.</p>
          )}
        </div>

        {/* specchietto outcomes count (read-only mirror) */}
        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Outcomes <HiddenField label="outcomes[]" /></p>
          {outcomes.length === 0 ? (
            <p className={styles.muted} style={{ margin: 0 }}>Nessun evento di lavorazione.</p>
          ) : (
            <p className={styles.rlMain} style={{ margin: 0 }}>
              {outcomes.length} evento{outcomes.length > 1 ? 'i' : ''} ·{' '}
              <span className={styles.muted}>ultimo {formatDateTime(outcomes.at(-1)?.createdAt)}</span>
            </p>
          )}
        </div>
      </div>

      {/* outcomes timeline (read-only) */}
      {outcomes.length > 0 ? (
        <div className={styles.card}>
          <p className={styles.lab}>Outcomes / eventi di lavorazione</p>
          <div className={styles.outcomes}>
            {outcomes.map((o) => (
              <div key={o.id} className={styles.outcome}>
                <span className={styles.outcomeDate}>{formatDateTime(o.createdAt)}</span>
                <span className={styles.outcomeText}>
                  {o.note || o.event}
                  {o.createdByEmail ? <small className={styles.muted}> · {o.createdByEmail}</small> : null}
                </span>
                <span className={`${styles.outcomeBadge} ${outcomeBadgeClass(o.event)}`}>{o.event}</span>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {/* registro azienda (read-only mirror del card-dossier) */}
      {companyKey ? (
        <CompanyRegistrySection
          companyKey={companyKey}
          vatCode={target.vatCode}
          companyName={target.companyName}
          readOnly
        />
      ) : (
        <p className={styles.muted}>Registro non disponibile: chiave azienda mancante.</p>
      )}

      <div className={styles.linkRow}>
        <span className={styles.muted}>Diario completo e IRL nel dossier di lavorazione.</span>
        {dossier ? (
          <Link className={styles.dossierLink} to={dossier}>
            Apri dossier <Icon name="external-link" size={14} />
          </Link>
        ) : null}
      </div>
    </div>
  );
}
