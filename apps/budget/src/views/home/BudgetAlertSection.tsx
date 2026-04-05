import { useNavigate } from 'react-router-dom';
import { Skeleton } from '@mrsmith/ui';
import { useBudgetAlerts } from './queries';
import { ThresholdInput } from './ThresholdInput';
import { formatMoneyDisplay } from '../../utils/format';
import styles from './HomePage.module.css';

interface BudgetAlertSectionProps {
  percentage: number | null;
  onPercentageChange: (value: number | null) => void;
}

export function BudgetAlertSection({ percentage, onPercentageChange }: BudgetAlertSectionProps) {
  const { data, isLoading, isFetching } = useBudgetAlerts(percentage);
  const navigate = useNavigate();

  const hasData = data && data.length > 0;

  return (
    <div className={styles.tableCard}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Budget oltre il</h2>
        <ThresholdInput onChange={onPercentageChange} />
      </div>
      <div
        className={styles.sectionContent}
        style={{ opacity: isFetching && !isLoading ? 0.6 : 1 }}
      >
        {isLoading ? (
          <div className={styles.tableBody}>
            <Skeleton rows={3} />
          </div>
        ) : hasData ? (
          <>
            <div className={styles.alertTableHeader}>
              <span>Nome</span>
              <span className={styles.colCenter}>Anno</span>
              <span className={styles.colRight}>Limite</span>
              <span className={styles.colRight}>Corrente</span>
              <span />
            </div>
            <div className={styles.tableBody}>
              {data.map((b, i) => (
                <div
                  key={b.id}
                  className={styles.row}
                  onClick={() => navigate(`/budgets/${b.id}`)}
                  style={{ animationDelay: `${i * 50}ms` }}
                >
                  <div className={styles.rowAccent} />
                  <div className={styles.rowIcon}>
                    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                      <rect x="3" y="2" width="12" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" />
                      <path d="M6 6h6M6 9h5M6 12h3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                    </svg>
                  </div>
                  <span className={styles.rowName}>{b.name}</span>
                  <span className={styles.rowYear}>{b.year}</span>
                  <span className={styles.rowMoney}>{formatMoneyDisplay(b.limit)}</span>
                  <span className={styles.rowMoney}>{formatMoneyDisplay(b.current)}</span>
                  <svg className={styles.rowChevron} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                    <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className={styles.sectionEmpty}>
            <p className={styles.emptyText}>Nessun budget supera la soglia indicata</p>
          </div>
        )}
      </div>
    </div>
  );
}
