import type { LayoutGridCell } from '../../api/types';
import { fullRack, fullSlotStatus, isHalfPosition, rackAt, slotStatus, type HalfSide } from './positions';
import styles from './workspace.module.css';

// Single layout-grid cell renderer, shared by the island node grid. Mirrors the
// original LayoutGrid cell rendering (position / empty / label / plenum, half A/B),
// with optional search emphasis (matched) / dim.

function statusClass(cell: LayoutGridCell) {
  if (cell.type !== 'position') return '';
  if (!cell.positionId) return styles.layoutGridCellIncomplete;
  if (isHalfPosition(cell.positionType)) return styles.layoutGridCellHalf;
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

export function LayoutCell({
  cell,
  selected,
  interactive = true,
  matched,
  dimmed,
  onSelect,
}: {
  cell: LayoutGridCell;
  selected: boolean;
  interactive?: boolean;
  matched?: boolean;
  dimmed?: boolean;
  onSelect?: (positionId: number) => void;
}) {
  const className = [
    styles.layoutGridCell,
    cell.type === 'empty' ? styles.layoutGridCellEmpty : '',
    cell.type === 'label' ? styles.layoutGridCellLabel : '',
    cell.type === 'plenum' ? styles.layoutGridCellPlenum : '',
    cell.type === 'position' ? styles.layoutGridCellPosition : '',
    statusClass(cell),
    selected ? styles.layoutGridCellSelected : '',
    matched ? styles.layoutGridCellMatch : '',
    dimmed ? styles.layoutGridCellDim : '',
  ]
    .filter(Boolean)
    .join(' ');

  if (cell.type === 'position' && interactive) {
    return (
      <button
        type="button"
        className={className}
        disabled={!cell.positionId}
        onClick={() => cell.positionId && onSelect?.(cell.positionId)}
        role="gridcell"
        aria-label={`Posizione ${cell.pos ?? ''}`}
      >
        <CellContent cell={cell} />
      </button>
    );
  }
  return (
    <div className={className} role="gridcell">
      <CellContent cell={cell} />
    </div>
  );
}
