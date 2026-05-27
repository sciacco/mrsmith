import type { LayoutGridBlock, LayoutGridCell } from '../../api/types';
import type { LayoutSelection } from './LayoutScene';
import styles from './workspace.module.css';

interface LayoutGridProps {
  blocks: LayoutGridBlock[];
  selected: LayoutSelection;
  onSelect: (selection: LayoutSelection) => void;
}

function statusClass(cell: LayoutGridCell) {
  if (cell.type !== 'position') return '';
  if (!cell.positionId) return styles.layoutGridCellIncomplete;
  const status = (cell.positionStatus ?? '').toLowerCase();
  if (status === 'occupied') return styles.occupied;
  if (status === 'reserved') return styles.reserved;
  if (status === 'free') return styles.free;
  return styles.layoutGridCellUnknown;
}

function positionStatusText(cell: LayoutGridCell) {
  if (!cell.positionId) return 'Posizione non trovata nei dati Grappa';
  const status = (cell.positionStatus ?? '').toLowerCase();
  if (status === 'occupied') return cell.rackName ?? 'Occupata';
  if (status === 'reserved') return 'Riservata';
  if (status === 'free') return 'Libera';
  return cell.positionStatus ?? 'Stato non indicato';
}

function isHalfRack(cell: LayoutGridCell) {
  return cell.rackType?.toLowerCase() === 'half' || cell.positionType?.toLowerCase() === 'half';
}

function halfRackLabel(cell: LayoutGridCell) {
  if (!isHalfRack(cell)) return null;
  if (cell.rackPos === 'A') return 'posizione alta';
  if (cell.rackPos === 'B') return 'posizione bassa';
  return null;
}

function halfRackBadge(cell: LayoutGridCell) {
  if (!isHalfRack(cell)) return null;
  if (cell.rackPos === 'A') return 'A';
  if (cell.rackPos === 'B') return 'B';
  return null;
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
  const halfBadge = halfRackBadge(cell);
  const halfLabel = halfRackLabel(cell);
  return (
    <>
      <span className={styles.layoutGridPositionTopline}>
        <strong>{cell.pos}</strong>
        {halfBadge ? <em aria-label={halfLabel ?? undefined}>{halfBadge}</em> : null}
      </span>
      <span>{positionStatusText(cell)}</span>
      {halfLabel ? <small>{halfLabel}</small> : null}
    </>
  );
}

export function LayoutGrid({ blocks, selected, onSelect }: LayoutGridProps) {
  if (blocks.length === 0) {
    return (
      <div className={styles.layoutSceneFallback}>
        <h3 className={styles.emptyTitle}>Mappa non configurata</h3>
        <p className={styles.emptyText}>Usa la Vista 3D per consultare isole e posizioni disponibili.</p>
      </div>
    );
  }

  return (
    <div className={styles.layoutGridScene} aria-label="Vista 2D layout sala">
      <div className={styles.layoutGridBlocks}>
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
                    const className = [
                      styles.layoutGridCell,
                      cell.type === 'empty' ? styles.layoutGridCellEmpty : '',
                      cell.type === 'label' ? styles.layoutGridCellLabel : '',
                      cell.type === 'plenum' ? styles.layoutGridCellPlenum : '',
                      cell.type === 'position' ? styles.layoutGridCellPosition : '',
                      statusClass(cell),
                      selectedCell ? styles.layoutGridCellSelected : '',
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
      <div className={styles.layoutLegend}>
        <span><i className={styles.legendFree} />Libera</span>
        <span><i className={styles.legendOccupied} />Occupata</span>
        <span><i className={styles.legendReserved} />Riservata</span>
      </div>
    </div>
  );
}
