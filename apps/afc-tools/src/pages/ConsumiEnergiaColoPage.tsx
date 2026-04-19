import { useMemo, useState } from 'react';
import {
  Button,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
  TabNav,
  Tooltip,
  useTableFilter,
} from '@mrsmith/ui';
import { useEnergiaColoDetail, useEnergiaColoPivot } from '../api/queries';
import type { EnergiaColoDetailRow, EnergiaColoPivotRow } from '../types';
import { downloadCsv } from '../utils/csv';
import { formatDate, formatNumber } from '../utils/format';
import shared from './shared.module.css';
import styles from './ConsumiEnergiaColoPage.module.css';

const YEAR_WINDOW = 7;

type YearOption = { value: string; label: string };

function buildYearOptions(): [YearOption, ...YearOption[]] {
  const now = new Date().getFullYear();
  const head: YearOption = { value: String(now), label: String(now) };
  const tail: YearOption[] = [];
  for (let y = now - 1; y >= now - YEAR_WINDOW; y--) {
    tail.push({ value: String(y), label: String(y) });
  }
  return [head, ...tail];
}

const DETAIL_TOOLTIPS: Partial<Record<keyof EnergiaColoDetailRow, string>> = {
  pun: 'Prezzo Unico Nazionale dell’energia nel periodo, in €/MWh.',
  coefficiente: 'Fattore di conversione applicato al consumo.',
  fisso_cu: 'Corrispettivo unitario fisso del contratto, in €.',
  eccedenti: 'Consumo oltre la soglia contrattuale.',
  importo_eccedenti: 'Importo addebitato sugli eccedenti, in €.',
  tipo_variabile: 'Regime di variabilità del corrispettivo applicato.',
};

type MonthKey =
  | 'gennaio'
  | 'febbraio'
  | 'marzo'
  | 'aprile'
  | 'maggio'
  | 'giugno'
  | 'luglio'
  | 'agosto'
  | 'settembre'
  | 'ottobre'
  | 'novembre'
  | 'dicembre';

const MONTHS: { key: MonthKey; label: string }[] = [
  { key: 'gennaio', label: 'Gennaio' },
  { key: 'febbraio', label: 'Febbraio' },
  { key: 'marzo', label: 'Marzo' },
  { key: 'aprile', label: 'Aprile' },
  { key: 'maggio', label: 'Maggio' },
  { key: 'giugno', label: 'Giugno' },
  { key: 'luglio', label: 'Luglio' },
  { key: 'agosto', label: 'Agosto' },
  { key: 'settembre', label: 'Settembre' },
  { key: 'ottobre', label: 'Ottobre' },
  { key: 'novembre', label: 'Novembre' },
  { key: 'dicembre', label: 'Dicembre' },
];

function monthValues(row: EnergiaColoPivotRow, key: MonthKey): { a: number | null; kw: number | null } {
  return {
    a: row[`${key}_a` as keyof EnergiaColoPivotRow] as number | null,
    kw: row[`${key}_kw` as keyof EnergiaColoPivotRow] as number | null,
  };
}

function isPresent(v: number | null | undefined): v is number {
  return v != null && v !== 0;
}

function consumoUnit(tipoVariabile: string | null | undefined): 'A' | 'kW' {
  return tipoVariabile === '2' ? 'kW' : 'A';
}

function HeaderInfo({ label, hint }: { label: string; hint: string }) {
  return (
    <span className={styles.headerWithInfo}>
      {label}
      <Tooltip content={hint} placement="top">
        <span className={styles.infoIcon} aria-label={hint}>
          <Icon name="info" size={12} />
        </span>
      </Tooltip>
    </span>
  );
}

function Unit({ children }: { children: string }) {
  return <span className={styles.unit}>({children})</span>;
}

function MonthCell({ a, kw }: { a: number | null; kw: number | null }) {
  const hasA = isPresent(a);
  const hasKw = isPresent(kw);
  if (!hasA && !hasKw) {
    return <span className={styles.dash}>—</span>;
  }
  return (
    <div className={styles.monthStack}>
      {hasA && (
        <span className={styles.monthValue}>
          {formatNumber(a)}
          <span className={styles.monthUnit}>A</span>
        </span>
      )}
      {hasKw && (
        <span className={`${styles.monthValue} ${styles.monthValueMuted}`}>
          {formatNumber(kw)}
          <span className={styles.monthUnit}>kW</span>
        </span>
      )}
    </div>
  );
}

