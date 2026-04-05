import { useState } from 'react';
import { BudgetAlertSection } from './BudgetAlertSection';
import { UnassignedUsersSection } from './UnassignedUsersSection';
import { useBudgetAlerts, useUnassignedUsers } from './queries';
import styles from './HomePage.module.css';

export function HomePage() {
  const [percentage, setPercentage] = useState<number | null>(80);
  const alerts = useBudgetAlerts(percentage);
  const unassigned = useUnassignedUsers();

  const alertsSettled = !alerts.isLoading && !alerts.isFetching;
  const unassignedSettled = !unassigned.isLoading && !unassigned.isFetching;
  const bothSettled = alertsSettled && unassignedSettled;
  const bothEmpty =
    bothSettled &&
    alerts.data?.length === 0 &&
    unassigned.data?.length === 0;

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <h1 className={styles.pageTitle}>Budget Management</h1>
      </div>

      <BudgetAlertSection
        percentage={percentage}
        onPercentageChange={setPercentage}
      />
      <UnassignedUsersSection />

      {bothEmpty && (
        <div className={styles.allClear}>
          <div className={styles.allClearIcon}>
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <span>Nessun problema rilevato</span>
        </div>
      )}
    </div>
  );
}
