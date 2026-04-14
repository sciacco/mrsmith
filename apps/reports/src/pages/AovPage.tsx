import { useState, useCallback } from 'react';
import { Skeleton, MultiSelect } from '@mrsmith/ui';
import { useOrderStatuses } from '../api/queries';
import { useApiClient } from '../api/client';
import type { AovPreviewResponse } from '../types';
import shared from './shared.module.css';
import styles from './AovPage.module.css';

type AovTab = 'byType' | 'byCategory' | 'bySales' | 'detail';

export default function AovPage() {
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [statuses, setStatuses] = useState<string[]>([]);
  const [aovData, setAovData] = useState<AovPreviewResponse | null>(null);
  const [activeTab, setActiveTab] = useState<AovTab>('byType');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const statusesQ = useOrderStatuses();
  const api = useApiClient();

  const statiOptions = (statusesQ.data ?? []).map((st) => ({ value: st, label: st }));

  const canExecute = dateFrom !== '' && dateTo !== '' && statuses.length > 0;

  const handleExecute = useCallback(async () => {
    if (!canExecute) return;
    setLoading(true);
    setError(null);
    try {
      const data = await api.post<AovPreviewResponse>('/reports/v1/aov/preview', {
        dateFrom,
        dateTo,
        statuses,
      });
      setAovData(data);
      setActiveTab('byType');
    } catch {
      setError('Errore nel caricamento dei dati.');
      setAovData(null);
    } finally {
      setLoading(false);
    }
  }, [api, canExecute, dateFrom, dateTo, statuses]);

  const totalAov = aovData
    ? aovData.detail.reduce((sum, r) => sum + (r.valore_aov ?? 0), 0)
    : 0;

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>AOV</h1>

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
        <button className={shared.btnPrimary} onClick={handleExecute} disabled={!canExecute}>
          Esegui
        </button>
      </div>

      {loading && <Skeleton rows={8} />}

      {error && <p>{error}</p>}

      {aovData && !loading && (
        <>
          <div className={styles.cards}>
            <div
              className={`${styles.card} ${activeTab === 'detail' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('detail')}
            >
              <div className={styles.cardValue}>
                {totalAov.toLocaleString('it-IT', { style: 'currency', currency: 'EUR' })}
              </div>
              <div className={styles.cardLabel}>AOV Totale</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'byType' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('byType')}
            >
              <div className={styles.cardValue}>{aovData.byType.length}</div>
              <div className={styles.cardLabel}>Per tipo righe</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'byCategory' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('byCategory')}
            >
              <div className={styles.cardValue}>{aovData.byCategory.length}</div>
              <div className={styles.cardLabel}>Per categoria righe</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'bySales' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('bySales')}
            >
              <div className={styles.cardValue}>{aovData.bySales.length}</div>
              <div className={styles.cardLabel}>Per commerciale righe</div>
            </div>
          </div>

          <div className={styles.tabs}>
            {(['byType', 'byCategory', 'bySales', 'detail'] as const).map((tab) => (
              <button
                key={tab}
                type="button"
                className={`${styles.tab} ${activeTab === tab ? styles.tabActive : ''}`}
                onClick={() => setActiveTab(tab)}
              >
                {tab === 'byType' && 'Per tipo'}
                {tab === 'byCategory' && 'Per categoria'}
                {tab === 'bySales' && 'Per commerciale'}
                {tab === 'detail' && 'Dettaglio'}
              </button>
            ))}
          </div>

          {activeTab === 'byType' && <ByTypeTable data={aovData.byType} />}
          {activeTab === 'byCategory' && <ByCategoryTable data={aovData.byCategory} />}
          {activeTab === 'bySales' && <BySalesTable data={aovData.bySales} />}
          {activeTab === 'detail' && <DetailTable data={aovData.detail} />}
        </>
      )}
    </div>
  );
}

function ByTypeTable({ data }: { data: AovPreviewResponse['byType'] }) {
  return (
    <div className={shared.tableWrap}>
      <div className={shared.info}>{data.length} righe</div>
      <table className={shared.table}>
        <thead>
          <tr>
            <th>Anno/Mese</th>
            <th>Tipo ordine</th>
            <th className={shared.numCol}>N. Ordini</th>
            <th className={shared.numCol}>AOV</th>
            <th className={shared.numCol}>Totale MRC</th>
            <th className={shared.numCol}>Totale NRC</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
              <td>{row.anno_mese}</td>
              <td>{row.tipo_ordine}</td>
              <td className={shared.numCol}>{row.numero_ordini}</td>
              <td className={shared.numCol}>{row.valore_aov != null ? row.valore_aov.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_mrc != null ? row.totale_mrc.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_nrc != null ? row.totale_nrc.toFixed(2) : ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ByCategoryTable({ data }: { data: AovPreviewResponse['byCategory'] }) {
  return (
    <div className={shared.tableWrap}>
      <div className={shared.info}>{data.length} righe</div>
      <table className={shared.table}>
        <thead>
          <tr>
            <th>Anno/Mese</th>
            <th>Categoria</th>
            <th className={shared.numCol}>N. Ordini</th>
            <th className={shared.numCol}>AOV</th>
            <th className={shared.numCol}>Totale MRC</th>
            <th className={shared.numCol}>Totale NRC</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
              <td>{row.anno_mese}</td>
              <td>{row.categoria ?? ''}</td>
              <td className={shared.numCol}>{row.numero_ordini}</td>
              <td className={shared.numCol}>{row.valore_aov != null ? row.valore_aov.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_mrc != null ? row.totale_mrc.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_nrc != null ? row.totale_nrc.toFixed(2) : ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BySalesTable({ data }: { data: AovPreviewResponse['bySales'] }) {
  return (
    <div className={shared.tableWrap}>
      <div className={shared.info}>{data.length} righe</div>
      <table className={shared.table}>
        <thead>
          <tr>
            <th>Anno</th>
            <th>Commerciale</th>
            <th>Tipo ordine</th>
            <th className={shared.numCol}>N. Ordini</th>
            <th className={shared.numCol}>AOV</th>
            <th className={shared.numCol}>Totale MRC</th>
            <th className={shared.numCol}>Totale NRC</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
              <td>{row.anno}</td>
              <td>{row.commerciale ?? ''}</td>
              <td>{row.tipo_ordine}</td>
              <td className={shared.numCol}>{row.numero_ordini}</td>
              <td className={shared.numCol}>{row.valore_aov != null ? row.valore_aov.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_mrc != null ? row.totale_mrc.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.totale_nrc != null ? row.totale_nrc.toFixed(2) : ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DetailTable({ data }: { data: AovPreviewResponse['detail'] }) {
  return (
    <div className={shared.tableWrap}>
      <div className={shared.info}>{data.length} righe</div>
      <table className={shared.table}>
        <thead>
          <tr>
            <th>Cliente</th>
            <th>Tipo ordine</th>
            <th>N. Ordine</th>
            <th>Commerciale</th>
            <th>Data documento</th>
            <th>Descrizione</th>
            <th className={shared.numCol}>Quantita</th>
            <th className={shared.numCol}>MRC</th>
            <th className={shared.numCol}>NRC</th>
            <th className={shared.numCol}>AOV</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
              <td>{row.ragione_sociale}</td>
              <td>{row.tipo_ordine}</td>
              <td className={shared.mono}>{row.numero_ordine}</td>
              <td>{row.commerciale ?? ''}</td>
              <td>{row.data_documento?.slice(0, 10) ?? ''}</td>
              <td>{row.descrizione_long ?? ''}</td>
              <td className={shared.numCol}>{row.quantita ?? ''}</td>
              <td className={shared.numCol}>{row.mrc != null ? row.mrc.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.nrc != null ? row.nrc.toFixed(2) : ''}</td>
              <td className={shared.numCol}>{row.valore_aov != null ? row.valore_aov.toFixed(2) : ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
