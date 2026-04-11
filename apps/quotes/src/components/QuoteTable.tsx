import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import type { Quote } from '../api/types';
import { StatusBadge } from './StatusBadge';
import { KebabMenu } from './KebabMenu';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { useDeleteQuote } from '../api/queries';
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
  const deleteQuote = useDeleteQuote();
  const [deletingQuoteId, setDeletingQuoteId] = useState<number | null>(null);

  const handleDelete = (id: number) => {
    if (deleteQuote.isPending) return;
    setDeletingQuoteId(id);
    deleteQuote.mutate(id, {
      onSettled: () => {
        setDeletingQuoteId(current => (current === id ? null : current));
      },
    });
  };

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
      <div className={styles.skeletonWrap}>
        <Skeleton rows={8} />
      </div>
    );
  }

  if (quotes.length === 0) {
    return (
      <div className={styles.empty}>
        <div className={styles.emptyIcon}>
          <Icon name={hasFilters ? 'filter' : 'file-text'} size={32} strokeWidth={1.5} />
        </div>
        <div className={styles.emptyTitle}>
          {hasFilters ? 'Nessun risultato' : 'Nessuna proposta ancora'}
        </div>
        <div className={styles.emptyText}>
          {hasFilters
            ? 'Prova a modificare i filtri o a cancellare la ricerca per vedere altre proposte.'
            : 'Crea la tua prima proposta per iniziare.'}
        </div>
        <div className={styles.emptyAction}>
          {hasFilters ? (
            <Button variant="ghost" onClick={onClearFilters}>
              Cancella filtri
            </Button>
          ) : (
            <Button
              variant="primary"
              leftIcon={<Icon name="plus" size={16} />}
              onClick={() => navigate('/quotes/new')}
            >
              Nuova proposta
            </Button>
          )}
        </div>
      </div>
    );
  }

  const deleteError = deleteQuote.isError
    ? (deleteQuote.error instanceof Error ? deleteQuote.error.message : 'Errore durante la cancellazione')
    : null;

  return (
    <>
      {deleteError && (
        <div className={styles.errorBar} role="alert">
          {deleteError}
          <button
            type="button"
            className={styles.errorDismiss}
            onClick={() => deleteQuote.reset()}
            aria-label="Chiudi"
          >
            &times;
          </button>
        </div>
      )}
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
              <KebabMenu
                quoteId={q.id}
                canDelete={canDelete}
                onDelete={() => handleDelete(q.id)}
                deleteDisabled={deleteQuote.isPending}
                deleteLabel={
                  deleteQuote.isPending
                    ? (deletingQuoteId === q.id ? 'Eliminazione in corso…' : 'Eliminazione in corso')
                    : 'Elimina'
                }
              />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
    </>
  );
}
