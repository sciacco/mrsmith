import { useCallback, useState } from 'react';
import { Skeleton, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import { useXConnectOrders } from '../api/queries';
import { formatDate } from '../utils/format';
import shared from './shared.module.css';
import styles from './ReportXConnectRhPage.module.css';

type Tab = 'ticket' | 'orders';
type Lang = 'it' | 'en';

export default function ReportXConnectRhPage() {
  const [tab, setTab] = useState<Tab>('ticket');
  const [ticketId, setTicketId] = useState('');
  const [lang, setLang] = useState<Lang>('it');
  const [downloading, setDownloading] = useState(false);

  const { toast } = useToast();
  const api = useApiClient();
  const ordersQ = useXConnectOrders();

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

  const handleDownloadTicket = useCallback(async () => {
    const id = ticketId.trim();
    if (!id) {
      toast('Inserisci un numero di ticket', 'warning');
      return;
    }
    setDownloading(true);
    try {
      const blob = await api.getBlob(
        `/afc-tools/v1/tickets/${encodeURIComponent(id)}/pdf?lang=${lang}`,
      );
      triggerDownload(blob, `ticket_${id}.pdf`);
      toast('Download avviato', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Errore durante il download', 'error');
    } finally {
      setDownloading(false);
    }
  }, [api, lang, ticketId, toast]);

  const handleDownloadOrderPDF = useCallback(async (orderId: number) => {
    try {
      const blob = await api.getBlob(`/afc-tools/v1/orders/${orderId}/pdf`);
      triggerDownload(blob, `order_${orderId}.pdf`);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        const body = e.body as { error?: string } | null;
        toast(body?.error ?? 'Il PDF non è ancora pronto.', 'warning');
        return;
      }
      toast(e instanceof Error ? e.message : 'Errore durante il download', 'error');
    }
  }, [api, toast]);

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>XConnect &amp; Remote Hands</h1>

      <div className={styles.tabs} role="tablist">
        <button
          role="tab"
          aria-selected={tab === 'ticket'}
          className={`${styles.tab} ${tab === 'ticket' ? styles.tabActive : ''}`}
          onClick={() => setTab('ticket')}
        >
          Ticket Remote Hands
        </button>
        <button
          role="tab"
          aria-selected={tab === 'orders'}
          className={`${styles.tab} ${tab === 'orders' ? styles.tabActive : ''}`}
          onClick={() => setTab('orders')}
        >
          Ordini XConnect
        </button>
      </div>

      {tab === 'ticket' && (
        <div>
          <div className={styles.instructions}>
            Inserisci il numero del ticket Remote Hands e seleziona la lingua per scaricarne il PDF.
          </div>
          <div className={shared.toolbar}>
            <div className={shared.field}>
              <label>Numero ticket</label>
              <input
                type="text"
                value={ticketId}
                onChange={(e) => setTicketId(e.target.value)}
                placeholder="Es. 12345"
              />
            </div>
            <div className={shared.field}>
              <label>Lingua</label>
              <select value={lang} onChange={(e) => setLang(e.target.value as Lang)}>
                <option value="it">Italiano</option>
                <option value="en">English</option>
              </select>
            </div>
            <button
              className={shared.btnPrimary}
              onClick={handleDownloadTicket}
              disabled={downloading}
            >
              {downloading ? 'Download…' : 'Scarica PDF'}
            </button>
          </div>
        </div>
      )}

      {tab === 'orders' && (
        <div>
          {ordersQ.isLoading && <Skeleton rows={8} />}
          {ordersQ.isError && <div className={shared.error}>Errore nel caricamento degli ordini.</div>}
          {ordersQ.data && (
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
                  {ordersQ.data.map((o, i) => (
                    <tr key={o.id_ordine} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                      <td className={shared.numCol}>{o.id_ordine}</td>
                      <td className={shared.mono}>{o.codice_ordine ?? ''}</td>
                      <td>{o.cliente ?? ''}</td>
                      <td>{formatDate(o.data_creazione)}</td>
                      <td>
                        <button
                          className={shared.btnLink}
                          onClick={() => handleDownloadOrderPDF(o.id_ordine)}
                        >
                          Scarica PDF
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {ordersQ.data.length === 0 && <div className={shared.empty}>Nessun ordine XConnect evaso.</div>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