type TabKey = 'riepilogo' | 'dettaglio';

const TABS = [
  { key: 'riepilogo', label: 'Riepilogo mensile' },
  { key: 'dettaglio', label: 'Dettaglio per periodo' },
];

type DetailSortKey = 'cliente' | 'periodo';
type SortDir = 'asc' | 'desc';

function compareStr(a: string | null | undefined, b: string | null | undefined): number {
  return (a ?? '').localeCompare(b ?? '');
}

function compareDate(a: string | null | undefined, b: string | null | undefined): number {
  const at = a ? new Date(a).getTime() : NaN;
  const bt = b ? new Date(b).getTime() : NaN;
  const aNull = Number.isNaN(at);
  const bNull = Number.isNaN(bt);
  if (aNull && bNull) return 0;
  if (aNull) return 1;
  if (bNull) return -1;
  return at - bt;
}

export default function ConsumiEnergiaColoPage() {
  const yearOptions = useMemo(() => buildYearOptions(), []);
  const [year, setYear] = useState(yearOptions[0].value);
  const [activeTab, setActiveTab] = useState<TabKey>('riepilogo');
  const [pivotSearch, setPivotSearch] = useState('');
  const [detailSearch, setDetailSearch] = useState('');
  const [detailSortKey, setDetailSortKey] = useState<DetailSortKey>('cliente');
  const [detailSortDir, setDetailSortDir] = useState<SortDir>('asc');

  const pivotQ = useEnergiaColoPivot(year, true);
  const detailQ = useEnergiaColoDetail(year, true);

  const pivotData = pivotQ.data ?? [];
  const detailData = detailQ.data ?? [];

  const { filtered: pivotFiltered } = useTableFilter<EnergiaColoPivotRow>({
    data: pivotData,
    searchQuery: pivotSearch,
    searchFields: ['customer'],
  });

  const { filtered: detailFiltered } = useTableFilter<EnergiaColoDetailRow>({
    data: detailData,
    searchQuery: detailSearch,
    searchFields: ['customer'],
  });

  const detailSorted = useMemo(() => {
    const arr = [...detailFiltered];
    const dir = detailSortDir === 'asc' ? 1 : -1;
    arr.sort((a, b) => {
      if (detailSortKey === 'cliente') {
        const primary = compareStr(a.customer, b.customer) * dir;
        if (primary !== 0) return primary;
        return compareDate(a.start_period, b.start_period);
      }
      const primary = compareDate(a.start_period, b.start_period) * dir;
      if (primary !== 0) return primary;
      return compareStr(a.customer, b.customer);
    });
    return arr;
  }, [detailFiltered, detailSortKey, detailSortDir]);

  function toggleDetailSort(key: DetailSortKey) {
    if (detailSortKey === key) {
      setDetailSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setDetailSortKey(key);
      setDetailSortDir('asc');
    }
  }

  function ariaDetailSort(key: DetailSortKey): 'ascending' | 'descending' | 'none' {
    if (detailSortKey !== key) return 'none';
    return detailSortDir === 'asc' ? 'ascending' : 'descending';
  }

  function detailSortHeader(key: DetailSortKey, label: string) {
    const active = detailSortKey === key;
    const arrow = active ? (detailSortDir === 'asc' ? '▲' : '▼') : '↕';
    return (
      <button
        type="button"
        className={`${styles.sortBtn} ${active ? styles.sortBtnActive : ''}`}
        onClick={() => toggleDetailSort(key)}
      >
        {label}
        <span className={styles.sortArrow}>{arrow}</span>
      </button>
    );
  }

  const pivotHasFilter = pivotSearch.trim() !== '';
  const detailHasFilter = detailSearch.trim() !== '';

  function handleExportPivot() {
    const headers = ['Cliente'];
    for (const m of MONTHS) {
      headers.push(`${m.label} A`, `${m.label} kW`);
    }
    const rows = pivotFiltered.map((p) => {
      const row: (string | number | null)[] = [p.customer ?? ''];
      for (const m of MONTHS) {
        const { a, kw } = monthValues(p, m.key);
        row.push(a, kw);
      }
      return row;
    });
    downloadCsv(`consumi-energia-colo_riepilogo_${year}.csv`, headers, rows);
  }

  function handleExportDetail() {
    const headers = [
      'Cliente',
      'Inizio periodo',
      'Fine periodo',
      'Consumo',
      'Unità',
      'Importo (€)',
      'PUN (€/MWh)',
      'Coefficiente',
      'Fisso CU (€)',
      'Eccedenti',
      'Importo eccedenti (€)',
      'Tipo variabile',
    ];
    const rows = detailSorted.map((d) => [
      d.customer ?? '',
      d.start_period ?? '',
      d.end_period ?? '',
      d.consumo,
      d.consumo == null ? '' : consumoUnit(d.tipo_variabile),
      d.amount,
      d.pun,
      d.coefficiente,
      d.fisso_cu,
      d.eccedenti,
      d.importo_eccedenti,
      d.tipo_variabile ?? '',
    ]);
    downloadCsv(`consumi-energia-colo_dettaglio_${year}.csv`, headers, rows);
  }

  return (
    <div className={shared.page}>
      <div className={styles.pageHead}>
        <h1 className={shared.title}>Consumi energia colocation</h1>
        <div className={styles.yearSelect}>
          <SingleSelect
            options={yearOptions}
            selected={year}
            onChange={(v) => setYear((v as string | null) ?? yearOptions[0].value)}
          />
        </div>
      </div>

      <div className={styles.tabBar}>
        <TabNav
          items={TABS}
          activeKey={activeTab}
          onTabChange={(k) => setActiveTab(k as TabKey)}
        />
      </div>

      {activeTab === 'riepilogo' && (
      <section>
        <div className={styles.tabTools}>
          <div className={styles.search}>
            <SearchInput
              value={pivotSearch}
              onChange={setPivotSearch}
              placeholder="Filtra per cliente…"
            />
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleExportPivot}
            disabled={pivotFiltered.length === 0}
            leftIcon={<Icon name="download" size={14} />}
          >
            Esporta CSV
          </Button>
        </div>

        {pivotQ.isLoading && <Skeleton rows={6} />}

        {pivotQ.isError && (
          <div className={styles.errorBanner}>
            <span>Impossibile caricare il riepilogo.</span>
            <Button variant="secondary" size="sm" onClick={() => pivotQ.refetch()}>
              Riprova
            </Button>
          </div>
        )}

        {pivotQ.data && pivotFiltered.length > 0 && (
          <div className={`${shared.tableWrap} ${styles.pivotWrap}`}>
            <table className={`${shared.table} ${styles.pivotTable}`}>
              <thead>
                <tr>
                  <th>Cliente</th>
                  {MONTHS.map((m) => (
                    <th key={m.key} className={shared.numCol}>{m.label}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {pivotFiltered.map((p, i) => (
                  <tr key={`${p.customer ?? 'row'}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                    <td title={p.customer ?? ''}>{p.customer ?? ''}</td>
                    {MONTHS.map((m) => {
                      const { a, kw } = monthValues(p, m.key);
                      return (
                        <td key={m.key} className={shared.numCol}>
                          <MonthCell a={a} kw={kw} />
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {pivotQ.data && pivotFiltered.length === 0 && (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <Icon name="bar-chart-2" size={24} />
            </div>
            <div className={styles.emptyTitle}>
              {pivotHasFilter
                ? 'Nessun cliente corrisponde al filtro'
                : `Nessun consumo registrato nel ${year}`}
            </div>
            <div className={styles.emptyDesc}>
              {pivotHasFilter
                ? 'Modifica il testo o azzera il filtro per vedere tutti i clienti.'
                : 'I consumi vengono importati dopo la chiusura dei periodi di fatturazione. Prova con un anno precedente.'}
            </div>
          </div>
        )}
      </section>
      )}

      {activeTab === 'dettaglio' && (
      <section>
        <div className={styles.tabTools}>
          <div className={styles.search}>
            <SearchInput
              value={detailSearch}
              onChange={setDetailSearch}
              placeholder="Filtra per cliente…"
            />
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleExportDetail}
            disabled={detailSorted.length === 0}
            leftIcon={<Icon name="download" size={14} />}
          >
            Esporta CSV
          </Button>
        </div>

        {detailQ.isLoading && <Skeleton rows={6} />}

        {detailQ.isError && (
          <div className={styles.errorBanner}>
            <span>Impossibile caricare il dettaglio.</span>
            <Button variant="secondary" size="sm" onClick={() => detailQ.refetch()}>
              Riprova
            </Button>
          </div>
        )}

        {detailQ.data && detailSorted.length > 0 && (
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th aria-sort={ariaDetailSort('cliente')}>{detailSortHeader('cliente', 'Cliente')}</th>
                  <th aria-sort={ariaDetailSort('periodo')}>{detailSortHeader('periodo', 'Inizio periodo')}</th>
                  <th>Fine periodo</th>
                  <th className={shared.numCol}>Consumo</th>
                  <th className={shared.numCol}>Importo<Unit>€</Unit></th>
                  <th className={shared.numCol}>
                    <HeaderInfo label="PUN" hint={DETAIL_TOOLTIPS.pun!} />
                    <Unit>€/MWh</Unit>
                  </th>
                  <th className={shared.numCol}>
                    <HeaderInfo label="Coefficiente" hint={DETAIL_TOOLTIPS.coefficiente!} />
                  </th>
                  <th className={shared.numCol}>
                    <HeaderInfo label="Fisso CU" hint={DETAIL_TOOLTIPS.fisso_cu!} />
                    <Unit>€</Unit>
                  </th>
                  <th className={shared.numCol}>
                    <HeaderInfo label="Eccedenti" hint={DETAIL_TOOLTIPS.eccedenti!} />
                  </th>
                  <th className={shared.numCol}>
                    <HeaderInfo label="Importo eccedenti" hint={DETAIL_TOOLTIPS.importo_eccedenti!} />
                    <Unit>€</Unit>
                  </th>
                  <th>
                    <HeaderInfo label="Tipo variabile" hint={DETAIL_TOOLTIPS.tipo_variabile!} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {detailSorted.map((d, i) => (
                  <tr key={`${d.customer ?? 'row'}-${d.start_period ?? ''}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                    <td title={d.customer ?? ''}>{d.customer ?? ''}</td>
                    <td>{formatDate(d.start_period)}</td>
                    <td>{formatDate(d.end_period)}</td>
                    <td className={`${shared.numCol} ${styles.consumoCell}`}>
                      {d.consumo == null ? '' : (
                        <>
                          {formatNumber(d.consumo)}
                          <span className={styles.consumoUnit}>{consumoUnit(d.tipo_variabile)}</span>
                        </>
                      )}
                    </td>
                    <td className={shared.numCol}>{formatNumber(d.amount)}</td>
                    <td className={shared.numCol}>{formatNumber(d.pun)}</td>
                    <td className={shared.numCol}>{formatNumber(d.coefficiente)}</td>
                    <td className={shared.numCol}>{formatNumber(d.fisso_cu)}</td>
                    <td className={shared.numCol}>{formatNumber(d.eccedenti)}</td>
                    <td className={shared.numCol}>{formatNumber(d.importo_eccedenti)}</td>
                    <td>{d.tipo_variabile ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {detailQ.data && detailSorted.length === 0 && (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <Icon name="file-text" size={24} />
            </div>
            <div className={styles.emptyTitle}>
              {detailHasFilter
                ? 'Nessun periodo corrisponde al filtro'
                : `Nessun dettaglio registrato nel ${year}`}
            </div>
            <div className={styles.emptyDesc}>
              {detailHasFilter
                ? 'Modifica il testo o azzera il filtro per vedere tutti i periodi.'
                : 'I periodi di fatturazione appaiono qui dopo la chiusura.'}
            </div>
          </div>
        )}
      </section>
      )}
    </div>
  );
}
