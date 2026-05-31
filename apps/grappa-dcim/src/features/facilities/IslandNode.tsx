import { Icon } from '@mrsmith/ui';
import type { Islet, LayoutGridBlock, LayoutGridCell, Position } from '../../api/types';
import { CANVAS_HEADER, type IsletFootprint } from './layoutData';
import { LayoutCell } from './LayoutCell';
import { positionEffectiveStatus, type SlotStatus } from './positions';
import styles from './workspace.module.css';

const STATUS_DOT: Record<SlotStatus, string> = {
  free: 'statusDotFree',
  occupied: 'statusDotOccupied',
  reserved: 'statusDotReserved',
  shared: 'statusDotShared',
};
const BAR_ORDER: SlotStatus[] = ['occupied', 'shared', 'reserved', 'free'];

function positionToCell(position: Position): LayoutGridCell {
  return {
    type: 'position',
    pos: position.num,
    positionId: position.id,
    positionStatus: position.status,
    positionType: position.type,
    racks: position.racks,
  };
}

function chunk<T>(items: T[], size: number): T[][] {
  const rows: T[][] = [];
  for (let i = 0; i < items.length; i += Math.max(size, 1)) rows.push(items.slice(i, i + Math.max(size, 1)));
  return rows;
}

// An islet rendered as a positioned canvas node. Below the detail threshold it
// shows a compact summary (name + occupancy bar); above it, the position grid.
export function IslandNode({
  islet,
  block,
  positions,
  footprint,
  detail,
  editMode,
  scoped,
  dimmed,
  selectedPositionId,
  emphasizedIds,
  filtersActive,
  onSelectIslet,
  onOpenDetail,
  onSelectPosition,
}: {
  islet: Islet;
  block?: LayoutGridBlock;
  positions: Position[];
  footprint: IsletFootprint;
  detail: boolean;
  editMode: boolean;
  scoped: boolean;
  dimmed?: boolean;
  selectedPositionId: number | null;
  emphasizedIds?: Set<number>;
  filtersActive?: boolean;
  onSelectIslet: () => void;
  onOpenDetail: () => void;
  onSelectPosition: (id: number) => void;
}) {
  const counts: Record<SlotStatus, number> = { free: 0, occupied: 0, reserved: 0, shared: 0 };
  for (const position of positions) counts[positionEffectiveStatus(position)] += 1;
  const total = positions.length || islet.rackNum;
  const occupied = counts.occupied + counts.shared;

  const gridRows: LayoutGridCell[][] = block && block.grid.length > 0 ? block.grid : chunk(positions.map(positionToCell), footprint.cols);

  return (
    <div
      className={`${styles.islandNode} ${scoped ? styles.islandNodeScoped : ''} ${dimmed ? styles.islandNodeDim : ''} ${editMode ? styles.islandNodeEditable : ''}`}
      style={{ width: footprint.width, minHeight: footprint.height }}
    >
      <div className={styles.islandNodeHeader} style={{ minHeight: CANVAS_HEADER }}>
        <button type="button" className={styles.islandNodeTitle} onClick={onSelectIslet} disabled={editMode}>
          {islet.name}
        </button>
        <span className={styles.islandNodeCount}>{occupied}/{total}</span>
        <button
          type="button"
          className={styles.islandNodeDetail}
          aria-label={`Dettaglio e azioni isola ${islet.name}`}
          onClick={(event) => {
            event.stopPropagation();
            onOpenDetail();
          }}
          disabled={editMode}
        >
          <Icon name="eye" size={14} />
        </button>
      </div>

      {detail ? (
        <div className={styles.islandNodeGrid}>
          {gridRows.length === 0 ? (
            <span className={styles.emptyText}>Nessuna posizione.</span>
          ) : (
            gridRows.map((row, rowIndex) => (
              // eslint-disable-next-line react/no-array-index-key
              <div key={rowIndex} className={styles.layoutGridRow} style={{ gridTemplateColumns: `repeat(${Math.max(row.length, 1)}, minmax(0, 1fr))` }} role="row">
                {row.map((cell, colIndex) => {
                  const matched = filtersActive && cell.type === 'position' && cell.positionId ? emphasizedIds?.has(cell.positionId) ?? false : false;
                  const cellDimmed = Boolean(filtersActive) && cell.type === 'position' && !matched;
                  return (
                    <LayoutCell
                      // eslint-disable-next-line react/no-array-index-key
                      key={colIndex}
                      cell={cell}
                      selected={Boolean(cell.positionId) && selectedPositionId === cell.positionId}
                      interactive={!editMode}
                      matched={matched}
                      dimmed={cellDimmed}
                      onSelect={onSelectPosition}
                    />
                  );
                })}
              </div>
            ))
          )}
        </div>
      ) : (
        <div className={styles.islandNodeSummary}>
          <div className={styles.islandNodeBar} aria-hidden="true">
            {BAR_ORDER.map((status) =>
              counts[status] > 0 && total > 0 ? (
                <span key={status} className={`${styles.islandNodeBarSeg} ${styles[STATUS_DOT[status]]}`} style={{ width: `${(counts[status] / total) * 100}%` }} />
              ) : null,
            )}
          </div>
          <span className={styles.islandNodeMeta}>{total} posizioni · {counts.free} libere</span>
        </div>
      )}
    </div>
  );
}
