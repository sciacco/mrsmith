import { useCallback, useState } from 'react';
import { Icon, SearchInput, Skeleton, StatusBadge, TableToolbar, useTableFilter, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import { useXConnectOrders } from '../api/queries';
import type { XConnectOrder } from '../types';
import { formatDate } from '../utils/format';
import { getOrderPdfNotReadyMessage } from './orderPdfState';
import shared from './shared.module.css';
import styles from './ReportXConnectRhPage.module.css';

type OrderPdfState =
  | { status: 'downloading' }
  | { status: 'not_ready'; message: string };

function getOrderPdfErrorMessage(error: unknown): string {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body !== null) {
    const body = error.body as { message?: unknown; error?: unknown };
    if (typeof body.message === 'string' && body.message.trim() !== '') return body.message;
    if (typeof body.error === 'string' && body.error.trim() !== '') return body.error;
  }

  return error instanceof Error ? error.message : 'Errore durante il download';
}

export default function ReportXConnectRhPage() {
  const [orderPdfStates, setOrderPdfStates] = useState<Record<number, OrderPdfState>>({});
  const [search, setSearch] = useState('');

  const { toast } = useToast();
  const api = useApiClient();
  const ordersQ = useXConnectOrders();

  const orders = ordersQ.data ?? [];
  const { filtered: filteredOrders } = useTableFilter<XConnectOrder>({
    data: orders,
    searchQuery: search,
    searchFields: ['id_ordine', 'codice_ordine', 'cliente', 'data_creazione'],
  });

  const triggerDownload = (blob: Blob, filename: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  const setOrderPdfState = useCallback((orderId: number, next: OrderPdfState | null) => {
    setOrderPdfStates((current) => {
      if (next == null) {
        if (!(orderId in current)) return current;
        const { [orderId]: _discarded, ...rest } = current;
        return rest;
      }
      return { ...current, [orderId]: next };
    });
  }, []);

  const handleDownloadOrderPDF = useCallback(async (orderId: number) => {
    setOrderPdfState(orderId, { status: 'downloading' });
    try {
      const blob = await api.getBlob(`/afc-tools/v1/orders/${orderId}/pdf`);
      triggerDownload(blob, `order_${orderId}.pdf`);
      setOrderPdfState(orderId, null);
    } catch (e) {
      if (e instanceof ApiError) {
        const notReadyMessage = getOrderPdfNotReadyMessage(e.status, e.body);
        if (notReadyMessage) {
          setOrderPdfState(orderId, { status: 'not_ready', message: notReadyMessage });
          return;
        }
      }

      setOrderPdfState(orderId, null);
      toast(getOrderPdfErrorMessage(e), 'error');
    }
  }, [api, setOrderPdfState, toast]);

  const renderOrderPdfAction = (orderId: number) => {
    const state = orderPdfStates[orderId];
    const loading = state?.status === 'downloading';

    return (
      <div className={styles.orderPdfCell}>
        {state?.status === 'not_ready' && (
          <div className={styles.orderPdfStatus} aria-live="polite">
            <StatusBadge
              value="pdf_not_ready"
              label="PDF non disponibile"
              variant="warning"
            />
            <span className={styles.orderPdfHint}>{state.message}</span>
          </div>
        )}
        <button
          className={`${shared.btnLink} ${styles.orderPdfButton}`}
          onClick={() => handleDownloadOrderPDF(orderId)}
          disabled={loading}
          aria-busy={loading || undefined}
        >
          {loading ? (
            <>
              <Icon name="loader" size={14} className={styles.spin} />
              <span>Verifica…</span>
            </>
          ) : state?.status === 'not_ready' ? (
            <span>Riprova</span>
          ) : (
            <span>Scarica PDF</span>
          )}
        </button>
      </div>
    );
  };

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ordini XConnect</h1>

      <TableToolbar>
        <div className={styles.search}>
          <SearchInput
            value={search}
            onChange={(v) => setSearch(v)}
            placeholder="Cerca per ID, codice, cliente o data…"
          />
        </div>
      </TableToolbar>

      {ordersQ.isLoading && <Skeleton rows={8} />}
      {ordersQ.isError && <div className={shared.error}>Errore nel caricamento degli ordini.</div>}

      {ordersQ.data && filteredOrders.length > 0 && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th className={shared.numCol}>ID ordine</th>
                <th>Codice ordine</th>
                <th>Cliente</th>
                <th>Data creazione</th>
                <th>Azione</th>
              </tr>
            </thead>
            <tbody>
              {filteredOrders.map((o, i) => (
                <tr key={o.id_ordine} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td className={shared.numCol}>{o.id_ordine}</td>
                  <td className={shared.mono}>{o.codice_ordine ?? ''}</td>
                  <td>{o.cliente ?? ''}</td>
                  <td>{formatDate(o.data_creazione)}</td>
                  <td>{renderOrderPdfAction(o.id_ordine)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {ordersQ.data && orders.length === 0 && (
        <div className={shared.empty}>Nessun ordine XConnect evaso.</div>
      )}

      {ordersQ.data && orders.length > 0 && filteredOrders.length === 0 && (
        <div className={shared.empty}>Nessun ordine corrisponde alla ricerca.</div>
      )}
    </div>
  );
}
