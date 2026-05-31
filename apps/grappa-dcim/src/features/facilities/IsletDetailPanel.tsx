import { Icon } from '@mrsmith/ui';
import type { Islet, LayoutGridCell } from '../../api/types';
import { LayoutCell } from './LayoutCell';
import { gridCellStatus } from './positions';
import styles from './workspace.module.css';

// Focus detail of one islet: its full position grid (rich LayoutCell), fit to the
// panel width with internal vertical scroll — so tall islets (DC1: 3×12) never
// overflow the room. Always renders the canonical (unrotated) orientation for
// readability; rotation only affects the overview glyph + room placement.
export function IsletDetailPanel({
  islet,
  cells,
  canOperate,
  selectedPositionId,
  emphasizedIds,
  filtersActive,
  onSelectPosition,
  onOpenActions,
  onClose,
}: {
  islet: Islet | null;
  cells: LayoutGridCell[][];
  canOperate: boolean;
  selectedPositionId: number | null;
  emphasizedIds: Set<number>;
  filtersActive: boolean;
  onSelectPosition: (id: number) => void;
  onOpenActions: () => void;
  onClose: () => void;
}) {
  if (!islet) {
    return (
      <aside className={styles.isletDetailPanel} aria-label="Dettaglio isola">
        <div className={styles.isletDetailEmpty}>
          <h3 className={styles.emptyTitle}>Seleziona un'isola</h3>
          <p className={styles.emptyText}>Clicca un'isola nella mappa per vederne le posizioni e agire.</p>
        </div>
      </aside>
    );
  }

  let occupied = 0;
  let free = 0;
  let total = 0;
  for (const row of cells) {
    for (const cell of row) {
      const status = gridCellStatus(cell);
      if (status === null) continue;
      total += 1;
      if (status === 'occupied' || status === 'shared') occupied += 1;
      else if (status === 'free') free += 1;
    }
  }
  const cols = cells.reduce((max, row) => Math.max(max, row.length), 1);

  return (
    <aside className={styles.isletDetailPanel} aria-label={`Dettaglio isola ${islet.name}`}>
      <div className={styles.isletDetailHeader}>
        <div className={styles.isletDetailTitle}>
          <span className={styles.eyebrow}>Isola</span>
          <h3>{islet.name}</h3>
        </div>
        <span className={styles.badgeMuted}>{occupied}/{total || islet.rackNum}</span>
        {canOperate ? (
          <button type="button" className={styles.isletDetailAction} aria-label="Azioni isola" title="Azioni isola" onClick={onOpenActions}>
            <Icon name="settings" size={16} />
          </button>
        ) : null}
        <button type="button" className={styles.isletDetailAction} aria-label="Chiudi dettaglio" title="Chiudi" onClick={onClose}>
          <Icon name="x" size={16} />
        </button>
      </div>
      <div className={styles.isletDetailMeta}>
        <span><strong>{free}</strong> libere</span>
        <span><strong>{occupied}</strong> occupate</span>
        <span>Tipo {islet.type}</span>
      </div>
      <div className={styles.isletDetailScroll}>
        <div className={styles.isletDetailGrid}>
          {cells.map((row, rowIndex) => (
            // eslint-disable-next-line react/no-array-index-key
            <div key={rowIndex} className={styles.layoutGridRow} style={{ gridTemplateColumns: `repeat(${Math.max(row.length, 1)}, minmax(0, 1fr))` }} role="row">
              {row.map((cell, colIndex) => {
                const matched = !filtersActive || (cell.type === 'position' && cell.positionId ? emphasizedIds.has(cell.positionId) : false);
                return (
                  <LayoutCell
                    // eslint-disable-next-line react/no-array-index-key
                    key={colIndex}
                    cell={cell}
                    selected={Boolean(cell.positionId) && selectedPositionId === cell.positionId}
                    matched={filtersActive && cell.type === 'position' && Boolean(cell.positionId) && matched}
                    dimmed={filtersActive && cell.type === 'position' && !matched}
                    onSelect={onSelectPosition}
                  />
                );
              })}
            </div>
          ))}
          {cols === 0 ? <p className={styles.emptyText}>Nessuna posizione.</p> : null}
        </div>
      </div>
    </aside>
  );
}
