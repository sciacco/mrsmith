import { useState } from 'react';
import type { LayoutGridBlock, LayoutGridCell } from '../../api/types';
import type { LayoutSelection } from './layoutTypes';
import { fullRack, fullSlotStatus, isHalfPosition, rackAt, slotStatus, type HalfSide } from './positions';
import styles from './workspace.module.css';

interface LayoutGridProps {
  blocks: LayoutGridBlock[];
  selected: LayoutSelection;
  onSelect: (selection: LayoutSelection) => void;
  // When a search/filter is active, matching position cells are emphasised and the
  // rest dimmed, so the room map doubles as a "where is X" surface.
  emphasizedIds?: Set<number>;
  filtersActive?: boolean;
}

const ZOOM_STEP = 0.15;
const ZOOM_MIN = 0.6;
const ZOOM_MAX = 1.8;

function statusClass(cell: LayoutGridCell) {
  if (cell.type !== 'position') return '';
  if (!cell.positionId) return styles.layoutGridCellIncomplete;
  if (isHalfPosition(cell.positionType)) return styles.layoutGridCellHalf;
  // Full tile occupancy is the operator-maintained position status (a rack record
  // can exist on a free position) — except a shared cabinet, which is condiviso
  // (occupied by multiple customers) and can never be free. Half slots colour by side.
  const status = fullSlotStatus({ status: cell.positionStatus ?? '', racks: cell.racks });
  if (status === 'occupied') return styles.occupied;
  if (status === 'shared') return styles.shared;
  if (status === 'reserved') return styles.reserved;
  return styles.free;
}

function positionStatusText(cell: LayoutGridCell) {
  if (!cell.positionId) return 'Posizione non trovata nei dati Grappa';
  const status = fullSlotStatus({ status: cell.positionStatus ?? '', racks: cell.racks });
  if (status === 'occupied') return fullRack(cell.racks)?.name ?? 'Occupata';
  if (status === 'shared') return fullRack(cell.racks)?.name ?? 'Condivisa';
  if (status === 'reserved') return 'Riservata';
  if (status === 'free') return 'Libera';
  return cell.positionStatus ?? 'Stato non indicato';
}

function HalfSlot({ cell, side }: { cell: LayoutGridCell; side: HalfSide }) {
  const rack = rackAt(cell.racks, side);
  const status = slotStatus(cell.positionStatus, rack);
  return (
    <span className={`${styles.layoutGridHalfSlot} ${styles[status] || ''}`} aria-label={side === 'A' ? 'mezzo alto' : 'mezzo basso'}>
      <em>{side}</em>
      <small>{rack?.name ?? (status === 'reserved' ? 'Riservata' : 'Libera')}</small>
    </span>
  );
}

function plenumLabel(cell: LayoutGridCell) {
  const type = cell.plenumType?.trim();
  return type ? `Plenum ${type}` : 'Plenum';
}

function blockWidthClass(layoutWidth?: string) {
  const value = layoutWidth ?? '';
  if (value.includes('col-12') || value.includes('col-10')) return styles.layoutGridBlockFull;
  if (value.includes('col-7') || value.includes('col-4')) return styles.layoutGridBlockWide;
  if (value.includes('col-2')) return styles.layoutGridBlockNarrow;
  return styles.layoutGridBlockMedium;
}

function CellContent({ cell }: { cell: LayoutGridCell }) {
  if (cell.type === 'empty') return null;
  if (cell.type === 'label') return <span className={styles.layoutGridLabelText}>{cell.text}</span>;
  if (cell.type === 'plenum') {
    return (
      <>
        <strong>{plenumLabel(cell)}</strong>
        <span>{cell.plenumName ?? (cell.plenumId ? 'Collegato' : 'Da verificare')}</span>
      </>
    );
  }
  if (cell.positionId && isHalfPosition(cell.positionType)) {
    return (
      <>
        <span className={styles.layoutGridPositionTopline}>
          <strong>{cell.pos}</strong>
          <em aria-label="posizione divisa">½</em>
        </span>
        <span className={styles.layoutGridHalfStack}>
          <HalfSlot cell={cell} side="A" />
          <HalfSlot cell={cell} side="B" />
        </span>
      </>
    );
  }
  return (
    <>
      <span className={styles.layoutGridPositionTopline}>
        <strong>{cell.pos}</strong>
      </span>
      <span>{positionStatusText(cell)}</span>
    </>
  );
}

