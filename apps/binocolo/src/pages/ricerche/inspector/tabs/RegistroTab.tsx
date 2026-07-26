import { useState } from 'react';
import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { MATarget } from '../../../../api/types';
import { ActivityTimeline } from '../../../../components/company/activity/ActivityTimeline';
import { useCompanyActivity } from '../../../../hooks/useCompanyActivity';
import { CompanyRegistrySection } from '../../../iniziative/CompanyRegistrySection';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { dossierHref } from './format';

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

export function RegistroTab({
  target,
  initiativeId,
}: {
  target: MATarget;
  initiativeId?: string;
}) {
  const rating = target.rating;
  const dossier = dossierHref({ initiativeId, companyKey: target.companyKey });
  // Mai la P.IVA come chiave: fatti e note finirebbero su un'identità inventata
  // dal client (issue #86). Senza chiave il pannello non si mostra — la guardia
  // c'è già più sotto.
  const companyKey = target.companyKey ?? '';
  const activity = useCompanyActivity(companyKey, { includeDeleted: true });
  const [annotationsOnly, setAnnotationsOnly] = useState(false);
  const [initiativeFilter, setInitiativeFilter] = useState('');
  const items = (activity.data?.items ?? []).filter((item) => {
    if (annotationsOnly && item.event !== 'nota') return false;
    if (!initiativeFilter) return true;
    if (item.initiativeId === initiativeFilter) return true;
    return activity.data?.sessions.find((session) => session.id === item.sessionId)?.initiativeId === initiativeFilter;
  });

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

        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Traccia attività</p>
          <p className={styles.rlMain} style={{ margin: 0 }}>
            {activity.data?.items.length ?? 0} elementi, incluse le annotazioni eliminate.
          </p>
        </div>
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>Attività e annotazioni</p>
        <div className={styles.linkRow}>
          <div>
            <button type="button" className={styles.dossierLink} onClick={() => setAnnotationsOnly(false)}>Traccia completa</button>
            {' · '}
            <button type="button" className={styles.dossierLink} onClick={() => setAnnotationsOnly(true)}>Annotazioni</button>
          </div>
          {(activity.data?.initiatives.length ?? 0) > 1 ? (
            <select value={initiativeFilter} onChange={(event) => setInitiativeFilter(event.target.value)} aria-label="Filtra attività per iniziativa">
              <option value="">Tutte le iniziative</option>
              {activity.data?.initiatives.map((initiative) => <option key={initiative.id} value={initiative.id}>{initiative.title}</option>)}
            </select>
          ) : null}
        </div>
        {activity.isError ? <p className={styles.muted} role="alert">Cronologia non disponibile.</p> : (
          <ActivityTimeline
            items={items}
            initiatives={activity.data?.initiatives}
            sessions={activity.data?.sessions}
            companyKey={companyKey}
            showTechnical
          />
        )}
      </div>

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
