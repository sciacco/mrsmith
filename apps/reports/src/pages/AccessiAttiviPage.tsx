import { useState, useCallback, useMemo } from 'react';
import { Skeleton, MultiSelect, useToast } from '@mrsmith/ui';
import { useConnectionTypes } from '../api/queries';
import { useApiClient } from '../api/client';
import type { ActiveLineRow } from '../types';
import shared from './shared.module.css';
import styles from './AccessiAttiviPage.module.css';

const defaultStati = ['Attiva'];
const statiOptions = [
  { value: 'Attiva', label: 'Attiva' },
  { value: 'Cessata', label: 'Cessata' },
  { value: 'da attivare', label: 'da attivare' },
  { value: 'in attivazione', label: 'in attivazione' },
  { value: 'KO', label: 'KO' },
];

function formatCurrency(value: number): string {
  return value.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function AccessiAttiviPage() {
  const [connectionTypes, setConnectionTypes] = useState<string[]>([]);
  const [statuses, setStatuses] = useState<string[]>(defaultStati);
  const [previewData, setPreviewData] = useState<ActiveLineRow[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connTypesQ = useConnectionTypes();
  const api = useApiClient();
  const { toast } = useToast();

  const connOptions = (connTypesQ.data ?? []).map((t) => ({ value: t, label: t }));

  const canExecute = statuses.length > 0 && connectionTypes.length > 0;
  const hasPreviewRows = (previewData?.length ?? 0) > 0;

  const handlePreview = useCallback(async () => {
    if (!canExecute) return;
    setLoading(true);
    setError(null);
    try {
      const data = await api.post<ActiveLineRow[]>('/reports/v1/active-lines/preview', {
        connectionTypes,
        statuses,
      });
      setPreviewData(data);
    } catch {
      setError('Errore nel caricamento dei dati.');
      setPreviewData(null);
    } finally {
      setLoading(false);
    }
  }, [api, canExecute, connectionTypes, statuses]);

  const handleExport = useCallback(async () => {
    if (!hasPreviewRows) return;
    setExporting(true);
    try {
      const blob = await api.postBlob('/reports/v1/active-lines/export', {
        connectionTypes,
        statuses,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'report_accessi_attivi.xlsx';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast("Errore durante l'esportazione", 'error');
    } finally {
      setExporting(false);
    }
  }, [api, connectionTypes, hasPreviewRows, statuses, toast]);

  const tipoConnBreakdown = useMemo(() => {
    if (!previewData) return [];
    const counts = new Map<string, number>();
    for (const row of previewData) {
      const key = row.tipo_conn || '(vuoto)';
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([tipo, count]) => ({ tipo, count }));
  }, [previewData]);

  const statoBreakdown = useMemo(() => {
    if (!previewData) return [];
    const counts = new Map<string, number>();
    for (const row of previewData) {
      const key = row.stato || '(vuoto)';
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([stato, count]) => ({ stato, count }));
  }, [previewData]);

  const detailRows = useMemo(() => {
    if (!previewData) return [];
    return previewData.slice(0, 100);
  }, [previewData]);

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Accessi attivi</h1>

      <div className={shared.toolbar}>
        <div className={shared.field} style={{ minWidth: 220 }}>
          <label>Tipo connessione</label>
          <MultiSelect options={connOptions} selected={connectionTypes} onChange={setConnectionTypes} placeholder="Tipi..." />
        </div>
        <div className={shared.field} style={{ minWidth: 200 }}>
          <label>Stato</label>
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
              <div className={styles.metricLabel}>Linee</div>
            </div>
          </div>

          {tipoConnBreakdown.length > 0 && (
            <div className={styles.chipGroup}>
              <div className={styles.chipGroupLabel}>Per tipo connessione</div>
              <div className={styles.chips}>
                {tipoConnBreakdown.map((t) => (
                  <span key={t.tipo} className={styles.chip}>
                    {t.tipo} <span className={styles.chipCount}>{t.count}</span>
                  </span>
                ))}
              </div>
            </div>
          )}

          {statoBreakdown.length > 0 && (
            <div className={styles.chipGroup}>
              <div className={styles.chipGroupLabel}>Per stato</div>
              <div className={styles.chips}>
                {statoBreakdown.map((s) => (
                  <span key={s.stato} className={styles.chip}>
                    {s.stato} <span className={styles.chipCount}>{s.count}</span>
                  </span>
                ))}
              </div>
            </div>
          )}

          <div className={styles.actions}>
            <button className={shared.btnPrimary} onClick={handleExport} disabled={!hasPreviewRows || exporting}>
              {exporting ? 'Esportazione…' : 'Esporta XLSX'}
            </button>
          </div>
        </div>
      )}

      {previewData && previewData.length > 0 && !loading && (
        <>
          <div className={styles.banner}>
            {previewData.length <= 100
              ? `${previewData.length} Righe accessi`
              : `Campione di 100 righe su ${previewData.length} in totale`}
          </div>
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>Cliente</th>
                  <th>Tipo conn.</th>
                  <th>Fornitore</th>
                  <th>Provincia</th>
                  <th>Comune</th>
                  <th>Tipo</th>
                  <th>Profilo</th>
                  <th>Intestatario</th>
                  <th>Ordine</th>
                  <th>Stato</th>
                  <th>Serialnumber</th>
                  <th className={shared.numCol}>Quantita</th>
                  <th className={shared.numCol}>Canone</th>
                </tr>
              </thead>
              <tbody>
                {detailRows.map((row, i) => (
                  <tr key={row.id} style={{ animationDelay: `${Math.min(i * 15, 300)}ms` }}>
                    <td>{row.ragione_sociale}</td>
                    <td>{row.tipo_conn ?? ''}</td>
                    <td>{row.fornitore ?? ''}</td>
                    <td>{row.provincia ?? ''}</td>
                    <td>{row.comune ?? ''}</td>
                    <td>{row.tipo ?? ''}</td>
                    <td>{row.profilo_commerciale ?? ''}</td>
                    <td>{row.intestatario ?? ''}</td>
                    <td className={shared.mono}>{row.ordine ?? ''}</td>
                    <td>{row.stato ?? ''}</td>
                    <td className={shared.mono}>{row.serialnumber ?? ''}</td>
                    <td className={shared.numCol}>{row.quantita ?? ''}</td>
                    <td className={shared.numCol}>{row.canone != null ? formatCurrency(row.canone) : ''}</td>
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
