import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Icon, Skeleton, Tooltip, useToast } from '@mrsmith/ui';
import { hasRole } from '@mrsmith/auth-client';
import type { Quote } from '../api/types';
import { StatusBadge } from './StatusBadge';
import { KebabMenu } from './KebabMenu';
import { ConfirmDialog } from './ConfirmDialog';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { useDeleteQuote, useDuplicateQuote } from '../api/queries';
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
  const canDelete = hasRole(user?.roles, 'app_quotes_delete');
  const { toast } = useToast();
  const deleteQuote = useDeleteQuote();
  const duplicateQuote = useDuplicateQuote();
  const [pendingDeleteQuote, setPendingDeleteQuote] = useState<Quote | null>(null);
  const [deletingQuoteId, setDeletingQuoteId] = useState<number | null>(null);
  const [duplicatingQuoteId, setDuplicatingQuoteId] = useState<number | null>(null);

  const requestDelete = (quote: Quote) => {
    if (deleteQuote.isPending) return;
    setPendingDeleteQuote(quote);
  };

  const confirmDelete = () => {
    if (!pendingDeleteQuote || deleteQuote.isPending) return;
    const quote = pendingDeleteQuote;
    setDeletingQuoteId(quote.id);
    deleteQuote.mutate(quote.id, {
      onSuccess: () => {
        toast(`Proposta ${quote.quote_number} eliminata`, 'success');
      },
      onSettled: () => {
        setPendingDeleteQuote(null);
        setDeletingQuoteId(current => (current === quote.id ? null : current));
      },
    });
  };

  const handleDuplicate = (id: number) => {
    if (duplicateQuote.isPending) return;
    duplicateQuote.reset();
    setDuplicatingQuoteId(id);
    duplicateQuote.mutate(id, {
      onSuccess: result => {
        toast(`Proposta ${result.quote_number} duplicata`, 'success');
        navigate(`/quotes/${result.id}`);
      },
      onError: () => {
        toast('Duplicazione non completata. Verifica la proposta e riprova.', 'error');
      },
      onSettled: () => {
        setDuplicatingQuoteId(current => (current === id ? null : current));
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
  const duplicateError = duplicateQuote.isError
    ? 'Duplicazione non completata. Verifica la proposta e riprova.'
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
      {duplicateError && (
        <div className={styles.errorBar} role="alert">
          {duplicateError}
          <button
            type="button"
            className={styles.errorDismiss}
            onClick={() => duplicateQuote.reset()}
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
            <td>
              <div className={`${styles.cell} ${styles.customerCell} ${styles.truncate}`}>
                {q.customer_name ?? '—'}
              </div>
            </td>
            <td>
              {q.deal_name ? (
                <Tooltip
                  content={
                    <div className={styles.dealTooltip} onClick={(e) => e.stopPropagation()}>
                      <div className={styles.dealTooltipHeader}>
                        <span className={styles.dealTooltipNumber}>
                          {q.deal_number || 'Senza Numero'}
                        </span>
                        {q.hs_deal_id && (
                          <a
                            href={`https://app-eu1.hubspot.com/contacts/26622471/record/0-3/${q.hs_deal_id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={styles.dealTooltipLink}
                            onClick={(e) => e.stopPropagation()}
                          >
                            HubSpot ↗
                          </a>
                        )}
                      </div>
                      <div className={styles.dealTooltipTitle}>{q.deal_name}</div>
                      {(q.customer_name || q.owner_name) && (
                        <div className={styles.dealTooltipFooter}>
                          {q.customer_name && (
                            <span className={styles.dealTooltipCustomer}>
                              {q.customer_name}
                            </span>
                          )}
                          {q.owner_name && (
                            <span className={styles.dealTooltipOwner}>
                              Owner: {abbreviateName(q.owner_name)}
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  }
                  placement="top"
                  maxWidth={360}
                >
                  <div className={`${styles.cell} ${styles.dealCell} ${styles.truncate} ${styles.muted}`}>
                    {q.deal_name}
                  </div>
                </Tooltip>
              ) : (
                <div className={`${styles.cell} ${styles.dealCell} ${styles.truncate} ${styles.muted}`}>
                  —
                </div>
              )}
            </td>
            <td><div className={`${styles.cell} ${styles.muted}`}>{abbreviateName(q.owner_name)}</div></td>
            <td><div className={styles.cell}><StatusBadge status={q.status} /></div></td>
            <td className={styles.kebabCell}>
              {duplicateQuote.isPending && duplicatingQuoteId === q.id ? (
                <span className={styles.duplicatePending} role="status" aria-live="polite">
                  Duplicazione in corso…
                </span>
              ) : (
                <KebabMenu
                  quoteId={q.id}
                  canDelete={canDelete}
                  onDelete={() => requestDelete(q)}
                  deleteDisabled={deleteQuote.isPending}
                  deleteLabel={
                    deleteQuote.isPending
                      ? (deletingQuoteId === q.id ? 'Eliminazione in corso…' : 'Eliminazione in corso')
                      : 'Elimina'
                  }
                  onDuplicate={() => handleDuplicate(q.id)}
                  duplicateDisabled={duplicateQuote.isPending}
                  duplicateLabel={
                    duplicateQuote.isPending
                      ? (duplicatingQuoteId === q.id ? 'Duplicazione in corso…' : 'Duplicazione in corso')
                      : 'Duplica'
                  }
                />
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
      <ConfirmDialog
        open={pendingDeleteQuote !== null}
        title="Eliminare la proposta?"
        message={
          pendingDeleteQuote
            ? `La proposta ${pendingDeleteQuote.quote_number} verrà eliminata in modo irreversibile. L'operazione non può essere annullata.`
            : ''
        }
        confirmLabel="Elimina proposta"
        confirmLoading={deleteQuote.isPending}
        onConfirm={confirmDelete}
        onCancel={() => {
          if (!deleteQuote.isPending) setPendingDeleteQuote(null);
        }}
      />
    </>
  );
}
