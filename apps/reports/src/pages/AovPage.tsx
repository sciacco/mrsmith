import { useState, useCallback } from 'react';
import { Icon, Skeleton, MultiSelect, useToast } from '@mrsmith/ui';
import { useOrderStatuses } from '../api/queries';
import { useApiClient } from '../api/client';
import type { AovPreviewResponse } from '../types';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';
import styles from './AovPage.module.css';

type AovTab = 'byType' | 'byCategory' | 'bySales' | 'detail';

const csvMoneyFormatter = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
  useGrouping: false,
});

const csvIntegerFormatter = new Intl.NumberFormat('it-IT', {
  maximumFractionDigits: 0,
  useGrouping: false,
});

const tabLabels: Record<AovTab, string> = {
  byType: 'Per tipo',
  byCategory: 'Per categoria',
  bySales: 'Per commerciale',
  detail: 'Dettaglio',
};

function formatDateInput(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function defaultDateFrom(): string {
  const now = new Date();
  return `${now.getFullYear()}-01-01`;
}

function defaultDateTo(): string {
  return formatDateInput(new Date());
}

function formatYearMonth(anno: string | null, mese: string | null): string {
  if (anno && mese) return `${anno}/${mese}`;
  if (anno) return anno;
  return mese ?? '';
}

function formatTipoDocumento(tipoDocumento: string | null): string {
  if (!tipoDocumento) return '';
  if (tipoDocumento === 'TSC-ORDINE-RIC') return 'Ordine ricorrente';
  return 'Ordine Spot';
}

function formatCsvMoney(value: number | null | undefined): string {
  if (value == null) return '';
  return csvMoneyFormatter.format(value);
}

function formatCsvInteger(value: number | null | undefined): string {
  if (value == null) return '';
  return csvIntegerFormatter.format(value);
}

function escapeCsvCell(value: string): string {
  const escaped = value.replaceAll('"', '""');
  if (/[;"\n\r]/.test(escaped)) {
    return `"${escaped}"`;
  }
  return escaped;
}

function downloadCsv(filename: string, headers: string[], rows: string[][]): void {
  const csv = [headers, ...rows]
    .map((row) => row.map((cell) => escapeCsvCell(cell)).join(';'))
    .join('\r\n');

  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

function buildAovCsvFilename(
  section: 'per_tipo' | 'per_categoria' | 'per_commerciale' | 'dettaglio',
  dateFrom: string,
  dateTo: string,
): string {
  return `aov_${section}_${dateFrom}_${dateTo}.csv`;
}

function buildByTypeCsvRows(data: AovPreviewResponse['byType']): string[][] {
  return data.map((row) => [
    formatYearMonth(row.anno, row.mese),
    row.tipo_ordine ?? '',
    formatCsvInteger(row.numero_ordini),
    formatCsvMoney(row.valore_aov),
    formatCsvMoney(row.totale_mrc),
    formatCsvMoney(row.totale_nrc),
  ]);
}

function buildByCategoryCsvRows(data: AovPreviewResponse['byCategory']): string[][] {
  return data.map((row) => [
    formatYearMonth(row.anno, row.mese),
    row.categoria ?? '',
    formatCsvInteger(row.numero_ordini),
    formatCsvMoney(row.valore_aov),
    formatCsvMoney(row.totale_mrc),
    formatCsvMoney(row.totale_nrc),
  ]);
}

function buildBySalesCsvRows(data: AovPreviewResponse['bySales']): string[][] {
  return data.map((row) => [
    row.anno ?? '',
    row.commerciale ?? '',
    row.tipo_ordine ?? '',
    formatCsvInteger(row.numero_ordini),
    formatCsvMoney(row.valore_aov),
    formatCsvMoney(row.totale_mrc),
    formatCsvMoney(row.totale_nrc),
  ]);
}

function buildDetailCsvRows(data: AovPreviewResponse['detail']): string[][] {
  return data.map((row) => [
    formatYearMonth(row.anno, row.mese),
    row.nome_testata_ordine ?? '',
    row.tipo_ordine ?? '',
    row.commerciale ?? '',
    formatTipoDocumento(row.tipo_documento),
    row.sost_ord ?? '',
    formatCsvMoney(row.totale_mrc),
    formatCsvMoney(row.totale_mrc_odv_sost),
    formatCsvMoney(row.totale_mrc_new),
    formatCsvMoney(row.totale_nrc),
    formatCsvMoney(row.valore_aov),
  ]);
}

function TableSectionHeader({
  count,
  onExport,
  disabled,
}: {
  count: number;
  onExport: () => void;
  disabled: boolean;
}) {
  return (
    <div className={styles.sectionHeader}>
      <div className={styles.sectionMeta}>
        <div className={styles.sectionCount}>{count} righe</div>
        <button
          type="button"
          className={styles.sectionExport}
          onClick={onExport}
          disabled={disabled}
          aria-label="Esporta CSV"
          title="Esporta CSV"
        >
          <Icon name="download" size={15} />
        </button>
      </div>
    </div>
  );
}

export default function AovPage() {
  const [dateFrom, setDateFrom] = useState(defaultDateFrom);
  const [dateTo, setDateTo] = useState(defaultDateTo);
  const [statuses, setStatuses] = useState<string[]>([]);
  const [aovData, setAovData] = useState<AovPreviewResponse | null>(null);
  const [activeTab, setActiveTab] = useState<AovTab>('byType');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const statusesQ = useOrderStatuses();
  const api = useApiClient();
  const { toast } = useToast();

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

  const exportCsv = useCallback(
    (filename: string, headers: string[], rows: string[][]) => {
      try {
        downloadCsv(filename, headers, rows);
      } catch {
        toast("Errore durante l'esportazione CSV", 'error');
      }
    },
    [toast],
  );

  const handleByTypeExport = useCallback(() => {
    exportCsv(
      buildAovCsvFilename('per_tipo', dateFrom, dateTo),
      ['Anno/Mese', 'Tipo ordine', 'N. Ordini', 'AOV', 'Totale MRC', 'Totale NRC'],
      buildByTypeCsvRows(aovData?.byType ?? []),
    );
  }, [aovData, dateFrom, dateTo, exportCsv]);

  const handleByCategoryExport = useCallback(() => {
    exportCsv(
      buildAovCsvFilename('per_categoria', dateFrom, dateTo),
      ['Anno/Mese', 'Categoria', 'N. Ordini', 'AOV', 'Totale MRC', 'Totale NRC'],
      buildByCategoryCsvRows(aovData?.byCategory ?? []),
    );
  }, [aovData, dateFrom, dateTo, exportCsv]);

  const handleBySalesExport = useCallback(() => {
    exportCsv(
      buildAovCsvFilename('per_commerciale', dateFrom, dateTo),
      ['Anno', 'Commerciale', 'Tipo ordine', 'N. Ordini', 'AOV', 'Totale MRC', 'Totale NRC'],
      buildBySalesCsvRows(aovData?.bySales ?? []),
    );
  }, [aovData, dateFrom, dateTo, exportCsv]);

  const handleDetailExport = useCallback(() => {
    exportCsv(
      buildAovCsvFilename('dettaglio', dateFrom, dateTo),
      [
        'Anno/Mese',
        'Codice ordine',
        'Tipo ordine',
        'Account',
        'Tipo documento',
        'Ordine sostituito',
        'MRC',
        'MRC ordine sostituito',
        'MRC netto',
        'NRC',
        'AOV',
      ],
      buildDetailCsvRows(aovData?.detail ?? []),
    );
  }, [aovData, dateFrom, dateTo, exportCsv]);

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
              <div className={styles.cardValue}>{formatMoneyEUR(totalAov)}</div>
              <div className={styles.cardLabel}>AOV Totale</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'byType' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('byType')}
            >
              <div className={styles.cardValue}>{aovData.byType.length}</div>
              <div className={styles.cardLabel}>Righe per tipo</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'byCategory' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('byCategory')}
            >
              <div className={styles.cardValue}>{aovData.byCategory.length}</div>
              <div className={styles.cardLabel}>Righe per categoria</div>
            </div>
            <div
              className={`${styles.card} ${activeTab === 'bySales' ? styles.cardActive : ''}`}
              onClick={() => setActiveTab('bySales')}
            >
              <div className={styles.cardValue}>{aovData.bySales.length}</div>
              <div className={styles.cardLabel}>Righe per commerciale</div>
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
                {tabLabels[tab]}
              </button>
            ))}
          </div>

          {activeTab === 'byType' && <ByTypeTable data={aovData.byType} onExport={handleByTypeExport} />}
          {activeTab === 'byCategory' && <ByCategoryTable data={aovData.byCategory} onExport={handleByCategoryExport} />}
          {activeTab === 'bySales' && <BySalesTable data={aovData.bySales} onExport={handleBySalesExport} />}
          {activeTab === 'detail' && <DetailTable data={aovData.detail} onExport={handleDetailExport} />}
        </>
      )}
    </div>
  );
}

