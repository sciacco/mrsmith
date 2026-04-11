import { Icon } from '@mrsmith/ui';
import styles from './Pagination.module.css';

interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, pageSize, total, onPageChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const disablePrev = page <= 1;
  const disableNext = page >= totalPages;

  return (
    <div className={styles.pagination}>
      <span className={styles.info}>
        {total.toLocaleString('it-IT')} {total === 1 ? 'proposta' : 'proposte'}
      </span>
      <div className={styles.controls}>
        <button
          type="button"
          className={styles.btn}
          disabled={disablePrev}
          onClick={() => onPageChange(page - 1)}
          aria-label="Pagina precedente"
        >
          <Icon name="chevron-left" size={16} />
        </button>
        <span className={styles.pageText}>
          Pagina <strong>{page}</strong> di {totalPages}
        </span>
        <button
          type="button"
          className={styles.btn}
          disabled={disableNext}
          onClick={() => onPageChange(page + 1)}
          aria-label="Pagina successiva"
        >
          <Icon name="chevron-right" size={16} />
        </button>
      </div>
    </div>
  );
}
