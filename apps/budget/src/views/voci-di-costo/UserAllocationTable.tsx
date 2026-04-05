import { useState } from 'react';
import type { UserBudgetAllocation } from '../../api/types';
import { formatMoneyDisplay } from '../../utils/format';
import { AllocationCreateModal } from './AllocationCreateModal';
import { AllocationEditModal } from './AllocationEditModal';
import { ApprovalRulesPanel } from './ApprovalRulesPanel';
import styles from './BudgetDetailPage.module.css';

interface UserAllocationTableProps {
  budgetId: number;
  allocations: UserBudgetAllocation[];
}

export function UserAllocationTable({ budgetId, allocations }: UserAllocationTableProps) {
  const [showCreate, setShowCreate] = useState(false);
  const [editAlloc, setEditAlloc] = useState<UserBudgetAllocation | null>(null);
  const [expandedUserId, setExpandedUserId] = useState<number | null>(null);

  const allocatedUserIds = allocations.map((a) => a.user_id);

  function toggleExpand(userId: number) {
    setExpandedUserId(expandedUserId === userId ? null : userId);
  }

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
        <div>
          <div className={styles.allocHeader}>
            <span />
            <span>Utente</span>
            <span className={styles.allocRight}>Limite</span>
            <span className={styles.allocRight}>Corrente</span>
            <span className={styles.allocCenter}>Attivo</span>
            <span />
          </div>
          {allocations.map((a) => {
            const isExpanded = expandedUserId === a.user_id;
            return (
              <div key={a.user_id}>
                <div
                  className={`${styles.allocRow} ${isExpanded ? styles.allocRowExpanded : ''}`}
                  onClick={() => toggleExpand(a.user_id)}
                >
                  <button
                    className={`${styles.expandBtn} ${isExpanded ? styles.expandBtnOpen : ''}`}
                    onClick={(e) => { e.stopPropagation(); toggleExpand(a.user_id); }}
                    title="Regole di approvazione"
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                      <path d="M5 3l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <span className={styles.allocName}>{a.user_email}</span>
                  <span className={styles.allocMoney}>{formatMoneyDisplay(a.limit)}</span>
                  <span className={styles.allocMoney}>{formatMoneyDisplay(a.current)}</span>
                  <span className={styles.allocCenter}>
                    <span className={`${styles.statusBadge} ${a.enabled ? styles.statusActive : styles.statusInactive}`}>
                      <span className={styles.statusDot} />
                    </span>
                  </span>
                  <button
                    className={styles.allocEditBtn}
                    onClick={(e) => { e.stopPropagation(); setEditAlloc(a); }}
                    title="Modifica"
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                      <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                    </svg>
                  </button>
                </div>
                <div className={`${styles.expandWrapper} ${isExpanded ? styles.expandWrapperOpen : ''}`}>
                  <div className={styles.expandInner}>
                    {isExpanded && (
                      <ApprovalRulesPanel
                        type="user"
                        budgetId={budgetId}
                        userId={a.user_id}
                        userEmail={a.user_email}
                      />
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      <AllocationCreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        type="user"
        budgetId={budgetId}
        excludeUserIds={allocatedUserIds}
      />

      {editAlloc && (
        <AllocationEditModal
          open={!!editAlloc}
          onClose={() => setEditAlloc(null)}
          type="user"
          budgetId={budgetId}
          identifier={editAlloc.user_id}
          currentLimit={editAlloc.limit}
          currentEnabled={editAlloc.enabled}
        />
      )}
    </>
  );
}
