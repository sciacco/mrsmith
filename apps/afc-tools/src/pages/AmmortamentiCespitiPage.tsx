import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
  TableToolbar,
  ToggleSwitch,
  useToast,
} from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import { downloadDepreciationExcel, useDepreciationRows } from '../api/queries';
import type { DepreciationRow } from '../types';
import { formatDate, formatMoneyEUR, formatNumber } from '../utils/format';
import shared from './shared.module.css';
import styles from './AmmortamentiCespitiPage.module.css';

type ColumnKind = 'text' | 'int' | 'money' | 'percent' | 'date';
type ColumnKey = keyof DepreciationRow;
type SortDir = 'asc' | 'desc';
type ViewMode = 'compact' | 'extended';

interface ColumnDef {
  key: ColumnKey;
  label: string;
  kind: ColumnKind;
}

const VIEW_STORAGE_KEY = 'afc-tools.ammortamenti-cespiti.view';

// Text search covers asset codes, description and serial number.
const SEARCH_FIELDS: ColumnKey[] = [
  'asset_code',
  'fixed_asset_code',
  'fixed_asset_description',
  'description',
  'serial_number',
];

// Columns whose values set the bottom totals row.
const TOTAL_KEYS: ColumnKey[] = [
  'historical_cost',
  'prior_depreciation',
  'current_year_depreciation',
  'residual_to_depreciate',
];

// All 28 endpoint fields, in endpoint order (Estesa view).
const ALL_COLUMNS: ColumnDef[] = [
  { key: 'company', label: 'Ditta', kind: 'int' },
  { key: 'group', label: 'Gruppo', kind: 'int' },
  { key: 'category', label: 'Categoria', kind: 'text' },
  { key: 'subcategory', label: 'Sottocategoria', kind: 'int' },
  { key: 'subcategory_description', label: 'Descrizione sottocategoria', kind: 'text' },
  { key: 'progress', label: 'Progressivo', kind: 'int' },
  { key: 'asset_code', label: 'Codice cespite', kind: 'int' },
  { key: 'fixed_asset_code', label: 'Codice immobilizzo', kind: 'int' },
  { key: 'fixed_asset_description', label: 'Descrizione immobilizzo', kind: 'text' },
  { key: 'fixed_asset_account', label: 'Conto immobilizzo', kind: 'text' },
  { key: 'depreciation_fund_account', label: 'Conto fondo ammortamento', kind: 'text' },
  { key: 'depreciation_expense_account', label: 'Conto costo ammortamento', kind: 'text' },
  { key: 'serial_number', label: 'Matricola', kind: 'text' },
  { key: 'description', label: 'Descrizione', kind: 'text' },
  { key: 'statutory_depreciation_rate', label: '% ammortamento civilistico', kind: 'percent' },
  { key: 'statutory_depreciation_amount', label: 'Quota ammortamento civilistico', kind: 'money' },
  { key: 'purchase_year', label: 'Anno acquisto', kind: 'int' },
  { key: 'sale_year', label: 'Anno vendita', kind: 'int' },
  { key: 'historical_cost', label: 'Costo storico', kind: 'money' },
  { key: 'prior_depreciation', label: 'Ammortamento precedente', kind: 'money' },
  { key: 'ordinary_depreciation', label: 'Ammortamento ordinario', kind: 'money' },
  { key: 'initial_residual_value', label: 'Residuo iniziale', kind: 'money' },
  { key: 'current_year_depreciation', label: 'Quota anno in corso', kind: 'money' },
  { key: 'residual_to_depreciate', label: 'Residuo da ammortizzare', kind: 'money' },
  { key: 'sale_amount', label: 'Vendita', kind: 'money' },
  { key: 'activation_date', label: 'Data attivazione', kind: 'date' },
  { key: 'deactivation_date', label: 'Data disattivazione', kind: 'date' },
  { key: 'notes', label: 'Note', kind: 'text' },
];

// Compact view keeps this subset, in this order.
const COMPACT_ORDER: ColumnKey[] = [
  'asset_code',
  'fixed_asset_code',
  'fixed_asset_description',
  'description',
  'serial_number',
  'subcategory',
  'purchase_year',
  'activation_date',
  'deactivation_date',
  'historical_cost',
  'prior_depreciation',
  'current_year_depreciation',
  'residual_to_depreciate',
];

const COLUMNS_BY_KEY = new Map<ColumnKey, ColumnDef>(ALL_COLUMNS.map((column) => [column.key, column]));

function compactColumns(): ColumnDef[] {
  return COMPACT_ORDER.map((key) => COLUMNS_BY_KEY.get(key)).filter(
    (column): column is ColumnDef => column !== undefined,
  );
}

function readStoredView(): ViewMode {
  try {
    return localStorage.getItem(VIEW_STORAGE_KEY) === 'extended' ? 'extended' : 'compact';
  } catch {
    return 'compact';
  }
}

