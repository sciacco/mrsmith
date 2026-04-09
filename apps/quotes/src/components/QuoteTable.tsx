import { useNavigate, useSearchParams } from 'react-router-dom';
import type { Quote } from '../api/types';
import { StatusBadge } from './StatusBadge';
import { KebabMenu } from './KebabMenu';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import styles from './QuoteTable.module.css';

interface QuoteTableProps {
  quotes: Quote[];
  isLoading: boolean;
  isFetching: boolean;
  hasFilters: boolean;
  onClearFilters: () => void;
}

function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return '—';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

function abbreviateName(name: string | null | undefined): string {
  if (!name) return '—';
  const parts = name.trim().split(/\s+/);
  if (parts.length < 2) return name;
  return `${parts[0]?.[0] ?? ''}.${parts.slice(1).join(' ')}`;
}

export function QuoteTable({ quotes, isLoading, isFetching, hasFilters, onClearFilters }: QuoteTableProps) {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const currentSort = params.get('sort') ?? 'quote_number';
  const currentDir = params.get('dir') ?? 'desc';
  const { user } = useOptionalAuth();
  const canDelete = user?.roles?.includes('app_quotes_delete') ?? false;

  const handleSort = (col: string) => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      if (currentSort === col) {
        next.set('dir', currentDir === 'desc' ? 'asc' : 'desc');
      } else {
        next.set('sort', col);
        next.set('dir', 'desc');
      }
      return next;
    });
  };

  const sortIndicator = (col: string) => {
    if (currentSort !== col) return '';
    return currentDir === 'asc' ? ' \u2191' : ' \u2193';
  };

  if (isLoading) {
    return (
      <table className={styles.table}>
        <thead>
          <tr>
            <th style={{ width: 4 }} />
            <th>Numero</th>
            <th>Data</th>
            <th>Cliente</th>
            <th>Deal</th>
            <th>Owner</th>
            <th>Stato</th>
            <th style={{ width: 48 }} />
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 8 }, (_, i) => (
            <tr key={i} className={styles.skeletonRow}>
              <td />
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 100 }} /></div></td>
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 80 }} /></div></td>
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 140 }} /></div></td>
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 120 }} /></div></td>
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 60 }} /></div></td>
              <td><div className={styles.cell}><span className={styles.skeleton} style={{ width: 80 }} /></div></td>
              <td />
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  if (quotes.length === 0) {
    return (
      <div className={styles.empty}>
        {hasFilters ? (
          <>
            <p className={styles.emptyTitle}>Nessun risultato</p>
            <div className={styles.emptyAction}>
              <button className={styles.clearLink} onClick={onClearFilters}>Cancella filtri</button>
            </div>
          </>
        ) : (
          <>
            <p className={styles.emptyTitle}>Nessuna proposta ancora</p>
            <p>Crea la tua prima proposta per iniziare.</p>
            <div className={styles.emptyAction}>
              <button className={styles.clearLink} onClick={() => navigate('/quotes/new')}>
                Crea la tua prima proposta
              </button>
            </div>
          </>
        )}
      </div>
    );
  }

  return (
    <table className={`${styles.table} ${isFetching ? styles.refetching : ''}`}>
      <thead>
        <tr>
          <th style={{ width: 4 }} />
          <th className={styles.sortable} onClick={() => handleSort('quote_number')}>
            Numero{sortIndicator('quote_number')}
          </th>
          <th className={styles.sortable} onClick={() => handleSort('document_date')}>
            Data{sortIndicator('document_date')}
          </th>
          <th className={styles.sortable} onClick={() => handleSort('customer_name')}>
            Cliente{sortIndicator('customer_name')}
          </th>
          <th>Deal</th>
          <th>Owner</th>
          <th className={styles.sortable} onClick={() => handleSort('status')}>
            Stato{sortIndicator('status')}
          </th>
          <th style={{ width: 48 }} />
        </tr>
      </thead>
      <tbody>
        {quotes.map((q, i) => (
          <tr
            key={q.id}
            className={styles.row}
            style={{ animationDelay: `${i * 30}ms` }}
            onClick={() => navigate(`/quotes/${q.id}`)}
          >
            <td className={styles.accentCell}>
              <div className={styles.accentBar} />
            </td>
            <td><div className={`${styles.cell} ${styles.mono}`}>{q.quote_number}</div></td>
            <td><div className={`${styles.cell} ${styles.muted}`}>{formatDate(q.document_date)}</div></td>
            <td><div className={`${styles.cell} ${styles.truncate}`}>{q.customer_name ?? '—'}</div></td>
            <td><div className={`${styles.cell} ${styles.truncate} ${styles.muted}`}>{q.deal_name ?? '—'}</div></td>
            <td><div className={`${styles.cell} ${styles.muted}`}>{abbreviateName(q.owner_name)}</div></td>
            <td><div className={styles.cell}><StatusBadge status={q.status} /></div></td>
            <td className={styles.kebabCell}>
              <KebabMenu quoteId={q.id} canDelete={canDelete} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
