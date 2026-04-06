import { useState, useRef, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useToast, Skeleton } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { getApiErrorMessage, isUpstreamAuthFailed } from '../../api/errors';
import { useBudgetDetails } from './queries';
import { formatMoneyDisplay } from '../../utils/format';
import { BudgetEditModal } from './BudgetEditModal';
import { BudgetDeleteConfirm } from './BudgetDeleteConfirm';
import { UserAllocationTable } from './UserAllocationTable';
import { CcAllocationTable } from './CcAllocationTable';
import styles from './BudgetDetailPage.module.css';

const CURRENT_YEAR = new Date().getFullYear();
const TABS = [
  { key: 'cost-centers', label: 'Centri di costo' },
  { key: 'users', label: 'Utenti' },
] as const;

type TabKey = (typeof TABS)[number]['key'];

export function BudgetDetailPage() {
  const { id } = useParams<{ id: string }>();
  const budgetId = Number(id);
  const navigate = useNavigate();
  const { toast } = useToast();

  const [showEdit, setShowEdit] = useState(false);
  const [showDelete, setShowDelete] = useState(false);
  const [activeTab, setActiveTab] = useState<TabKey>('cost-centers');

  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const [indicatorStyle, setIndicatorStyle] = useState({ left: 0, width: 0 });

  const { data: details, isLoading, error } = useBudgetDetails(
    isNaN(budgetId) ? null : budgetId,
  );

  // Handle 404
  useEffect(() => {
    if (error) {
      if (error instanceof ApiError && error.status === 404) {
        toast('Budget non trovato', 'error');
        navigate('/budgets');
      } else if (isUpstreamAuthFailed(error)) {
        toast('Servizio budget temporaneamente non disponibile', 'error');
      } else {
        toast(getApiErrorMessage(error), 'error');
      }
    }
  }, [error, navigate, toast]);

  // Tab indicator position
  useEffect(() => {
    const el = tabRefs.current[activeTab];
    if (el) {
      setIndicatorStyle({ left: el.offsetLeft, width: el.offsetWidth });
    }
  }, [activeTab, details]);

  if (isNaN(budgetId)) {
    navigate('/budgets');
    return null;
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <Skeleton rows={6} />
      </div>
    );
  }

  if (isUpstreamAuthFailed(error)) {
    return (
      <div className={styles.page}>
        <div className={styles.headerCard}>
          <h1 className={styles.headerTitle}>Servizio temporaneamente non disponibile</h1>
          <p className={styles.moneyLabel}>Il dettaglio budget non e al momento raggiungibile dal backend.</p>
        </div>
      </div>
    );
  }

  if (!details) return null;

  const isActive = details.year === CURRENT_YEAR;
  const userBudgets = details.user_budgets ?? [];
  const costCenterBudgets = details.cost_center_budgets ?? [];

  return (
    <div className={styles.page}>
      {/* Breadcrumb */}
      <nav className={styles.breadcrumb}>
        <Link to="/budgets" className={styles.breadcrumbLink}>Voci di costo</Link>
        <span className={styles.breadcrumbSep}>/</span>
        <span className={styles.breadcrumbCurrent}>{details.name} {details.year}</span>
      </nav>

      {/* Header */}
      <div className={styles.headerCard}>
        <div className={styles.headerTop}>
          <div className={styles.headerInfo}>
            <div className={styles.headerIcon}>
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <rect x="4" y="3" width="16" height="18" rx="2" stroke="currentColor" strokeWidth="1.5" />
                <path d="M8 8h8M8 12h6M8 16h4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </div>
            <div>
              <h1 className={styles.headerTitle}>{details.name} {details.year}</h1>
              <div className={styles.headerMeta}>
                <span className={`${styles.statusBadge} ${isActive ? styles.statusActive : styles.statusInactive}`}>
                  <span className={styles.statusDot} />
                  {isActive ? 'Attivo' : 'Non attivo'}
                </span>
              </div>
            </div>
          </div>
          <div className={styles.headerActions}>
            <button className={styles.btnSecondary} onClick={() => setShowEdit(true)}>
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
              </svg>
              Modifica
            </button>
            <button className={styles.btnDanger} onClick={() => setShowDelete(true)}>
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                <path d="M3 4h8M5.5 4V3a.5.5 0 01.5-.5h2a.5.5 0 01.5.5v1M6 6.5v3M8 6.5v3" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                <path d="M3.5 4l.5 7.5a1 1 0 001 .5h4a1 1 0 001-.5L10.5 4" stroke="currentColor" strokeWidth="1.25" />
              </svg>
              Elimina
            </button>
          </div>
        </div>
        <div className={styles.moneyRow}>
          <div className={styles.moneyItem}>
            <span className={styles.moneyLabel}>Limite</span>
            <span className={styles.moneyValue}>{formatMoneyDisplay(details.limit)}</span>
          </div>
          <div className={styles.moneyItem}>
            <span className={styles.moneyLabel}>Corrente</span>
            <span className={styles.moneyValue}>{formatMoneyDisplay(details.current)}</span>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className={styles.tabsCard}>
        <div className={styles.tabBar}>
          {TABS.map((tab) => (
            <button
              key={tab.key}
              ref={(el) => { tabRefs.current[tab.key] = el; }}
              className={`${styles.tab} ${activeTab === tab.key ? styles.tabActive : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
          <div className={styles.tabIndicator} style={indicatorStyle} />
        </div>
        <div className={styles.tabContent} key={activeTab}>
          {activeTab === 'users' ? (
            <UserAllocationTable budgetId={budgetId} allocations={userBudgets} />
          ) : (
            <CcAllocationTable budgetId={budgetId} allocations={costCenterBudgets} />
          )}
        </div>
      </div>

      {/* Modals */}
      <BudgetEditModal open={showEdit} onClose={() => setShowEdit(false)} budget={details} />
      <BudgetDeleteConfirm
        open={showDelete}
        onClose={() => setShowDelete(false)}
        budgetId={budgetId}
        budgetName={details.name}
      />
    </div>
  );
}
