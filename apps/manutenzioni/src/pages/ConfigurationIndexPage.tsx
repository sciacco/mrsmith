import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import { useConfigSummary, useLLMModels, useServiceDependencies, type ConfigSummary } from '../api/queries';
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
  summary: ConfigSummary | null;
  summaryLoading: boolean;
}) {
  const items = RESOURCE_KEYS.map((key) => RESOURCE_META[key]).filter(
    (meta) => meta.group === group.id && !meta.hiddenFromIndex,
  );
  const isAutomationGroup = group.id === 'automation';
  const isImpactGroup = group.id === 'impact';
  const hasDomainsCard = items.some((meta) => meta.key === 'technical-domains');
  return (
    <div className={styles.group}>
      <h2 className={styles.groupTitle}>{group.label}</h2>
      <div className={styles.grid}>
        {items.map((meta) => (
          <ResourceCard
            key={meta.key}
            meta={meta}
            counts={summary?.[meta.key] ?? null}
            summary={summary}
            loading={summaryLoading}
          />
        ))}
        {isImpactGroup ? <DependencyGraphCard /> : null}
        {isAutomationGroup ? <LLMModelsCard /> : null}
      </div>
      {hasDomainsCard ? (
        <div className={styles.groupFooter}>
          <Link to="/manutenzioni/configurazione/service-taxonomy" className={styles.secondaryLink}>
            Elenco servizi completo
            <Icon name="chevron-right" size={14} />
          </Link>
        </div>
      ) : null}
    </div>
  );
}

function DependencyGraphCard() {
  const dependencies = useServiceDependencies('all');
  const active = dependencies.data?.filter((item) => item.is_active).length ?? 0;
  const inactive = dependencies.data?.filter((item) => !item.is_active).length ?? 0;
  const isEmpty = !dependencies.isLoading && !dependencies.error && active + inactive === 0;
  return (
    <Link to="/manutenzioni/configurazione/dipendenze" className={styles.card}>
      <div className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>Grafo dipendenze</h3>
        {isEmpty ? (
          <span className={styles.emptyBadge}>
            <Icon name="triangle-alert" size={12} />
            Da configurare
          </span>
        ) : null}
      </div>
      <p className={styles.cardDescription}>Relazioni tra servizi usate per suggerire gli impatti.</p>
      <div className={styles.cardFooter}>
        {dependencies.isLoading ? (
          <span className={styles.counterSkeleton} aria-hidden="true" />
        ) : dependencies.error ? (
          <span className={styles.counters}>—</span>
        ) : (
          <span className={styles.counters}>
            {active} attive
            <span className={styles.dot}>·</span>
            {inactive} non attive
          </span>
        )}
        <Icon name="chevron-right" size={18} className={styles.chevron} />
      </div>
    </Link>
  );
}

function ResourceCard({
  meta,
  counts,
  summary,
  loading,
}: {
  meta: ResourceMeta;
  counts: { active: number; inactive: number } | null;
  summary: ConfigSummary | null;
  loading: boolean;
}) {
  const isEmpty = counts !== null && counts.active === 0 && counts.inactive === 0;
  const isDomainsCard = meta.key === 'technical-domains';
  const serviceCounts = summary?.['service-taxonomy'] ?? null;
  const title = meta.indexTitle ?? meta.title;
  const description = meta.indexDescription ?? meta.shortDescription;
  return (
    <Link to={`/manutenzioni/configurazione/${meta.key}`} className={styles.card}>
      <div className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>{title}</h3>
        {isEmpty ? (
          <span className={styles.emptyBadge}>
            <Icon name="triangle-alert" size={12} />
            Da configurare
          </span>
        ) : null}
      </div>
      <p className={styles.cardDescription}>{description}</p>
      <div className={styles.cardFooter}>
        {loading ? (
          <span className={styles.counterSkeleton} aria-hidden="true" />
        ) : counts ? (
          <span className={styles.counters}>
            {isDomainsCard && serviceCounts ? (
              <>
                {counts.active} domini
                <span className={styles.dot}>·</span>
                {serviceCounts.active} servizi
              </>
            ) : (
              <>
                {counts.active} attivi
                <span className={styles.dot}>·</span>
                {counts.inactive} non attivi
              </>
            )}
          </span>
        ) : (
          <span className={styles.counters}>—</span>
        )}
        <Icon name="chevron-right" size={18} className={styles.chevron} />
      </div>
    </Link>
  );
}

function LLMModelsCard() {
  const models = useLLMModels();
  const count = models.data?.length ?? 0;
  const isEmpty = !models.isLoading && !models.error && count === 0;
  return (
    <Link to="/manutenzioni/configurazione/modelli-llm" className={styles.card}>
      <div className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>Modelli AI</h3>
        {isEmpty ? (
          <span className={styles.emptyBadge}>
            <Icon name="triangle-alert" size={12} />
            Da configurare
          </span>
        ) : null}
      </div>
      <p className={styles.cardDescription}>Modelli usati dalle automazioni assistite.</p>
      <div className={styles.cardFooter}>
        {models.isLoading ? (
          <span className={styles.counterSkeleton} aria-hidden="true" />
        ) : models.error ? (
          <span className={styles.counters}>—</span>
        ) : (
          <span className={styles.counters}>
            {count} {count === 1 ? 'modello' : 'modelli'}
          </span>
        )}
        <Icon name="chevron-right" size={18} className={styles.chevron} />
      </div>
    </Link>
  );
}
