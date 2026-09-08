import { useMemo, useState, type CSSProperties } from 'react';
import { Icon } from '@mrsmith/ui';
import { formatInstant } from '@mrsmith/format';
import type { Segnalazione } from '../../api/types';
import { segnalazioneStateLabel, segnalazioneStateVars, segnalazioneTitle } from '../../lib/segnalazioneStates';
import board from '../iniziative/board/board.module.css';
import styles from './Segnalazioni.module.css';

type SortKey = 'title' | 'state' | 'location' | 'fiscalId' | 'author' | 'createdAt' | 'updatedAt';
type SortDir = 'asc' | 'desc';

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: 'title', label: 'Segnalazione' },
  { key: 'state', label: 'Stato' },
  { key: 'location', label: 'Località' },
  { key: 'fiscalId', label: 'Identificativo fiscale' },
  { key: 'author', label: 'Autore' },
  { key: 'createdAt', label: 'Creata' },
  { key: 'updatedAt', label: 'Aggiornata' },
];

function sortValue(item: Segnalazione, key: SortKey): string {
  switch (key) {
    case 'title': return segnalazioneTitle(item).toLocaleLowerCase('it');
    case 'state': return segnalazioneStateLabel(item.state).toLocaleLowerCase('it');
    case 'location': return item.location.trim().toLocaleLowerCase('it');
    case 'fiscalId': return item.fiscalId.trim().toLocaleLowerCase('it');
    case 'author': return (item.createdByEmail ?? '').toLocaleLowerCase('it');
    case 'createdAt': return item.createdAt;
    case 'updatedAt': return item.updatedAt;
  }
}

/** Vista tabella: tutte le segnalazioni già filtrate dal padre (stato e
 *  ricerca), ordinabili per colonna. Righe operabili da tastiera (UI-UX §13.2):
 *  la cella primaria è un bottone. */
export function SegnalazioniTable({ items, onRowClick }: {
  items: Segnalazione[];
  onRowClick: (item: Segnalazione) => void;
}) {
  const [sortKey, setSortKey] = useState<SortKey>('updatedAt');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const sorted = useMemo(() => {
    const collator = new Intl.Collator('it');
    const factor = sortDir === 'asc' ? 1 : -1;
    return [...items].sort((a, b) => {
      const va = sortValue(a, sortKey);
      const vb = sortValue(b, sortKey);
      // I vuoti vanno sempre in coda, qualunque sia la direzione.
      if (!va && vb) return 1;
      if (va && !vb) return -1;
      return collator.compare(va, vb) * factor || collator.compare(b.updatedAt, a.updatedAt);
    });
  }, [items, sortKey, sortDir]);

  const toggleSort = (key: SortKey) => {
    if (key === sortKey) {
      setSortDir((current) => (current === 'asc' ? 'desc' : 'asc'));
      return;
    }
    setSortKey(key);
    setSortDir(key === 'createdAt' || key === 'updatedAt' ? 'desc' : 'asc');
  };

  return (
    <div className={board.tableWrap}>
      <div className={styles.tableScroll}>
        <table className={board.table}>
          <thead>
            <tr>
              {COLUMNS.map((column) => {
                const active = column.key === sortKey;
                return (
                  <th key={column.key} aria-sort={active ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}>
                    <button type="button" className={`${styles.sortBtn} ${active ? styles.sortOn : ''}`} onClick={() => toggleSort(column.key)}>
                      {column.label}
                      {active ? <Icon name={sortDir === 'asc' ? 'chevron-up' : 'chevron-down'} size={12} /> : null}
                    </button>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.length === 0 ? (
              <tr>
                <td colSpan={COLUMNS.length} className={styles.tableEmpty}>Nessuna segnalazione corrisponde ai filtri.</td>
              </tr>
            ) : (
              sorted.map((item) => (
                <tr key={item.id} className={board.tableRow} onClick={() => onRowClick(item)}>
                  <td>
                    <button type="button" className={styles.rowTitle} onClick={(event) => { event.stopPropagation(); onRowClick(item); }}>
                      {segnalazioneTitle(item)}
                    </button>
                  </td>
                  <td>
                    <span className={styles.statePill} style={segnalazioneStateVars(item.state) as CSSProperties}>{segnalazioneStateLabel(item.state)}</span>
                  </td>
                  <td>{item.location.trim() || '—'}</td>
                  <td className={styles.mono}>{item.fiscalId.trim() || '—'}</td>
                  <td className={styles.cell}>{item.createdByEmail || '—'}</td>
                  <td className={styles.num}>{formatInstant(item.createdAt)}</td>
                  <td className={styles.num}>{formatInstant(item.updatedAt)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
