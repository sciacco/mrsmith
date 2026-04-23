import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import { useConfigSummary } from '../api/queries';
import { errorMessage } from '../lib/format';
import {
  RESOURCE_GROUPS,
  RESOURCE_KEYS,
  RESOURCE_META,
  type ResourceGroup,
  type ResourceMeta,
} from '../lib/resourceMeta';
import styles from './ConfigurationIndexPage.module.css';
import shared from './shared.module.css';

export function ConfigurationIndexPage() {
  const summary = useConfigSummary();

  return (
    <section className={shared.page}>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Configurazione</h1>
          <p className={shared.pageSubtitle}>
            Tabelle di riferimento usate nel registro manutenzioni.
          </p>
        </div>
      </div>

      {summary.error ? (
        <div className={styles.summaryError}>
          {errorMessage(summary.error, 'Conteggi non disponibili. Le risorse sono comunque accessibili.')}
        </div>
      ) : null}

      <nav className={styles.groups} aria-label="Risorse di configurazione">
        {RESOURCE_GROUPS.map((group) => (
          <ResourceGroupSection
            key={group.id}
            group={group}
            summary={summary.data ?? null}
            summaryLoading={summary.isLoading}
          />
        ))}
      </nav>
    </section>
  );
}

function ResourceGroupSection({
  group,
  summary,
  summaryLoading,
}: {
  group: { id: ResourceGroup; label: string };
  summary: Record<string, { active: number; inactive: number }> | null;
  summaryLoading: boolean;
}) {
  const items = RESOURCE_KEYS.map((key) => RESOURCE_META[key]).filter(
    (meta) => meta.group === group.id,
  );
  return (
    <div className={styles.group}>
      <h2 className={styles.groupTitle}>{group.label}</h2>
      <div className={styles.grid}>
        {items.map((meta) => (
          <ResourceCard
            key={meta.key}
            meta={meta}
            counts={summary?.[meta.key] ?? null}
            loading={summaryLoading}
          />
        ))}
      </div>
    </div>
  );
}

function ResourceCard({
  meta,
  counts,
  loading,
}: {
  meta: ResourceMeta;
  counts: { active: number; inactive: number } | null;
  loading: boolean;
}) {
  const isEmpty = counts !== null && counts.active === 0 && counts.inactive === 0;
  return (
    <Link to={`/manutenzioni/configurazione/${meta.key}`} className={styles.card}>
      <div className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>{meta.title}</h3>
        {isEmpty ? (
          <span className={styles.emptyBadge}>
            <Icon name="triangle-alert" size={12} />
            Da configurare
          </span>
        ) : null}
      </div>
      <p className={styles.cardDescription}>{meta.shortDescription}</p>
      <div className={styles.cardFooter}>
        {loading ? (
          <span className={styles.counterSkeleton} aria-hidden="true" />
        ) : counts ? (
          <span className={styles.counters}>
            {counts.active} attivi
            <span className={styles.dot}>·</span>
            {counts.inactive} non attivi
          </span>
        ) : (
          <span className={styles.counters}>—</span>
        )}
        <Icon name="chevron-right" size={18} className={styles.chevron} />
      </div>
    </Link>
  );
}
