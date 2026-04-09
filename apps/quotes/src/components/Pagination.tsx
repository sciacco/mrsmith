import styles from './Pagination.module.css';

interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, pageSize, total, onPageChange }: PaginationProps) {
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={styles.pagination}>
      <span className={styles.info}>
        {total} {total === 1 ? 'proposta' : 'proposte'}
      </span>
      <div className={styles.buttons}>
        <button
          className={styles.btn}
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          Precedente
        </button>
        <button
          className={styles.btn}
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Successivo
        </button>
      </div>
    </div>
  );
}
