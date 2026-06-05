import { useDeferredValue, useMemo, useState } from 'react';
import { SearchInput, Skeleton, SingleSelect, Drawer, StatusBadge } from '@mrsmith/ui';
import { useMorAnomalies } from '../../hooks/useMorAnomalies';
import { formatMoneyEUR } from '../../utils/format';
import { downloadCsv } from '../../utils/csv';
import type { MorAnomaly } from '../../api/morAnomalies';
import styles from './AnomalieMorPage.module.css';

type SortKey = 'conto' | 'cliente' | 'importo' | 'periodo' | 'stato' | 'tipologia';
type SortDirection = 'asc' | 'desc';
type FilterState = 'SI' | 'NO' | null;

function getWarningState(row: MorAnomaly): boolean {
  return row.ordine_presente === 'NO' || row.numero_ordine_corretto === 'NO';
}

function formatDaFatturare(val: boolean | string | null | undefined, fallback = 'N/D'): string {
  if (val === true || val === 'SI') return 'SI';
  if (val === false || val === 'NO') return 'NO';
  return fallback;
}

function getDaFatturareVariant(val: boolean | string | null | undefined): 'success' | 'neutral' {
  if (val === true || val === 'SI') return 'success';
  return 'neutral';
}

function sortRows(rows: MorAnomaly[], key: SortKey, dir: SortDirection): MorAnomaly[] {
  const m = dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    switch (key) {
      case 'conto':
        return a.conto.localeCompare(b.conto) * m;
      case 'cliente':
        const nameA = a.intestazione || '';
        const nameB = b.intestazione || '';
        return nameA.localeCompare(nameB) * m;
      case 'importo':
        return ((a.importo ?? 0) - (b.importo ?? 0)) * m;
      case 'periodo':
        const perA = a.periodo_inizio || '';
        const perB = b.periodo_inizio || '';
        return perA.localeCompare(perB) * m;
      case 'stato':
        const stA = a.stato || '';
        const stB = b.stato || '';
        return stA.localeCompare(stB) * m;
      case 'tipologia':
        const typA = a.tipologia || '';
        const typB = b.tipologia || '';
        return typA.localeCompare(typB) * m;
      default:
        return 0;
    }
  });
}