function ByTypeTable({ data, onExport }: { data: AovPreviewResponse['byType']; onExport: () => void }) {
  let activeGroupKey: string | null = null;
  let subtotalOrders = 0;
  let subtotalAov = 0;
  let subtotalMrc = 0;
  let subtotalNrc = 0;
  let groupRowCount = 0;

  const bodyRows = data.reduce<JSX.Element[]>((acc, row, i) => {
    const currentYearMonth = formatYearMonth(row.anno, row.mese);
    const isNewGroup = activeGroupKey !== currentYearMonth;

    if (isNewGroup) {
      if (activeGroupKey !== null && groupRowCount > 1) {
        acc.push(
          <tr key={`subtotal-${activeGroupKey}-${i}`} className={styles.subtotalRow}>
            <td colSpan={2} />
            <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
          </tr>,
        );
      }
      activeGroupKey = currentYearMonth;
      subtotalOrders = 0;
      subtotalAov = 0;
      subtotalMrc = 0;
      subtotalNrc = 0;
      groupRowCount = 0;
    }

    subtotalOrders += row.numero_ordini;
    subtotalAov += row.valore_aov ?? 0;
    subtotalMrc += row.totale_mrc ?? 0;
    subtotalNrc += row.totale_nrc ?? 0;
    groupRowCount += 1;

    acc.push(
      <tr
        key={i}
        className={isNewGroup ? styles.groupRowHead : undefined}
        style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
      >
        <td className={isNewGroup ? styles.groupValue : styles.groupGap}>
          {isNewGroup ? currentYearMonth : ''}
        </td>
        <td>{row.tipo_ordine ?? ''}</td>
        <td className={shared.numCol}>{row.numero_ordini}</td>
        <td className={shared.numCol}>{row.valore_aov != null ? formatMoneyEUR(row.valore_aov) : ''}</td>
        <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
        <td className={shared.numCol}>{row.totale_nrc != null ? formatMoneyEUR(row.totale_nrc) : ''}</td>
      </tr>,
    );

    if (i === data.length - 1 && activeGroupKey !== null && groupRowCount > 1) {
      acc.push(
        <tr key={`subtotal-${activeGroupKey}-final`} className={styles.subtotalRow}>
          <td colSpan={2} />
          <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
        </tr>,
      );
    }

    return acc;
  }, []);

  return (
    <>
      <TableSectionHeader count={data.length} onExport={onExport} disabled={data.length === 0} />
      <div className={shared.tableWrap}>
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
          <tbody>{bodyRows}</tbody>
        </table>
      </div>
    </>
  );
}

