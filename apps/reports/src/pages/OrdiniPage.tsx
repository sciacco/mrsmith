import { useState, useCallback, useMemo } from 'react';
import { Skeleton, MultiSelect } from '@mrsmith/ui';
import { useOrderStatuses } from '../api/queries';
import { useApiClient } from '../api/client';
import type { OrderRow } from '../types';
import shared from './shared.module.css';
import styles from './OrdiniPage.module.css';

function defaultDateFrom(): string {
  const d = new Date();
  d.setMonth(d.getMonth() - 1);
  return d.toISOString().slice(0, 10);
}

function defaultDateTo(): string {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  return d.toISOString().slice(0, 10);
}

function formatCurrency(value: number): string {
  return value.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function OrdiniPage() {
  const [dateFrom, setDateFrom] = useState(defaultDateFrom);
  const [dateTo, setDateTo] = useState(defaultDateTo);
  const [statuses, setStatuses] = useState<string[]>([]);
  const [previewData, setPreviewData] = useState<OrderRow[] | null>(null);
  const [showDetail, setShowDetail] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const statusesQ = useOrderStatuses();
  const api = useApiClient();

  const statiOptions = (statusesQ.data ?? []).map((st) => ({ value: st, label: st }));

  const canExecute = dateFrom !== '' && dateTo !== '' && statuses.length > 0;

  const handlePreview = useCallback(async () => {
    if (!canExecute) return;
    setLoading(true);
    setError(null);
    setShowDetail(false);
    try {
      const data = await api.post<OrderRow[]>('/reports/v1/orders/preview', {
        dateFrom,
        dateTo,
        statuses,
      });
      setPreviewData(data);
    } catch {
      setError('Errore nel caricamento dei dati.');
      setPreviewData(null);
    } finally {
      setLoading(false);
    }
  }, [api, canExecute, dateFrom, dateTo, statuses]);

  const handleExport = useCallback(async () => {
    try {
      const blob = await api.postBlob('/reports/v1/orders/export', {
        dateFrom,
        dateTo,
        statuses,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `report_ordini_dal_${dateFrom}_al_${dateTo}.xlsx`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // Export failed silently
    }
  }, [api, dateFrom, dateTo, statuses]);

  const totalMrc = useMemo(
    () => (previewData ?? []).reduce((sum, r) => sum + (r.totale_mrc ?? 0), 0),
    [previewData],
  );

  const totalNrc = useMemo(
    () => (previewData ?? []).reduce((sum, r) => sum + (r.nrc ?? 0), 0),
    [previewData],
  );

  const statusBreakdown = useMemo(() => {
    if (!previewData) return [];
    const counts = new Map<string, number>();
    for (const row of previewData) {
      const key = row.stato_ordine || '(vuoto)';
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([status, count]) => ({ status, count }));
  }, [previewData]);

  const detailRows = useMemo(() => {
    if (!previewData) return [];
    return previewData.slice(0, 100);
  }, [previewData]);

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ordini</h1>

      <div className={shared.toolbar}>
        <div className={`${shared.field} ${styles.field}`}>
          <label>Data da</label>
          <input type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
        </div>
        <div className={`${shared.field} ${styles.field}`}>
          <label>Data a</label>
          <input type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
        </div>
        <div className={shared.field} style={{ minWidth: 200 }}>
          <label>Stati ordine</label>
          <MultiSelect options={statiOptions} selected={statuses} onChange={setStatuses} placeholder="Stati..." />
        </div>
        <button className={shared.btnSecondary} onClick={handlePreview} disabled={!canExecute}>
          Anteprima
        </button>
      </div>

      {loading && <Skeleton rows={8} />}

      {error && <p>{error}</p>}

      {previewData && !loading && (
        <div className={styles.summary}>
          <div className={styles.metrics}>
            <div className={styles.metric}>
              <div className={styles.metricValue}>{previewData.length}</div>
              <div className={styles.metricLabel}>Ordini</div>
            </div>
            <div className={styles.metric}>
              <div className={styles.metricValue}>&euro;{formatCurrency(totalMrc)}</div>
              <div className={styles.metricLabel}>MRC totale</div>
            </div>
            <div className={styles.metric}>
              <div className={styles.metricValue}>&euro;{formatCurrency(totalNrc)}</div>
              <div className={styles.metricLabel}>NRC totale</div>
            </div>
          </div>

          <div className={styles.chips}>
            {statusBreakdown.map((s) => (
              <span key={s.status} className={styles.chip}>
                {s.status} <span className={styles.chipCount}>{s.count}</span>
              </span>
            ))}
          </div>

          <div className={styles.actions}>
            <button className={shared.btnPrimary} onClick={handleExport}>
              Esporta XLSX
            </button>
            <button className={shared.btnLink} onClick={() => setShowDetail((v) => !v)}>
              {showDetail ? 'Nascondi dettaglio' : 'Mostra dettaglio'}
            </button>
          </div>
        </div>
      )}

      {previewData && showDetail && !loading && (
        <>
          <div className={styles.banner}>
            Mostrando {Math.min(100, previewData.length)} di {previewData.length} righe
          </div>
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>Cliente</th>
                  <th>Stato ordine</th>
                  <th>N. Ordine</th>
                  <th>Descrizione</th>
                  <th className={shared.numCol}>Quantita</th>
                  <th className={shared.numCol}>NRC</th>
                  <th className={shared.numCol}>MRC</th>
                  <th className={shared.numCol}>Totale MRC</th>
                  <th>Data documento</th>
                  <th>Stato riga</th>
                  <th>Serial number</th>
                  <th>Metodo pagamento</th>
                  <th>Durata servizio</th>
                  <th>Data attivazione</th>
                  <th>Data cessazione</th>
                </tr>
              </thead>
              <tbody>
                {detailRows.map((row, i) => (
                  <tr key={`${row.numero_ordine}-${row.progressivo_riga ?? i}`} style={{ animationDelay: `${Math.min(i * 15, 300)}ms` }}>
                    <td>{row.ragione_sociale}</td>
                    <td>{row.stato_ordine}</td>
                    <td className={shared.mono}>{row.numero_ordine}</td>
                    <td>{row.descrizione_long ?? ''}</td>
                    <td className={shared.numCol}>{row.quantita ?? ''}</td>
                    <td className={shared.numCol}>{row.nrc != null ? formatCurrency(row.nrc) : ''}</td>
                    <td className={shared.numCol}>{row.mrc != null ? formatCurrency(row.mrc) : ''}</td>
                    <td className={shared.numCol}>{row.totale_mrc != null ? formatCurrency(row.totale_mrc) : ''}</td>
                    <td>{row.data_documento?.slice(0, 10) ?? ''}</td>
                    <td>{row.stato_riga ?? ''}</td>
                    <td className={shared.mono}>{row.serialnumber ?? ''}</td>
                    <td>{row.metodo_pagamento ?? ''}</td>
                    <td>{row.durata_servizio ?? ''}</td>
                    <td>{row.data_attivazione?.slice(0, 10) ?? ''}</td>
                    <td>{row.data_cessazione?.slice(0, 10) ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {!previewData && !loading && !error && (
        <div className={shared.empty}>Seleziona i filtri e premi Anteprima per visualizzare i dati.</div>
      )}
    </div>
  );
}
