import { Skeleton } from '@mrsmith/ui';
import { useUnassignedUsers } from './queries';
import styles from './HomePage.module.css';

export function UnassignedUsersSection() {
  const { data, isLoading, isFetching } = useUnassignedUsers();

  const hasData = data && data.length > 0;

  return (
    <div className={styles.tableCard}>
      <div className={styles.sectionHeaderSimple}>
        <h2 className={styles.sectionTitle}>
          Utenti non assegnati a nessun Budget
        </h2>
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
            <div className={styles.usersTableHeader}>
              <span>Nome</span>
              <span>Email</span>
              <span>Stato</span>
            </div>
            <div className={styles.tableBody}>
              {data.map((u, i) => (
                <div
                  key={u.id}
                  className={styles.userRow}
                  style={{ animationDelay: `${i * 30}ms` }}
                >
                  <span className={styles.rowName}>
                    {u.first_name} {u.last_name}
                  </span>
                  <span className={styles.rowEmail}>{u.email}</span>
                  <span className={styles.rowStatus}>{u.state.name}</span>
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className={styles.sectionEmpty}>
            <p className={styles.emptyText}>Tutti gli utenti sono assegnati a un budget</p>
          </div>
        )}
      </div>
    </div>
  );
}