function ByCategoryTable({ data, onExport }: { data: AovPreviewResponse['byCategory']; onExport: () => void }) {
  let activeGroupKey: string | null = null;
  let subtotalOrders = 0;
  let subtotalAov = 0;
  let subtotalMrc = 0;
  let subtotalNrc = 0;
  let groupRowCount = 0;

  const bodyRows = data.reduce<JSX.Element[]>((acc, row, i) => {
    const currentYearMonth = formatYearMonth(row.anno, row.mese);
    const isNewGroup = activeGroupKey !== currentYearMonth;

    if (isNewGroup) {
      if (activeGroupKey !== null && groupRowCount > 1) {
        acc.push(
          <tr key={`subtotal-${activeGroupKey}-${i}`} className={styles.subtotalRow}>
            <td colSpan={2} />
            <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
          </tr>,
        );
      }
      activeGroupKey = currentYearMonth;
      subtotalOrders = 0;
      subtotalAov = 0;
      subtotalMrc = 0;
      subtotalNrc = 0;
      groupRowCount = 0;
    }

    subtotalOrders += row.numero_ordini;
    subtotalAov += row.valore_aov ?? 0;
    subtotalMrc += row.totale_mrc ?? 0;
    subtotalNrc += row.totale_nrc ?? 0;
    groupRowCount += 1;

    acc.push(
      <tr
        key={i}
        className={isNewGroup ? styles.groupRowHead : undefined}
        style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
      >
        <td className={isNewGroup ? styles.groupValue : styles.groupGap}>
          {isNewGroup ? currentYearMonth : ''}
        </td>
        <td>{row.categoria ?? ''}</td>
        <td className={shared.numCol}>{row.numero_ordini}</td>
        <td className={shared.numCol}>{row.valore_aov != null ? formatMoneyEUR(row.valore_aov) : ''}</td>
        <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
        <td className={shared.numCol}>{row.totale_nrc != null ? formatMoneyEUR(row.totale_nrc) : ''}</td>
      </tr>,
    );

    if (i === data.length - 1 && activeGroupKey !== null && groupRowCount > 1) {
      acc.push(
        <tr key={`subtotal-${activeGroupKey}-final`} className={styles.subtotalRow}>
          <td colSpan={2} />
          <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
        </tr>,
      );
    }

    return acc;
  }, []);

  return (
    <>
      <TableSectionHeader count={data.length} onExport={onExport} disabled={data.length === 0} />
      <div className={shared.tableWrap}>
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
          <tbody>{bodyRows}</tbody>
        </table>
      </div>
    </>
  );
}

