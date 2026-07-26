import { useState } from 'react';
import { Button, Icon, SingleSelect, Skeleton } from '@mrsmith/ui';
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
  const initiativeOptions = (activity.data?.initiatives ?? []).map((initiative) => ({ value: initiative.id, label: initiative.title }));

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
          {activity.isLoading ? <Skeleton rows={1} /> : activity.isError ? (
            <p className={styles.activityErrorText}>Conteggio non disponibile.</p>
          ) : (
            <p className={styles.rlMain} style={{ margin: 0 }}>
              {items.length} {items.length === 1 ? 'elemento' : 'elementi'} con i filtri attivi.
            </p>
          )}
        </div>
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>Attività e annotazioni</p>
        <div className={styles.linkRow}>
          <div className={styles.activitySegmented} role="group" aria-label="Tipo di attività">
            <button type="button" className={!annotationsOnly ? styles.activitySegmentActive : ''} aria-pressed={!annotationsOnly} onClick={() => setAnnotationsOnly(false)}>Traccia completa</button>
            <button type="button" className={annotationsOnly ? styles.activitySegmentActive : ''} aria-pressed={annotationsOnly} onClick={() => setAnnotationsOnly(true)}>Annotazioni</button>
          </div>
          {initiativeOptions.length > 1 ? (
            <div className={styles.activityInitiativeFilter} role="group" aria-label="Filtra attività per iniziativa">
              <SingleSelect
                options={initiativeOptions}
                selected={initiativeFilter || null}
                onChange={(value) => setInitiativeFilter(value ?? '')}
                placeholder="Tutte le iniziative"
                allowClear
                clearLabel="Tutte le iniziative"
                ariaLabel="Filtra attività per iniziativa"
              />
            </div>
          ) : null}
        </div>
        {activity.isLoading ? <Skeleton rows={4} /> : activity.isError ? (
          <div className={styles.activityErrorState} role="alert">
            <span>Cronologia non disponibile.</span>
            <Button size="sm" variant="secondary" onClick={() => void activity.refetch()}>Riprova</Button>
          </div>
        ) : (
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
        <span className={styles.muted}>Attività completa e IRL nel dossier di lavorazione.</span>
        {dossier ? (
          <Link className={styles.dossierLink} to={dossier}>
            Apri dossier <Icon name="external-link" size={14} />
          </Link>
        ) : null}
      </div>
    </div>
  );
}
