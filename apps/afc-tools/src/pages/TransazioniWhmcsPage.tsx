import { useCallback, useState } from 'react';
import { Skeleton, useToast } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import { fetchTransactions } from '../api/queries';
import type { TransactionsExportResponse, WhmcsTransaction } from '../types';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';

function defaultFrom(): string {
  const d = new Date();
  d.setDate(d.getDate() - 15);
  return d.toISOString().slice(0, 10);
}

function defaultTo(): string {
  return new Date().toISOString().slice(0, 10);
}

export default function TransazioniWhmcsPage() {
  const [from, setFrom] = useState(defaultFrom);
  const [to, setTo] = useState(defaultTo);
  const [rows, setRows] = useState<WhmcsTransaction[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const api = useApiClient();
  const { toast } = useToast();

  const canExecute = from !== '' && to !== '';

  const handleSearch = useCallback(async () => {
    if (!canExecute) return;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchTransactions(api, from, to);
      setRows(data);
    } catch {
      setError('Errore nel caricamento delle transazioni.');
      setRows(null);
    } finally {
      setLoading(false);
    }
  }, [api, canExecute, from, to]);

  const handleExport = useCallback(async () => {
    if (!canExecute) return;
    setExporting(true);
    try {
      const res = await api.post<TransactionsExportResponse>(
        '/afc-tools/v1/whmcs/transactions/export',
        { from, to },
      );
      window.open(res.renderUrl, '_blank');
    } catch {
      toast('Errore durante l\'esportazione', 'error');
    } finally {
      setExporting(false);
    }
  }, [api, canExecute, from, to, toast]);

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Transazioni WHMCS</h1>

      <div className={shared.toolbar}>
        <div className={shared.field}>
          <label>Data da</label>
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
        </div>
        <div className={shared.field}>
          <label>Data a</label>
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        </div>
        <button className={shared.btnPrimary} onClick={handleSearch} disabled={!canExecute || loading}>
          {loading ? 'Caricamento…' : 'Cerca'}
        </button>
        <button className={shared.btnSecondary} onClick={handleExport} disabled={!canExecute || exporting}>
          {exporting ? 'Esportazione…' : 'Esporta XLSX'}
        </button>
      </div>

      {loading && <Skeleton rows={8} />}
      {error && <div className={shared.error}>{error}</div>}

      {rows && !loading && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Cliente</th>
                <th>Fattura</th>
                <th className={shared.numCol}>Invoice ID</th>
                <th className={shared.numCol}>User ID</th>
                <th>Metodo pagamento</th>
                <th>Data</th>
                <th>Descrizione</th>
                <th className={shared.numCol}>Importo in</th>
                <th className={shared.numCol}>Commissioni</th>
                <th className={shared.numCol}>Importo out</th>
                <th className={shared.numCol}>Cambio</th>
                <th>Transazione</th>
                <th className={shared.numCol}>Refund ID</th>
                <th className={shared.numCol}>Account</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={`${r.transid ?? 'row'}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td>{r.cliente ?? ''}</td>
                  <td>{r.fattura ?? ''}</td>
                  <td className={shared.numCol}>{r.invoiceid ?? ''}</td>
                  <td className={shared.numCol}>{r.userid ?? ''}</td>
                  <td>{r.payment_method ?? ''}</td>
                  <td>{r.date ?? ''}</td>
                  <td>{r.description ?? ''}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.amountin)}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.fees)}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.amountout)}</td>
                  <td className={shared.numCol}>{r.rate ?? ''}</td>
                  <td className={shared.mono}>{r.transid ?? ''}</td>
                  <td className={shared.numCol}>{r.refundid ?? ''}</td>
                  <td className={shared.numCol}>{r.accountsid ?? ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length === 0 && <div className={shared.empty}>Nessuna transazione nel periodo selezionato.</div>}
        </div>
      )}

      {!rows && !loading && !error && (
        <div className={shared.empty}>Imposta un intervallo di date e premi Cerca.</div>
      )}
    </div>
  );
}