function BySalesTable({ data, onExport }: { data: AovPreviewResponse['bySales']; onExport: () => void }) {
  let activeGroupKey: string | null = null;
  let subtotalOrders = 0;
  let subtotalAov = 0;
  let subtotalMrc = 0;
  let subtotalNrc = 0;
  let groupRowCount = 0;

  const bodyRows = data.reduce<JSX.Element[]>((acc, row, i) => {
    const groupKey = `${row.anno ?? ''}::${row.commerciale ?? ''}`;
    const isNewGroup = activeGroupKey !== groupKey;
    const prev = i > 0 ? data[i - 1] : undefined;
    const showAnno = i === 0 || row.anno !== prev?.anno;

    if (isNewGroup) {
      if (activeGroupKey !== null && groupRowCount > 1) {
        acc.push(
          <tr key={`subtotal-${activeGroupKey}-${i}`} className={styles.subtotalRow}>
            <td colSpan={3} />
            <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
            <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
          </tr>,
        );
      }
      activeGroupKey = groupKey;
      subtotalOrders = 0;
      subtotalAov = 0;
      subtotalMrc = 0;
      subtotalNrc = 0;
      groupRowCount = 0;
    }

    subtotalOrders += row.numero_ordini;
    subtotalAov += row.valore_aov ?? 0;
    subtotalMrc += row.totale_mrc ?? 0;
    subtotalNrc += row.totale_nrc ?? 0;
    groupRowCount += 1;

    acc.push(
      <tr
        key={i}
        className={isNewGroup ? styles.groupRowHead : undefined}
        style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
      >
        <td className={showAnno ? styles.groupValue : styles.groupGap}>
          {showAnno ? row.anno ?? '' : ''}
        </td>
        <td className={isNewGroup ? styles.groupValue : styles.groupGap}>
          {isNewGroup ? row.commerciale ?? '' : ''}
        </td>
        <td>{row.tipo_ordine ?? ''}</td>
        <td className={shared.numCol}>{row.numero_ordini}</td>
        <td className={shared.numCol}>{row.valore_aov != null ? formatMoneyEUR(row.valore_aov) : ''}</td>
        <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
        <td className={shared.numCol}>{row.totale_nrc != null ? formatMoneyEUR(row.totale_nrc) : ''}</td>
      </tr>,
    );

    if (i === data.length - 1 && activeGroupKey !== null && groupRowCount > 1) {
      acc.push(
        <tr key={`subtotal-${activeGroupKey}-final`} className={styles.subtotalRow}>
          <td colSpan={3} />
          <td className={shared.numCol}>{subtotalOrders.toLocaleString('it-IT')}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalAov)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalMrc)}</td>
          <td className={shared.numCol}>{formatMoneyEUR(subtotalNrc)}</td>
        </tr>,
      );
    }

    return acc;
  }, []);

  return (
    <>
      <TableSectionHeader count={data.length} onExport={onExport} disabled={data.length === 0} />
      <div className={shared.tableWrap}>
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
          <tbody>{bodyRows}</tbody>
        </table>
      </div>
    </>
  );
}

function DetailTable({ data, onExport }: { data: AovPreviewResponse['detail']; onExport: () => void }) {
  return (
    <>
      <TableSectionHeader count={data.length} onExport={onExport} disabled={data.length === 0} />
      <div className={shared.tableWrap}>
        <table className={shared.table}>
          <thead>
            <tr>
              <th>Anno/Mese</th>
              <th>Codice ordine</th>
              <th>Tipo ordine</th>
              <th>Account</th>
              <th>Tipo documento</th>
              <th>Ordine sostituito</th>
              <th className={shared.numCol}>MRC</th>
              <th className={shared.numCol}>MRC ordine sostituito</th>
              <th className={shared.numCol}>MRC netto</th>
              <th className={shared.numCol}>NRC</th>
              <th className={shared.numCol}>AOV</th>
            </tr>
          </thead>
          <tbody>
            {data.map((row, i) => (
              <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
                <td>{formatYearMonth(row.anno, row.mese)}</td>
                <td className={shared.mono}>{row.nome_testata_ordine ?? ''}</td>
                <td>{row.tipo_ordine ?? ''}</td>
                <td>{row.commerciale ?? ''}</td>
                <td>{formatTipoDocumento(row.tipo_documento)}</td>
                <td className={shared.mono}>{row.sost_ord ?? ''}</td>
                <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
                <td className={shared.numCol}>{row.totale_mrc_odv_sost != null ? formatMoneyEUR(row.totale_mrc_odv_sost) : ''}</td>
                <td className={shared.numCol}>{row.totale_mrc_new != null ? formatMoneyEUR(row.totale_mrc_new) : ''}</td>
                <td className={shared.numCol}>{row.totale_nrc != null ? formatMoneyEUR(row.totale_nrc) : ''}</td>
                <td className={shared.numCol}>{row.valore_aov != null ? formatMoneyEUR(row.valore_aov) : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
