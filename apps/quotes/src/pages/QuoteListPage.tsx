import { useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useQuotes } from '../api/queries';
import { FilterBar } from '../components/FilterBar';
import { QuoteTable } from '../components/QuoteTable';
import { Pagination } from '../components/Pagination';
import styles from './QuoteListPage.module.css';

export function QuoteListPage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();

  const page = Number(params.get('page') ?? '1');
  const status = params.get('status') ?? '';
  const owner = params.get('owner') ?? '';
  const search = params.get('q') ?? '';
  const dateFrom = params.get('date_from') ?? '';
  const dateTo = params.get('date_to') ?? '';
  const sort = params.get('sort') ?? '';
  const dir = params.get('dir') ?? '';

  const { data, isLoading, isFetching } = useQuotes({
    page, status, owner, q: search,
    date_from: dateFrom, date_to: dateTo,
    sort, dir,
  });

  const hasFilters = !!(status || owner || search || dateFrom || dateTo);

  const handleClearFilters = useCallback(() => {
    setParams({});
  }, [setParams]);

  const handlePageChange = useCallback((newPage: number) => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      next.set('page', String(newPage));
      return next;
    });
  }, [setParams]);

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>Proposte</h1>
        <button className={styles.cta} onClick={() => navigate('/quotes/new')}>
          Nuova proposta
        </button>
      </div>

      <FilterBar />

      <div className={styles.tableWrap}>
        <QuoteTable
          quotes={data?.quotes ?? []}
          isLoading={isLoading}
          isFetching={isFetching && !isLoading}
          hasFilters={hasFilters}
          onClearFilters={handleClearFilters}
        />
      </div>

      {data && data.total > 0 && (
        <Pagination
          page={data.page}
          pageSize={data.page_size}
          total={data.total}
          onPageChange={handlePageChange}
        />
      )}
    </div>
  );
}