function isEmptyValue(value: unknown): boolean {
  return value == null || value === '';
}

function compareRows(a: DepreciationRow, b: DepreciationRow, column: ColumnDef, dir: number): number {
  const aEmpty = isEmptyValue(a[column.key]);
  const bEmpty = isEmptyValue(b[column.key]);
  if (aEmpty && bEmpty) return 0;
  if (aEmpty) return 1; // missing values stay last regardless of direction
  if (bEmpty) return -1;

  if (column.kind === 'text') {
    return String(a[column.key]).localeCompare(String(b[column.key]), 'it') * dir;
  }
  if (column.kind === 'date') {
    return String(a[column.key]).localeCompare(String(b[column.key])) * dir;
  }

  return (Number(a[column.key]) - Number(b[column.key])) * dir;
}

function formatCell(column: ColumnDef, value: unknown): string {
  if (value == null || value === '') return '';
  switch (column.kind) {
    case 'money':
      return formatMoneyEUR(Number(value));
    case 'percent':
      return formatNumber(Number(value));
    case 'date':
      return formatDate(String(value));
    case 'int':
      return String(value);
    default:
      return String(value);
  }
}

function rowKey(row: DepreciationRow): string {
  return `${row.asset_code}-${row.fixed_asset_code}-${row.progress}`;
}

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export default function AmmortamentiCespitiPage() {
  const api = useApiClient();
  const { toast } = useToast();

  const yearOptions = useMemo(() => {
    const currentYear = new Date().getFullYear();
    return Array.from({ length: 11 }, (_, i) => {
      const value = currentYear - i;
      return { value, label: String(value) };
    });
  }, []);

  const [year, setYear] = useState(() => new Date().getFullYear());
  const [search, setSearch] = useState('');
  const [subcategory, setSubcategory] = useState<number | null>(null);
  const [view, setView] = useState<ViewMode>(readStoredView);
  const [sortKey, setSortKey] = useState<ColumnKey | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>('asc');
  const [exporting, setExporting] = useState(false);

  const q = useDepreciationRows(year);
  const rawData = q.data ?? [];

  useEffect(() => {
    try {
      localStorage.setItem(VIEW_STORAGE_KEY, view);
    } catch {
      // Persisting the preference is best-effort only.
    }
  }, [view]);

  const subcategoryOptions = useMemo(() => {
    const labels = new Map<number, string>();
    for (const row of rawData) {
      if (!labels.has(row.subcategory)) {
        labels.set(row.subcategory, row.subcategory_description?.trim() || String(row.subcategory));
      }
    }
    return Array.from(labels.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label, 'it'));
  }, [rawData]);

  const filtered = useMemo(() => {
    const needle = search.toLowerCase().trim();
    return rawData.filter((row) => {
      if (subcategory !== null && row.subcategory !== subcategory) return false;
      if (!needle) return true;
      return SEARCH_FIELDS.some((field) => String(row[field]).toLowerCase().includes(needle));
    });
  }, [rawData, search, subcategory]);

  const columns = view === 'extended' ? ALL_COLUMNS : compactColumns();

  const sorted = useMemo(() => {
    if (!sortKey) return filtered;
    const column = COLUMNS_BY_KEY.get(sortKey);
    if (!column) return filtered;
    const dir = sortDir === 'asc' ? 1 : -1;
    return [...filtered].sort((a, b) => compareRows(a, b, column, dir));
  }, [filtered, sortKey, sortDir]);

  const totals = useMemo(() => {
    const acc: Record<string, number> = {};
    for (const key of TOTAL_KEYS) acc[key] = 0;
    for (const row of filtered) {
      for (const key of TOTAL_KEYS) {
        const value = row[key];
        if (typeof value === 'number') acc[key] = (acc[key] ?? 0) + value;
      }
    }
    return acc;
  }, [filtered]);

  // Entrance animation fires only on the first data render of this page mount
  // (UI-UX §8.3): later year changes, searches, filters and sorts reuse or
  // suppress it.
  const firstDataRender = useRef(true);
  useEffect(() => {
    if (sorted.length > 0) firstDataRender.current = false;
  }, [sorted.length]);

  function toggleSort(key: ColumnKey) {
    if (sortKey === key) {
      setSortDir((dir) => (dir === 'asc' ? 'desc' : 'asc'));
      return;
    }
    setSortKey(key);
    setSortDir('asc');
  }

  function sortHeader(column: ColumnDef) {
    const active = sortKey === column.key;
    const arrow = active ? (sortDir === 'asc' ? '▲' : '▼') : '↕';
    return (
      <button
        type="button"
        className={`${styles.sortBtn} ${active ? styles.sortBtnActive : ''}`}
        onClick={() => toggleSort(column.key)}
      >
        {column.label}
        <span className={styles.sortArrow}>{arrow}</span>
      </button>
    );
  }

  function ariaSortFor(key: ColumnKey): 'ascending' | 'descending' | 'none' {
    if (sortKey !== key) return 'none';
    return sortDir === 'asc' ? 'ascending' : 'descending';
  }

  async function handleExport() {
    if (exporting) return;
    setExporting(true);
    try {
      const blob = await downloadDepreciationExcel(api, year);
      triggerDownload(blob, `ammortamenti-cespiti_${year}.xlsx`);
      toast("Download dell'Excel avviato", 'success');
    } catch {
      toast("Errore durante il download dell'Excel. Riprova.", 'error');
    } finally {
      setExporting(false);
    }
  }

  const hasActiveFilters = search.trim() !== '' || subcategory !== null;

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ammortamenti cespiti</h1>
      <p className={shared.info}>
        Registro annuale degli ammortamenti dei cespiti per l'anno selezionato.
      </p>

      <TableToolbar className={styles.toolbar}>
        <div className={styles.yearField}>
          <span className={styles.fieldLabel}>Anno</span>
          <SingleSelect
            options={yearOptions}
            selected={year}
            onChange={(value) => {
              if (value != null) setYear(value);
            }}
            ariaLabel="Anno"
          />
        </div>
        <div className={styles.search}>
          <SearchInput
            value={search}
            onChange={setSearch}
            placeholder="Cerca per codice cespite, immobilizzo, descrizione o matricola…"
            ariaLabel="Cerca cespiti"
          />
        </div>
        <div className={styles.selectWrap}>
          <SingleSelect
            options={subcategoryOptions}
            selected={subcategory}
            onChange={setSubcategory}
            placeholder="Sottocategoria"
            allowClear
            searchable
            ariaLabel="Filtra per sottocategoria"
          />
        </div>
        <div className={styles.viewToggle}>
          <ToggleSwitch
            id="ammortamenti-view"
            checked={view === 'extended'}
            onChange={(checked) => setView(checked ? 'extended' : 'compact')}
            label="Vista estesa"
          />
        </div>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            onClick={handleExport}
            disabled={rawData.length === 0}
            loading={exporting}
            leftIcon={<Icon name="download" size={16} />}
          >
            Esporta Excel
          </Button>
        </div>
      </TableToolbar>

      {q.isLoading && <Skeleton rows={10} />}

      {q.isError && (
        <div className={shared.error} role="alert">
          <span>Errore nel caricamento degli ammortamenti.</span>{' '}
          <Button variant="secondary" size="sm" onClick={() => q.refetch()}>
            Riprova
          </Button>
        </div>
      )}

      {q.data && rawData.length === 0 && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="package" size={32} />
          </div>
          <div className={styles.emptyTitle}>Nessun cespite per l'anno selezionato</div>
          <div className={styles.emptyDesc}>
            Non risultano ammortamenti per il {year}. Prova a selezionare un altro anno.
          </div>
        </div>
      )}

      {q.data && rawData.length > 0 && sorted.length === 0 && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="search" size={32} />
          </div>
          <div className={styles.emptyTitle}>Nessun cespite corrisponde ai filtri</div>
          <div className={styles.emptyDesc}>
            Modifica la ricerca o la sottocategoria, oppure svuotale per vedere tutti i cespiti dell'anno.
          </div>
        </div>
      )}

      {q.data && sorted.length > 0 && (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.key} aria-sort={ariaSortFor(column.key)}>
                    {sortHeader(column)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sorted.map((row, index) => (
                <tr
                  key={rowKey(row)}
                  className={styles.row}
                  style={
                    firstDataRender.current && index < 40
                      ? { animationDelay: `${Math.min(index * 8, 300)}ms` }
                      : { animation: 'none' }
                  }
                >
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={column.kind === 'text' ? undefined : styles.numCol}
                    >
                      {formatCell(column, row[column.key])}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className={styles.totalsRow}>
                {columns.map((column, index) => {
                  if (index === 0) {
                    return (
                      <td key={column.key} className={styles.totalsLabel}>
                        Totali
                      </td>
                    );
                  }
                  if (TOTAL_KEYS.includes(column.key)) {
                    return (
                      <td key={column.key} className={styles.numCol}>
                        {formatMoneyEUR(totals[column.key])}
                      </td>
                    );
                  }
                  return <td key={column.key} />;
                })}
              </tr>
            </tfoot>
          </table>
        </div>
      )}

      {q.data && sorted.length > 0 && (
        <p className={styles.rowCount}>
          {sorted.length} {sorted.length === 1 ? 'cespite' : 'cespiti'}
          {hasActiveFilters ? ` su ${rawData.length}` : ''}.
        </p>
      )}
    </div>
  );
}
