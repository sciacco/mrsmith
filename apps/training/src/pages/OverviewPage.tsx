import { Link, useSearchParams } from 'react-router-dom';
import { Skeleton } from '@mrsmith/ui';
import { useOverviewKpis } from '../api/queries';
import type { OverviewException, OverviewFamily } from '../api/types';
import styles from './OverviewPage.module.css';

interface OverviewPageProps {
  isPeopleAdmin: boolean;
}

const SEVERITY_RANK: Record<OverviewException['severity'], number> = {
  critical: 0,
  warning: 1,
  info: 2,
};

function severitySymbol(severity: OverviewException['severity']): string {
  switch (severity) {
    case 'critical':
      return '●';
    case 'warning':
      return '●';
    case 'info':
      return '○';
  }
}

export function OverviewPage({ isPeopleAdmin }: OverviewPageProps) {
  const [params] = useSearchParams();
  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team') ?? '';
  const overview = useOverviewKpis({ year, team }, isPeopleAdmin);

  if (!isPeopleAdmin) {
    return <main className={styles.page}><p>Accesso riservato al team People.</p></main>;
  }

  if (overview.isLoading || !overview.data) {
    return <main className={styles.page}><Skeleton rows={5} /></main>;
  }

  const data = overview.data;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <h1>Overview</h1>
        <p className={styles.subtitle}>
          Stato formativo {data.year}{team ? ` · ${team}` : ''}.
        </p>
      </header>

      <div className={styles.grid}>
        <FamilyCard
          title="Esecuzione"
          family={data.esecuzione}
          subtitle="Iscrizioni completate sul totale del piano"
        />
        <FamilyCard
          title="Compliance"
          family={data.compliance}
          subtitle="Copertura regole obbligatorie"
        />
        <FamilyCard
          title="Budget"
          family={data.budget}
          subtitle={
            data.budget.calendar_alignment
              ? `Allineamento calendario: ${alignmentLabel(data.budget.calendar_alignment)}`
              : undefined
          }
        />
        <FamilyCard
          title="Engagement"
          family={data.engagement}
          subtitle={
            data.engagement.courses_per_person !== undefined
              ? `${data.engagement.courses_per_person.toFixed(1)} corsi / persona`
              : undefined
          }
        />
      </div>
    </main>
  );
}

function alignmentLabel(value: NonNullable<OverviewFamily['calendar_alignment']>): string {
  switch (value) {
    case 'in_linea':
      return 'in linea';
    case 'in_ritardo':
      return 'in ritardo';
    case 'in_anticipo':
      return 'in anticipo';
  }
}

function FamilyCard({ title, family, subtitle }: { title: string; family: OverviewFamily; subtitle?: string }) {
  const sortedExceptions = [...family.exceptions].sort(
    (a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity],
  );
  return (
    <section className={styles.family}>
      <header className={styles.familyHead}>
        <h2>{title}</h2>
        <span className={styles.kpi}>{family.value}</span>
      </header>
      {subtitle && <p className={styles.familySubtitle}>{subtitle}</p>}
      {family.trend.vs_previous_year && (
        <p className={styles.trend}>vs anno precedente: {family.trend.vs_previous_year}</p>
      )}
      <div className={styles.exceptions}>
        <h3>Top eccezioni</h3>
        {sortedExceptions.length === 0 ? (
          <p className={styles.allGood}>Tutto in linea</p>
        ) : (
          <ul>
            {sortedExceptions.slice(0, 3).map((exception) => (
              <li key={exception.id} className={styles.exception}>
                <span className={`${styles.dot} ${styles[`severity_${exception.severity}`]}`}>
                  {severitySymbol(exception.severity)}
                </span>
                <Link to={exception.drilldown_url} className={styles.exceptionLink}>
                  {exception.title}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