export function AnomalieMorPage() {
  const { data, isLoading, error } = useMorAnomalies();

  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [ordinePresenteFilter, setOrdinePresenteFilter] = useState<FilterState>(null);
  const [ordineCorrettoFilter, setOrdineCorrettoFilter] = useState<FilterState>(null);
  const [filterPreset, setFilterPreset] = useState<'all' | 'anomalies' | 'missing_order' | 'incorrect_number'>('all');
  const [selectedRow, setSelectedRow] = useState<MorAnomaly | null>(null);

  const [sortKey, setSortKey] = useState<SortKey>('conto');
  const [sortDir, setSortDir] = useState<SortDirection>('asc');

  // Calculate absolute KPIs from raw data
  const { totalAnomalies, anomaliesAmount, missingOrders, incorrectNumbers } = useMemo(() => {
    if (!data) return { totalAnomalies: 0, anomaliesAmount: 0, missingOrders: 0, incorrectNumbers: 0 };
    
    let anomaliesCount = 0;
    let amountSum = 0;
    let missingCount = 0;
    let incorrectCount = 0;

    for (const row of data) {
      const isAnomaly = getWarningState(row);
      if (isAnomaly) {
        anomaliesCount++;
        amountSum += row.importo || 0;
      }
      if (row.ordine_presente === 'NO') {
        missingCount++;
      }
      if (row.numero_ordine_corretto === 'NO') {
        incorrectCount++;
      }
    }

    return {
      totalAnomalies: anomaliesCount,
      anomaliesAmount: amountSum,
      missingOrders: missingCount,
      incorrectNumbers: incorrectCount,
    };
  }, [data]);

  // Filters logic
  const filteredRows = useMemo(() => {
    if (!data) return [];
    
    return data.filter((row) => {
      // 1. Search Query
      if (deferredSearch.trim()) {
        const query = deferredSearch.toLowerCase();
        const matchConto = row.conto.toLowerCase().includes(query);
        const matchIntestazione = (row.intestazione || '').toLowerCase().includes(query);
        const matchSerial = (row.serialnumber || '').toLowerCase().includes(query);
        const matchTipologia = (row.tipologia || '').toLowerCase().includes(query);
        const matchName = `${row.firstname || ''} ${row.lastname || ''}`.toLowerCase().includes(query);
        
        if (!matchConto && !matchIntestazione && !matchSerial && !matchTipologia && !matchName) {
          return false;
        }
      }

      // 2. Dropdown: Ordine Presente
      if (ordinePresenteFilter !== null) {
        if (row.ordine_presente !== ordinePresenteFilter) return false;
      }

      // 3. Dropdown: N. Ordine Corretto
      if (ordineCorrettoFilter !== null) {
        if (row.numero_ordine_corretto !== ordineCorrettoFilter) return false;
      }

      // 4. KPI Tab Presets
      const isAnomaly = getWarningState(row);
      if (filterPreset === 'anomalies') {
        if (!isAnomaly) return false;
      } else if (filterPreset === 'missing_order') {
        if (row.ordine_presente !== 'NO') return false;
      } else if (filterPreset === 'incorrect_number') {
        if (row.numero_ordine_corretto !== 'NO') return false;
      }

      return true;
    });
  }, [data, deferredSearch, ordinePresenteFilter, ordineCorrettoFilter, filterPreset]);

  // Sort rows
  const sortedRows = useMemo(() => {
    return sortRows(filteredRows, sortKey, sortDir);
  }, [filteredRows, sortKey, sortDir]);

  const updateSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  };

  const sortHeader = (key: SortKey, label: string) => {
    const active = sortKey === key;
    return (
      <button
        type="button"
        className={`${styles.sortButton} ${active ? styles.sortButtonActive : ''}`}
        onClick={(e) => {
          e.stopPropagation();
          updateSort(key);
        }}
      >
        {label}
        <span className={styles.sortGlyph} aria-hidden="true">
          {active ? (sortDir === 'asc' ? ' ▲' : ' ▼') : ' ↕'}
        </span>
      </button>
    );
  };

  const handleExport = () => {
    if (!filteredRows.length) return;
    const headers = [
      'Conto',
      'Cognome',
      'Nome',
      'Da Fatturare',
      'Codice Ordine',
      'Serial Number',
      'Periodo Inizio',
      'Importo',
      'Stato',
      'Tipologia',
      'ID Cliente',
      'Intestazione',
      'Ordine Presente',
      'N. Ordine Corretto'
    ];
    const rows = filteredRows.map((row) => [
      row.conto,
      row.lastname || '',
      row.firstname || '',
      formatDaFatturare(row.is_da_fatturare, ''),
      row.codice_ordine || '',
      row.serialnumber || '',
      row.periodo_inizio || '',
      row.importo != null ? row.importo : '',
      row.stato || '',
      row.tipologia || '',
      row.id_cliente != null ? row.id_cliente : '',
      row.intestazione || '',
      row.ordine_presente,
      row.numero_ordine_corretto
    ]);
    downloadCsv('anomalie-mor.csv', headers, rows);
  };

  const isFilterActive = search.trim() !== '' || ordinePresenteFilter !== null || ordineCorrettoFilter !== null || filterPreset !== 'all';

  const handleResetFilters = () => {
    setSearch('');
    setOrdinePresenteFilter(null);
    setOrdineCorrettoFilter(null);
    setFilterPreset('all');
  };

  const optionsPresente: { value: 'SI' | 'NO'; label: string }[] = [
    { value: 'SI', label: 'Presente' },
    { value: 'NO', label: 'Mancante' },
  ];

  const optionsCorretto: { value: 'SI' | 'NO'; label: string }[] = [
    { value: 'SI', label: 'Corretto' },
    { value: 'NO', label: 'Errato' },
  ];

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.pageTitle}>Anomalie MOR</h1>
          <p className={styles.pageSubtitle}>
            Riconciliazione dei conti telefonici e identificazione delle discrepanze con gli ordini ERP.
          </p>
        </div>
      </div>

      {isLoading && <Skeleton rows={8} />}

      {error && (
        <div className={styles.stateBox}>
          <div className={styles.stateIcon}>
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true" stroke="currentColor" strokeWidth="1.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v4m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <p className={styles.stateTitle}>Errore nel caricamento</p>
          <p className={styles.stateText}>Non è stato possibile caricare le anomalie MOR. Per favore riprova.</p>
        </div>
      )}

      {data && (
        <>
          {/* KPI Summary Row */}
          <div className={styles.kpiContainer}>
            <button
              type="button"
              className={`${styles.kpiCard} ${filterPreset === 'anomalies' ? styles.kpiCardActive : ''}`}
              onClick={() => setFilterPreset((p) => (p === 'anomalies' ? 'all' : 'anomalies'))}
              title="Filtra per mostrare tutte le anomalie"
            >
              <span className={styles.kpiLabel}>Anomalie Totali</span>
              <span className={styles.kpiValue}>{totalAnomalies}</span>
            </button>
            
            <button
              type="button"
              className={`${styles.kpiCard} ${filterPreset === 'anomalies' ? styles.kpiCardActive : ''}`}
              onClick={() => setFilterPreset((p) => (p === 'anomalies' ? 'all' : 'anomalies'))}
              title="Filtra per mostrare tutte le anomalie"
            >
              <span className={styles.kpiLabel}>Valore Anomalie</span>
              <span className={styles.kpiValue}>{formatMoneyEUR(anomaliesAmount)}</span>
            </button>

            <button
              type="button"
              className={`${styles.kpiCard} ${filterPreset === 'missing_order' ? styles.kpiCardActive : ''}`}
              onClick={() => setFilterPreset((p) => (p === 'missing_order' ? 'all' : 'missing_order'))}
              title="Filtra per conti senza ordine ERP"
            >
              <span className={styles.kpiLabel}>Ordini non Trovati</span>
              <span className={styles.kpiValue}>{missingOrders}</span>
            </button>

            <button
              type="button"
              className={`${styles.kpiCard} ${filterPreset === 'incorrect_number' ? styles.kpiCardActive : ''}`}
              onClick={() => setFilterPreset((p) => (p === 'incorrect_number' ? 'all' : 'incorrect_number'))}
              title="Filtra per conti con codice ordine errato"
            >
              <span className={styles.kpiLabel}>Ordini Errati</span>
              <span className={styles.kpiValue}>{incorrectNumbers}</span>
            </button>
          </div>

          {/* Table Toolbar */}
          <div className={styles.tableCard}>
            <div className={styles.tableTools}>
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder="Cerca per conto, cliente, seriale, tipologia..."
                className={styles.search}
              />
              
              <div className={styles.filterWrapper}>
                <SingleSelect<'SI' | 'NO'>
                  options={optionsPresente}
                  selected={ordinePresenteFilter}
                  onChange={(v) => setOrdinePresenteFilter(v)}
                  placeholder="Ordine presente"
                  allowClear={true}
                  clearLabel="Tutti gli ordini"
                />
              </div>

              <div className={styles.filterWrapper}>
                <SingleSelect<'SI' | 'NO'>
                  options={optionsCorretto}
                  selected={ordineCorrettoFilter}
                  onChange={(v) => setOrdineCorrettoFilter(v)}
                  placeholder="Numero ordine"
                  allowClear={true}
                  clearLabel="Tutti i numeri"
                />
              </div>

              {isFilterActive && (
                <button type="button" className={styles.resetBtn} onClick={handleResetFilters}>
                  Ripristina filtri
                </button>
              )}

              <button
                type="button"
                className={styles.exportBtn}
                onClick={handleExport}
                disabled={filteredRows.length === 0}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="7 10 12 15 17 10" />
                  <line x1="12" y1="15" x2="12" y2="3" />
                </svg>
                Esporta CSV
              </button>
            </div>

            {/* Table */}
            {filteredRows.length === 0 ? (
              <div className={styles.stateBox}>
                <div className={styles.stateIcon}>
                  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="11" cy="11" r="8" />
                    <line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                </div>
                <p className={styles.stateTitle}>Nessun risultato trovato</p>
                <p className={styles.stateText}>Nessuna anomalia corrisponde ai criteri di ricerca impostati.</p>
                {isFilterActive && (
                  <button type="button" className={styles.resetBtn} onClick={handleResetFilters} style={{ marginTop: 'var(--space-3)' }}>
                    Azzera filtri di ricerca
                  </button>
                )}
              </div>
            ) : (
              <div className={styles.tableScroll}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.barCol} />
                      <th>{sortHeader('cliente', 'Cliente / Conto')}</th>
                      <th>Serial Number / Tipo</th>
                      <th>{sortHeader('periodo', 'Periodo')}</th>
                      <th className={styles.numCol}>{sortHeader('importo', 'Importo')}</th>
                      <th>Ordine Pres.</th>
                      <th>Ordine Corr.</th>
                      <th>{sortHeader('stato', 'Stato')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedRows.map((row, i) => {
                      const isAnomaly = getWarningState(row);
                      const isSelected = selectedRow?.conto === row.conto && selectedRow?.serialnumber === row.serialnumber;
                      
                      return (
                        <tr
                          key={`${row.conto}-${row.serialnumber}-${i}`}
                          className={`${isSelected ? styles.rowSelected : ''} ${isAnomaly ? styles.rowWarning : ''}`}
                          onClick={() => setSelectedRow(row)}
                          style={{ animationDelay: `${Math.min(i * 0.02, 0.6)}s` }}
                          aria-selected={isSelected}
                        >
                          <td className={styles.barCol}>
                            <div className={`${styles.accentBar} ${isAnomaly ? styles.barAnomaly : ''}`} />
                          </td>
                          <td>
                            <div className={styles.primaryText}>{row.intestazione || 'N/D'}</div>
                            <div className={styles.secondaryText}>
                              Conto: {row.conto} {row.id_cliente ? `• ID: ${row.id_cliente}` : ''}
                            </div>
                          </td>
                          <td>
                            <span className={styles.serialText}>{row.serialnumber || 'N/D'}</span>
                            <div className={styles.secondaryText}>{row.tipologia || 'N/D'}</div>
                          </td>
                          <td>{row.periodo_inizio || 'N/D'}</td>
                          <td className={styles.numCol}>
                            <span className={styles.amountText}>
                              {row.importo != null ? formatMoneyEUR(row.importo) : ''}
                            </span>
                          </td>
                          <td>
                            <StatusBadge
                              value={row.ordine_presente}
                              variant={row.ordine_presente === 'SI' ? 'success' : 'danger'}
                              dot={true}
                            />
                          </td>
                          <td>
                            <StatusBadge
                              value={row.numero_ordine_corretto}
                              variant={row.numero_ordine_corretto === 'SI' ? 'success' : 'danger'}
                              dot={true}
                            />
                          </td>
                          <td>
                            <StatusBadge value={row.stato} />
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}

      {/* Details Side Drawer */}
      <Drawer
        open={selectedRow !== null}
        onClose={() => setSelectedRow(null)}
        title="Dettaglio Anomalia MOR"
        subtitle={selectedRow ? `Conto #${selectedRow.conto}` : undefined}
        size="md"
      >
        {selectedRow && (
          <div className={styles.drawerContent}>
            {getWarningState(selectedRow) && (
              <div className={styles.drawerAlert}>
                <svg className={styles.alertIcon} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                  <line x1="12" y1="9" x2="12" y2="13" />
                  <line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
                <div>
                  <div className={styles.alertTitle}>Anomalia Rilevata</div>
                  <p className={styles.alertText}>
                    {selectedRow.ordine_presente === 'NO' && selectedRow.numero_ordine_corretto === 'NO'
                      ? 'Il conto telefonico non ha ordini associati nel sistema ERP e il codice ordine risulta non corretto.'
                      : selectedRow.ordine_presente === 'NO'
                      ? 'Il conto telefonico non presenta ordini corrispondenti nel sistema ERP per il serial number specificato.'
                      : "L'ordine corrispondente per il serial number specificato è presente, ma il numero ordine indicato non è corretto."}
                  </p>
                </div>
              </div>
            )}

            <div className={styles.drawerSection}>
              <h3>Dati Cliente</h3>
              <div className={styles.drawerGrid}>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Intestazione</span>
                  <span className={styles.fieldValue}>{selectedRow.intestazione || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>ID Cliente</span>
                  <span className={styles.fieldValue}>{selectedRow.id_cliente || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Referente</span>
                  <span className={styles.fieldValue}>
                    {selectedRow.firstname || selectedRow.lastname
                      ? `${selectedRow.firstname || ''} ${selectedRow.lastname || ''}`.trim()
                      : 'N/D'}
                  </span>
                </div>
              </div>
            </div>

            <div className={styles.drawerSection}>
              <h3>Dettagli Conto & Fatturazione</h3>
              <div className={styles.drawerGrid}>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Numero Conto</span>
                  <span className={styles.fieldValue}>{selectedRow.conto}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Tipologia</span>
                  <span className={styles.fieldValue}>{selectedRow.tipologia || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Stato Conto</span>
                  <span className={styles.fieldValue}>
                    <StatusBadge value={selectedRow.stato} />
                  </span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Importo</span>
                  <span className={`${styles.fieldValue} ${styles.drawerAmount}`}>
                    {selectedRow.importo != null ? formatMoneyEUR(selectedRow.importo) : 'N/D'}
                  </span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Inizio Periodo</span>
                  <span className={styles.fieldValue}>{selectedRow.periodo_inizio || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Da Fatturare</span>
                  <span className={styles.fieldValue}>
                    <StatusBadge
                      value={formatDaFatturare(selectedRow.is_da_fatturare)}
                      variant={getDaFatturareVariant(selectedRow.is_da_fatturare)}
                      label={formatDaFatturare(selectedRow.is_da_fatturare)}
                      dot={false}
                    />
                  </span>
                </div>
              </div>
            </div>

            <div className={styles.drawerSection}>
              <h3>Corrispondenza ERP</h3>
              <div className={styles.drawerGrid}>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Serial Number</span>
                  <span className={`${styles.fieldValue} ${styles.monoVal}`}>{selectedRow.serialnumber || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Codice Ordine ERP</span>
                  <span className={`${styles.fieldValue} ${styles.monoVal}`}>{selectedRow.codice_ordine || 'N/D'}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Ordine Presente</span>
                  <span className={styles.fieldValue}>
                    <StatusBadge
                      value={selectedRow.ordine_presente}
                      variant={selectedRow.ordine_presente === 'SI' ? 'success' : 'danger'}
                      dot={true}
                    />
                  </span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>N. Ordine Corretto</span>
                  <span className={styles.fieldValue}>
                    <StatusBadge
                      value={selectedRow.numero_ordine_corretto}
                      variant={selectedRow.numero_ordine_corretto === 'SI' ? 'success' : 'danger'}
                      dot={true}
                    />
                  </span>
                </div>
              </div>
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
