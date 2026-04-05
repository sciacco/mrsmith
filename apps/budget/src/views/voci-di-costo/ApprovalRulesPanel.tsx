import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useUserApprovalRules, useCcApprovalRules } from './queries';
import { formatMoneyDisplay } from '../../utils/format';
import type { UserBudgetApprovalRule, CcBudgetApprovalRule } from '../../api/types';
import { RuleCreateModal } from './RuleCreateModal';
import { RuleEditModal } from './RuleEditModal';
import { RuleDeleteConfirm } from './RuleDeleteConfirm';
import styles from './BudgetDetailPage.module.css';

type UserProps = { type: 'user'; budgetId: number; userId: number; costCenter?: undefined };
type CcProps = { type: 'cc'; budgetId: number; costCenter: string; userId?: undefined };

type ApprovalRulesPanelProps = UserProps | CcProps;

export function ApprovalRulesPanel(props: ApprovalRulesPanelProps) {
  const { type, budgetId } = props;
  const [showCreate, setShowCreate] = useState(false);
  const [editRule, setEditRule] = useState<UserBudgetApprovalRule | CcBudgetApprovalRule | null>(null);
  const [deleteRuleId, setDeleteRuleId] = useState<number | null>(null);

  const userQuery = useUserApprovalRules(budgetId, props.userId ?? 0, type === 'user');
  const ccQuery = useCcApprovalRules(budgetId, props.costCenter ?? '', type === 'cc');

  const query = type === 'user' ? userQuery : ccQuery;
  const rules = (query.data ?? []) as (UserBudgetApprovalRule | CcBudgetApprovalRule)[];

  if (query.isLoading) {
    return (
      <div className={styles.rulesPanel}>
        <Skeleton rows={2} />
      </div>
    );
  }

  return (
    <div className={styles.rulesPanel}>
      <div className={styles.rulesHeader}>
        <span className={styles.rulesTitle}>Regole di approvazione</span>
        <button className={`${styles.btnPrimary} ${styles.btnSmall}`} onClick={() => setShowCreate(true)}>
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
            <path d="M6 2v8M2 6h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          Regola
        </button>
      </div>

      {rules.length === 0 ? (
        <p className={styles.rulesEmpty}>Nessuna regola definita</p>
      ) : (
        rules.map((rule, i) => (
          <div key={rule.id} className={styles.ruleRow} style={{ animationDelay: `${i * 60}ms` }}>
            <span className={styles.ruleLevel}>Liv. {rule.level}</span>
            <span className={styles.ruleThreshold}>{formatMoneyDisplay(rule.threshold)}</span>
            <span className={styles.ruleApprover}>{rule.approver_email}</span>
            <span className={`${styles.ruleEmailBadge} ${!rule.send_email ? styles.ruleEmailOff : ''}`}>
              {rule.send_email ? 'Email' : '—'}
            </span>
            <button
              className={styles.ruleActionBtn}
              onClick={() => setEditRule(rule)}
              title="Modifica"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M9 1.5l1.5 1.5-6.5 6.5H2.5V8L9 1.5z" stroke="currentColor" strokeWidth="1" strokeLinejoin="round" />
              </svg>
            </button>
            <button
              className={styles.ruleDeleteBtn}
              onClick={() => setDeleteRuleId(rule.id)}
              title="Elimina"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        ))
      )}

      <RuleCreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        type={type}
        budgetId={budgetId}
        userId={props.userId}
        costCenter={props.costCenter}
      />

      {editRule && (
        <RuleEditModal
          open={!!editRule}
          onClose={() => setEditRule(null)}
          type={type}
          budgetId={budgetId}
          userId={props.userId}
          costCenter={props.costCenter}
          rule={editRule}
        />
      )}

      {deleteRuleId != null && (
        <RuleDeleteConfirm
          open={deleteRuleId != null}
          onClose={() => setDeleteRuleId(null)}
          type={type}
          budgetId={budgetId}
          userId={props.userId}
          costCenter={props.costCenter}
          ruleId={deleteRuleId}
        />
      )}
    </div>
  );
}
