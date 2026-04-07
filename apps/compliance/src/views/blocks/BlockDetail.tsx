import { Skeleton } from '@mrsmith/ui';
import type { BlockRequest, BlockDomain } from '../../api/types';
import { DomainList } from '../../components/DomainList';
import styles from './BlocksPage.module.css';

interface BlockDetailProps {
  block: BlockRequest;
  domains: BlockDomain[];
  domainsLoading: boolean;
  onEdit: () => void;
  onAddDomains: () => void;
  onEditDomain: (d: { id: number; domain: string }) => void;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
}

export function BlockDetail({ block, domains, domainsLoading, onEdit, onAddDomains, onEditDomain }: BlockDetailProps) {
  return (
    <div className={styles.detailContent}>
      <div className={styles.detailHeader}>
        <div className={styles.detailIconLg}>
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect x="3" y="3" width="18" height="18" rx="3" stroke="currentColor" strokeWidth="1.5" />
            <path d="M8 8l8 8M16 8l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
        </div>
        <div>
          <h2 className={styles.detailTitle}>{block.reference}</h2>
          <p className={styles.detailMeta}>{formatDate(block.request_date)}</p>
        </div>
      </div>

      <div className={styles.divider} />

      <div className={styles.infoSection}>
        <div className={styles.infoRow}>
          <div className={styles.infoIcon}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 1l6 3v4c0 3.3-2.7 5-6 7-3.3-2-6-3.7-6-7V4l6-3z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
            </svg>
          </div>
          <div>
            <p className={styles.infoLabel}>Provenienza</p>
            <p className={styles.infoValue}>{block.method_description}</p>
          </div>
        </div>
        <div className={styles.infoRow}>
          <div className={styles.infoIcon}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M2 3h12M2 7h8M2 11h10" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
            </svg>
          </div>
          <div>
            <p className={styles.infoLabel}>Riferimento</p>
            <p className={styles.infoValue}>{block.reference}</p>
          </div>
        </div>
        <div className={styles.infoRow}>
          <div className={styles.infoIcon}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <rect x="2" y="2" width="12" height="12" rx="2" stroke="currentColor" strokeWidth="1.25" />
              <path d="M2 6h12" stroke="currentColor" strokeWidth="1.25" />
            </svg>
          </div>
          <div>
            <p className={styles.infoLabel}>Data</p>
            <p className={styles.infoValue}>{formatDate(block.request_date)}</p>
          </div>
        </div>
      </div>

      <div className={styles.divider} />

      {domainsLoading ? (
        <Skeleton rows={3} />
      ) : (
        <DomainList domains={domains} onEdit={onEditDomain} />
      )}

      <div className={styles.divider} />

      <div className={styles.detailActions}>
        <button className={styles.btnSecondary} onClick={onEdit}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
          </svg>
          Modifica
        </button>
        <button className={styles.btnPrimary} onClick={onAddDomains}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <path d="M7 3v8M3 7h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          Aggiungi domini
        </button>
      </div>
    </div>
  );
}
