import styles from './Compliance.module.css';

interface DomainListProps {
  domains: Array<{ id: number; domain: string }>;
  onEdit: (domain: { id: number; domain: string }) => void;
}

export function DomainList({ domains, onEdit }: DomainListProps) {
  return (
    <div>
      <h3 className={styles.sectionLabel}>Domini</h3>
      {domains.length === 0 ? (
        <p className={styles.muted}>Nessun dominio</p>
      ) : (
        <div className={styles.domainListContainer}>
          {domains.map((d) => (
            <div key={d.id} className={styles.domainRow}>
              <span className={styles.domainText}>{d.domain}</span>
              <button
                className={styles.domainEditBtn}
                onClick={() => onEdit(d)}
                aria-label={`Modifica ${d.domain}`}
              >
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                  <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
