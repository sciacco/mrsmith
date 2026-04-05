import { useState } from 'react';
import type { CostCenterBudgetAllocation } from '../../api/types';
import { formatMoneyDisplay } from '../../utils/format';
import { AllocationCreateModal } from './AllocationCreateModal';
import { AllocationEditModal } from './AllocationEditModal';
import { ApprovalRulesPanel } from './ApprovalRulesPanel';
import styles from './BudgetDetailPage.module.css';

interface CcAllocationTableProps {
  budgetId: number;
  allocations: CostCenterBudgetAllocation[];
}

export function CcAllocationTable({ budgetId, allocations }: CcAllocationTableProps) {
  const [showCreate, setShowCreate] = useState(false);
  const [editAlloc, setEditAlloc] = useState<CostCenterBudgetAllocation | null>(null);
  const [expandedCc, setExpandedCc] = useState<string | null>(null);

  return (
    <>
      <div className={styles.allocToolbar}>
        <button className={`${styles.btnPrimary} ${styles.btnSmall}`} onClick={() => setShowCreate(true)}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <path d="M7 3v8M3 7h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          Allocazione
        </button>
      </div>

      {allocations.length === 0 ? (
        <p className={styles.allocEmpty}>Nessuna allocazione</p>
      ) : (
        <>
          <div className={`${styles.allocHeader} ${styles.allocHeaderCc}`}>
            <span>Centro di costo</span>
            <span className={styles.allocRight}>Limite</span>
            <span className={styles.allocRight}>Corrente</span>
            <span className={styles.allocCenter}>Attivo</span>
            <span />
            <span />
          </div>
          {allocations.map((a) => {
            const isExpanded = expandedCc === a.cost_center;
            return (
              <div key={a.cost_center}>
                <div className={`${styles.allocRow} ${styles.allocRowCc}`}>
                  <span className={styles.allocEmail}>{a.cost_center}</span>
                  <span className={styles.allocMoney}>{formatMoneyDisplay(a.limit)}</span>
                  <span className={styles.allocMoney}>{formatMoneyDisplay(a.current)}</span>
                  <span className={styles.allocCenter}>
                    <span className={`${styles.statusBadge} ${a.enabled ? styles.statusActive : styles.statusInactive}`}>
                      <span className={styles.statusDot} />
                    </span>
                  </span>
                  <button
                    className={styles.allocEditBtn}
                    onClick={() => setEditAlloc(a)}
                    title="Modifica"
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                      <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <button
                    className={`${styles.expandBtn} ${isExpanded ? styles.expandBtnOpen : ''}`}
                    onClick={() => setExpandedCc(isExpanded ? null : a.cost_center)}
                    title="Regole di approvazione"
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                      <path d="M5 3l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                </div>
                <div className={`${styles.expandWrapper} ${isExpanded ? styles.expandWrapperOpen : ''}`}>
                  <div className={styles.expandInner}>
                    {isExpanded && (
                      <ApprovalRulesPanel
                        type="cc"
                        budgetId={budgetId}
                        costCenter={a.cost_center}
                      />
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </>
      )}

      <AllocationCreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        type="cc"
        budgetId={budgetId}
      />

      {editAlloc && (
        <AllocationEditModal
          open={!!editAlloc}
          onClose={() => setEditAlloc(null)}
          type="cc"
          budgetId={budgetId}
          identifier={editAlloc.cost_center}
          currentLimit={editAlloc.limit}
          currentEnabled={editAlloc.enabled}
        />
      )}
    </>
  );
}
