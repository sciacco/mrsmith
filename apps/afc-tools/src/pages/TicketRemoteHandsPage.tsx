import { useCallback, useState } from 'react';
import { useToast } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import shared from './shared.module.css';
import styles from './TicketRemoteHandsPage.module.css';

type Lang = 'it' | 'en';

export default function TicketRemoteHandsPage() {
  const [ticketId, setTicketId] = useState('');
  const [lang, setLang] = useState<Lang>('it');
  const [downloading, setDownloading] = useState(false);

  const { toast } = useToast();
  const api = useApiClient();

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

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ticket Remote Hands</h1>

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
  );
}
