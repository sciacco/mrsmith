import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Skeleton } from '@mrsmith/ui';
import { useBudgets } from './queries';
import { BudgetCreateModal } from './BudgetCreateModal';
import { formatMoneyDisplay } from '../../utils/format';
import styles from './BudgetListPage.module.css';

const CURRENT_YEAR = new Date().getFullYear();

export function BudgetListPage() {
  const [showCreate, setShowCreate] = useState(false);
  const { data: budgets, isLoading } = useBudgets();
  const navigate = useNavigate();

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div>
          <h1 className={styles.pageTitle}>Voci di costo</h1>
          <p className={styles.pageSubtitle}>Gestisci i budget e le allocazioni</p>
        </div>
        <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
          Nuovo budget
        </button>
      </div>

      <div className={styles.tableCard}>
        {isLoading ? (
          <div className={styles.tableBody}>
            <Skeleton rows={5} />
          </div>
        ) : !budgets || budgets.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                <rect x="8" y="6" width="32" height="36" rx="4" stroke="currentColor" strokeWidth="2" />
                <path d="M16 16h16M16 24h12M16 32h8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessun budget trovato</p>
            <p className={styles.emptyText}>Crea il tuo primo budget per iniziare</p>
          </div>
        ) : (
          <>
            <div className={styles.tableHeader}>
              <span>Nome</span>
              <span className={styles.colCenter}>Anno</span>
              <span className={styles.colRight}>Limite</span>
              <span className={styles.colRight}>Corrente</span>
              <span className={styles.colCenter}>Attivo</span>
              <span />
            </div>
            <div className={styles.tableBody}>
              {budgets.map((b, i) => (
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
                  <span className={styles.colCenter}>
                    <span className={`${styles.statusBadge} ${b.year === CURRENT_YEAR ? styles.statusActive : styles.statusInactive}`}>
                      <span className={styles.statusDot} />
                    </span>
                  </span>
                  <svg className={styles.rowChevron} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                    <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      <BudgetCreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={(id) => navigate(`/budgets/${id}`)}
      />
    </div>
  );
}
