import { useMemo, useState } from 'react';
import {
  Button,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
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

export default function ConsumiEnergiaColoPage() {
  const yearOptions = useMemo(() => buildYearOptions(), []);
  const [year, setYear] = useState(yearOptions[0].value);
  const [pivotSearch, setPivotSearch] = useState('');
  const [detailSearch, setDetailSearch] = useState('');

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

  const pivotHasFilter = pivotSearch.trim() !== '';
  const detailHasFilter = detailSearch.trim() !== '';

  function handleExportPivot() {
    const headers = [
      'Cliente',
      'Gennaio',
      'Febbraio',
      'Marzo',
      'Aprile',
      'Maggio',
      'Giugno',
      'Luglio',
      'Agosto',
      'Settembre',
      'Ottobre',
      'Novembre',
      'Dicembre',
    ];
    const rows = pivotFiltered.map((p) => [
      p.customer ?? '',
      p.gennaio,
      p.febbraio,
      p.marzo,
      p.aprile,
      p.maggio,
      p.giugno,
      p.luglio,
      p.agosto,
      p.settembre,
      p.ottobre,
      p.novembre,
      p.dicembre,
    ]);
    downloadCsv(`consumi-energia-colo_riepilogo_${year}.csv`, headers, rows);
  }

  function handleExportDetail() {
    const headers = [
      'Cliente',
      'Inizio periodo',
      'Fine periodo',
      'Consumo',
      'Importo (€)',
      'PUN (€/MWh)',
      'Coefficiente',
      'Fisso CU (€)',
      'Eccedenti',
      'Importo eccedenti (€)',
      'Tipo variabile',
    ];
    const rows = detailFiltered.map((d) => [
      d.customer ?? '',
      d.start_period ?? '',
      d.end_period ?? '',
      d.consumo,
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
      <h1 className={shared.title}>Consumi energia colocation</h1>
      <p className={styles.subtitle}>Consumi elettrici per clienti in colocation data center.</p>

      <div className={shared.toolbar}>
        <div className={shared.field}>
          <label>Anno</label>
          <div className={styles.yearSelect}>
            <SingleSelect
              options={yearOptions}
              selected={year}
              onChange={(v) => setYear((v as string | null) ?? yearOptions[0].value)}
            />
          </div>
        </div>
      </div>

      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h2 className={styles.sectionTitle}>Riepilogo mensile per cliente</h2>
          <div className={styles.sectionTools}>
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
                  <th className={shared.numCol}>Gennaio</th>
                  <th className={shared.numCol}>Febbraio</th>
                  <th className={shared.numCol}>Marzo</th>
                  <th className={shared.numCol}>Aprile</th>
                  <th className={shared.numCol}>Maggio</th>
                  <th className={shared.numCol}>Giugno</th>
                  <th className={shared.numCol}>Luglio</th>
                  <th className={shared.numCol}>Agosto</th>
                  <th className={shared.numCol}>Settembre</th>
                  <th className={shared.numCol}>Ottobre</th>
                  <th className={shared.numCol}>Novembre</th>
                  <th className={shared.numCol}>Dicembre</th>
                </tr>
              </thead>
              <tbody>
                {pivotFiltered.map((p, i) => (
                  <tr key={`${p.customer ?? 'row'}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                    <td title={p.customer ?? ''}>{p.customer ?? ''}</td>
                    <td className={shared.numCol}>{formatNumber(p.gennaio)}</td>
                    <td className={shared.numCol}>{formatNumber(p.febbraio)}</td>
                    <td className={shared.numCol}>{formatNumber(p.marzo)}</td>
                    <td className={shared.numCol}>{formatNumber(p.aprile)}</td>
                    <td className={shared.numCol}>{formatNumber(p.maggio)}</td>
                    <td className={shared.numCol}>{formatNumber(p.giugno)}</td>
                    <td className={shared.numCol}>{formatNumber(p.luglio)}</td>
                    <td className={shared.numCol}>{formatNumber(p.agosto)}</td>
                    <td className={shared.numCol}>{formatNumber(p.settembre)}</td>
                    <td className={shared.numCol}>{formatNumber(p.ottobre)}</td>
                    <td className={shared.numCol}>{formatNumber(p.novembre)}</td>
                    <td className={shared.numCol}>{formatNumber(p.dicembre)}</td>
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

      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h2 className={styles.sectionTitle}>Dettaglio per periodo</h2>
          <div className={styles.sectionTools}>
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
              disabled={detailFiltered.length === 0}
              leftIcon={<Icon name="download" size={14} />}
            >
              Esporta CSV
            </Button>
          </div>
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

        {detailQ.data && detailFiltered.length > 0 && (
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>Cliente</th>
                  <th>Inizio periodo</th>
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
                {detailFiltered.map((d, i) => (
                  <tr key={`${d.customer ?? 'row'}-${d.start_period ?? ''}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                    <td title={d.customer ?? ''}>{d.customer ?? ''}</td>
                    <td>{formatDate(d.start_period)}</td>
                    <td>{formatDate(d.end_period)}</td>
                    <td className={shared.numCol}>{formatNumber(d.consumo)}</td>
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

        {detailQ.data && detailFiltered.length === 0 && (
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
    </div>
  );
}
