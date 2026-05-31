import type { Islet, LayoutGridCell } from '../../api/types';
import { GLYPH_CELL, GLYPH_HEADER, type GlyphSize } from './layoutData';
import { gridCellSlots, type GridCellSlot, type SlotStatus } from './positions';
import styles from './workspace.module.css';

const STATUS_DOT: Record<SlotStatus, string> = {
  free: 'statusDotFree',
  occupied: 'statusDotOccupied',
  reserved: 'statusDotReserved',
  shared: 'statusDotShared',
};

const STATUS_LABEL: Record<SlotStatus, string> = {
  free: 'libera',
  occupied: 'occupata',
  reserved: 'riservata',
  shared: 'condivisa',
};

function slotLabel(slot: GridCellSlot): string {
  const side = slot.side === 'A' ? 'A mezzo alto' : slot.side === 'B' ? 'B mezzo basso' : '';
  return [side, STATUS_LABEL[slot.status], slot.rack?.name].filter(Boolean).join(' · ');
}

function cellLabel(cell: LayoutGridCell, slots: GridCellSlot[]): string {
  const position = `Posizione ${cell.pos ?? ''}`.trim();
  return slots.length > 1
    ? `${position} · Half · ${slots.map(slotLabel).join(' / ')}`
    : `${position} · ${slotLabel(slots[0]!)}`;
}

// Compact occupancy glyph for the room overview: the islet's real (oriented)
// rows×cols grid of status-colored cells. Fills its footprint with information —
// every position is a visible coloured pixel — so the whole room reads at a glance.
export function IsletGlyph({
  islet,
  orientedCells,
  size,
  focused,
  dimmed,
  arrangeMode,
  selectedPositionId,
  emphasizedIds,
  filtersActive,
  onFocus,
  onSelectPosition,
}: {
  islet: Islet;
  orientedCells: LayoutGridCell[][];
  size: GlyphSize;
  focused: boolean;
  dimmed: boolean;
  arrangeMode: boolean;
  selectedPositionId: number | null;
  emphasizedIds: Set<number>;
  filtersActive: boolean;
  onFocus: () => void;
  onSelectPosition: (id: number) => void;
}) {
  let occupied = 0;
  let total = 0;
  for (const row of orientedCells) {
    for (const cell of row) {
      const slots = gridCellSlots(cell);
      total += slots.length;
      occupied += slots.filter((slot) => slot.status === 'occupied' || slot.status === 'shared').length;
    }
  }

  const nameLength = islet.name.length;
  const availableWidth = Math.max(size.width - 32, 28);
  const estimatedFontSize = availableWidth / (nameLength * 0.55);
  const fontSizePx = Math.max(8.5, Math.min(13, estimatedFontSize));

  return (
    <div
      className={`${styles.isletGlyph} ${focused ? styles.isletGlyphFocused : ''} ${dimmed ? styles.isletGlyphDim : ''} ${arrangeMode ? styles.isletGlyphArrange : ''}`}
      style={{ width: size.width }}
    >
      <button
        type="button"
        className={styles.isletGlyphHeader}
        style={{ height: GLYPH_HEADER }}
        onClick={onFocus}
        disabled={arrangeMode}
        title={`${islet.name} — apri dettaglio`}
      >
        <strong style={{ fontSize: `${fontSizePx}px` }}>{islet.name}</strong>
        <span>{occupied}/{total || islet.rackNum}</span>
      </button>
      <div
        className={styles.isletGlyphGrid}
        style={{ gridTemplateColumns: `repeat(${Math.max(size.cols, 1)}, ${GLYPH_CELL}px)`, gridAutoRows: `${GLYPH_CELL}px` }}
      >
        {orientedCells.flatMap((row, rowIndex) =>
          row.map((cell, colIndex) => {
            const slots = gridCellSlots(cell);
            const key = `${rowIndex}-${colIndex}`;
            if (slots.length === 0) {
              return <span key={key} className={styles.glyphCellEmpty} aria-hidden="true" />;
            }
            const matched = !filtersActive || (cell.positionId ? emphasizedIds.has(cell.positionId) : false);
            const selected = Boolean(cell.positionId) && selectedPositionId === cell.positionId;
            const isHalf = slots.length > 1;
            const primarySlot = slots[0]!;
            return (
              <button
                key={key}
                type="button"
                className={`${styles.glyphCell} ${isHalf ? styles.glyphCellHalf : styles[STATUS_DOT[primarySlot.status]]} ${selected ? styles.glyphCellSelected : ''} ${filtersActive && !matched ? styles.glyphCellDim : ''}`}
                disabled={arrangeMode || !cell.positionId}
                title={cellLabel(cell, slots)}
                aria-label={cellLabel(cell, slots)}
                onClick={(event) => {
                  event.stopPropagation();
                  if (cell.positionId) onSelectPosition(cell.positionId);
                }}
              >
                {isHalf ? (
                  <>
                    <span className={styles.glyphHalfStack} aria-hidden="true">
                      {slots.map((slot) => (
                        <span key={slot.side ?? 'F'} className={`${styles.glyphHalfSlot} ${styles[STATUS_DOT[slot.status]]}`} />
                      ))}
                    </span>
                    <span className={styles.glyphCellNumber}>{cell.pos}</span>
                  </>
                ) : cell.pos}
              </button>
            );
          }),
        )}
      </div>
    </div>
  );
}