export function LayoutGrid({ blocks, selected, onSelect, emphasizedIds, filtersActive }: LayoutGridProps) {
  const [zoom, setZoom] = useState(1);

  if (blocks.length === 0) {
    return (
      <div className={styles.layoutSceneFallback}>
        <h3 className={styles.emptyTitle}>Mappa non configurata</h3>
        <p className={styles.emptyText}>Consulta isole e posizioni nell'elenco qui sotto.</p>
      </div>
    );
  }

  return (
    <div className={styles.layoutGridScene} aria-label="Vista 2D layout sala">
      <div className={styles.layoutZoomBar} role="group" aria-label="Zoom mappa">
        <button type="button" onClick={() => setZoom((z) => Math.max(ZOOM_MIN, Math.round((z - ZOOM_STEP) * 100) / 100))} disabled={zoom <= ZOOM_MIN} aria-label="Riduci zoom">−</button>
        <button type="button" onClick={() => setZoom(1)} disabled={zoom === 1} aria-label="Reimposta zoom">{Math.round(zoom * 100)}%</button>
        <button type="button" onClick={() => setZoom((z) => Math.min(ZOOM_MAX, Math.round((z + ZOOM_STEP) * 100) / 100))} disabled={zoom >= ZOOM_MAX} aria-label="Aumenta zoom">+</button>
      </div>
      <div className={styles.layoutGridViewport}>
      <div className={styles.layoutGridBlocks} style={{ transform: `scale(${zoom})`, transformOrigin: 'top left' }}>
        {blocks.map((block) => (
          <section key={block.id} className={`${styles.layoutGridBlock} ${blockWidthClass(block.layoutWidth)}`}>
            <div className={styles.layoutGridBlockHeader}>
              <div>
                <h2>{block.title}</h2>
                <span>{block.isletName}</span>
              </div>
              {!block.isletId ? <span className={styles.badgeMuted}>Incompleta</span> : null}
            </div>
            <div className={styles.layoutGridTable} role="grid" aria-label={block.title}>
              {block.grid.map((row, rowIndex) => (
                <div
                  // eslint-disable-next-line react/no-array-index-key
                  key={`${block.id}-${rowIndex}`}
                  className={styles.layoutGridRow}
                  style={{ gridTemplateColumns: `repeat(${Math.max(row.length, 1)}, minmax(4.75rem, 1fr))` }}
                  role="row"
                >
                  {row.map((cell, colIndex) => {
                    const selectedCell = cell.positionId && selected?.type === 'position' && selected.id === cell.positionId;
                    const matched = filtersActive && cell.type === 'position' && cell.positionId
                      ? emphasizedIds?.has(cell.positionId) ?? false
                      : false;
                    const dimmed = Boolean(filtersActive) && cell.type === 'position' && !matched;
                    const className = [
                      styles.layoutGridCell,
                      cell.type === 'empty' ? styles.layoutGridCellEmpty : '',
                      cell.type === 'label' ? styles.layoutGridCellLabel : '',
                      cell.type === 'plenum' ? styles.layoutGridCellPlenum : '',
                      cell.type === 'position' ? styles.layoutGridCellPosition : '',
                      statusClass(cell),
                      selectedCell ? styles.layoutGridCellSelected : '',
                      matched ? styles.layoutGridCellMatch : '',
                      dimmed ? styles.layoutGridCellDim : '',
                    ].filter(Boolean).join(' ');
                    if (cell.type === 'position') {
                      return (
                        <button
                          // eslint-disable-next-line react/no-array-index-key
                          key={`${block.id}-${rowIndex}-${colIndex}`}
                          type="button"
                          className={className}
                          disabled={!cell.positionId}
                          onClick={() => cell.positionId && onSelect({ type: 'position', id: cell.positionId })}
                          role="gridcell"
                          aria-label={`Posizione ${cell.pos ?? ''}`}
                        >
                          <CellContent cell={cell} />
                        </button>
                      );
                    }
                    return (
                      <div
                        // eslint-disable-next-line react/no-array-index-key
                        key={`${block.id}-${rowIndex}-${colIndex}`}
                        className={className}
                        role="gridcell"
                      >
                        <CellContent cell={cell} />
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
      </div>
      <div className={styles.layoutLegend}>
        <span><i className={styles.legendFree} />Libera</span>
        <span><i className={styles.legendOccupied} />Occupata</span>
        <span><i className={styles.legendShared} />Condivisa</span>
        <span><i className={styles.legendReserved} />Riservata</span>
      </div>
    </div>
  );
}
