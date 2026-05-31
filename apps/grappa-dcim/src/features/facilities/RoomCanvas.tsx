import { useMemo, useRef, useState } from 'react';
import { Button } from '@mrsmith/ui';
import type { Islet, LayoutGridBlock, Position } from '../../api/types';
import { defaultIsletPosition, isletFootprint } from './layoutData';
import { IslandNode } from './IslandNode';
import type { LayoutSelection } from './layoutTypes';
import styles from './workspace.module.css';

const ZOOM_MIN = 0.3;
const ZOOM_MAX = 2.4;
const ZOOM_STEP = 0.12;
const GRID_THRESHOLD = 1; // at/above this zoom, islands reveal their position grid

interface IslandLayout {
  islet: Islet;
  block?: LayoutGridBlock;
  positions: Position[];
  x: number;
  y: number;
  footprint: ReturnType<typeof isletFootprint>;
}

export function RoomCanvas({
  islets,
  blocks,
  positions,
  canOperate,
  selection,
  scopeIsletId,
  filtersActive,
  emphasizedIds,
  onSelectIslet,
  onOpenIsletDetail,
  onSelectPosition,
  onMoveIslet,
}: {
  islets: Islet[];
  blocks: LayoutGridBlock[];
  positions: Position[];
  canOperate: boolean;
  selection: LayoutSelection;
  scopeIsletId: number | null;
  filtersActive: boolean;
  emphasizedIds: Set<number>;
  onSelectIslet: (id: number) => void;
  onOpenIsletDetail: (id: number) => void;
  onSelectPosition: (id: number) => void;
  onMoveIslet: (id: number, x: number, y: number) => void;
}) {
  const [zoom, setZoom] = useState(0.7);
  const [pan, setPan] = useState({ x: 24, y: 24 });
  const [editMode, setEditMode] = useState(false);
  const [overrides, setOverrides] = useState<Record<number, { x: number; y: number }>>({});

  const zoomRef = useRef(zoom);
  zoomRef.current = zoom;
  const gesture = useRef<
    | { kind: 'pan'; startX: number; startY: number; panX: number; panY: number }
    | { kind: 'island'; isletId: number; startX: number; startY: number; baseX: number; baseY: number; lastX: number; lastY: number }
    | null
  >(null);

  const blockByIslet = useMemo(() => {
    const map = new Map<number, LayoutGridBlock>();
    for (const block of blocks) if (block.isletId) map.set(block.isletId, block);
    return map;
  }, [blocks]);

  const positionsByIslet = useMemo(() => {
    const map = new Map<number, Position[]>();
    for (const position of positions) {
      const list = map.get(position.isletId) ?? [];
      list.push(position);
      map.set(position.isletId, list);
    }
    return map;
  }, [positions]);

  const layouts = useMemo<IslandLayout[]>(() => {
    return islets.map((islet, index) => {
      const block = blockByIslet.get(islet.id);
      const isletPositions = positionsByIslet.get(islet.id) ?? [];
      const footprint = isletFootprint(block, isletPositions.length || islet.rackNum);
      const override = overrides[islet.id];
      const saved = islet.canvasX != null && islet.canvasY != null ? { x: islet.canvasX, y: islet.canvasY } : null;
      const fallback = defaultIsletPosition(index);
      const { x, y } = override ?? saved ?? fallback;
      return { islet, block, positions: isletPositions, x, y, footprint };
    });
  }, [islets, blockByIslet, positionsByIslet, overrides]);

  const planeSize = useMemo(() => {
    let width = 600;
    let height = 400;
    for (const item of layouts) {
      width = Math.max(width, item.x + item.footprint.width + 40);
      height = Math.max(height, item.y + item.footprint.height + 40);
    }
    return { width, height };
  }, [layouts]);

  function endGesture() {
    const active = gesture.current;
    gesture.current = null;
    window.removeEventListener('pointermove', handlePointerMove);
    window.removeEventListener('pointerup', endGesture);
    if (active?.kind === 'island') {
      onMoveIslet(active.isletId, Math.round(active.lastX), Math.round(active.lastY));
    }
  }

  function handlePointerMove(event: PointerEvent) {
    const active = gesture.current;
    if (!active) return;
    if (active.kind === 'pan') {
      setPan({ x: active.panX + (event.clientX - active.startX), y: active.panY + (event.clientY - active.startY) });
      return;
    }
    const z = zoomRef.current || 1;
    const nextX = Math.max(0, active.baseX + (event.clientX - active.startX) / z);
    const nextY = Math.max(0, active.baseY + (event.clientY - active.startY) / z);
    active.lastX = nextX;
    active.lastY = nextY;
    setOverrides((prev) => ({ ...prev, [active.isletId]: { x: nextX, y: nextY } }));
  }

  function startPan(event: React.PointerEvent) {
    gesture.current = { kind: 'pan', startX: event.clientX, startY: event.clientY, panX: pan.x, panY: pan.y };
    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', endGesture);
  }

  function startIslandDrag(event: React.PointerEvent, item: IslandLayout) {
    event.stopPropagation();
    if (!editMode) return;
    gesture.current = { kind: 'island', isletId: item.islet.id, startX: event.clientX, startY: event.clientY, baseX: item.x, baseY: item.y, lastX: item.x, lastY: item.y };
    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', endGesture);
  }

  const detail = zoom >= GRID_THRESHOLD;

  return (
    <div className={styles.roomCanvas}>
      <div className={styles.roomCanvasToolbar}>
        {canOperate ? (
          <Button variant={editMode ? 'primary' : 'secondary'} size="sm" onClick={() => setEditMode((value) => !value)}>
            {editMode ? 'Fine modifica' : 'Modifica layout'}
          </Button>
        ) : null}
        {editMode ? <span className={styles.roomCanvasHint}>Trascina le isole per disporle</span> : null}
        <div className={styles.roomCanvasZoom} role="group" aria-label="Zoom mappa">
          <button type="button" onClick={() => setZoom((z) => Math.max(ZOOM_MIN, Math.round((z - ZOOM_STEP) * 100) / 100))} disabled={zoom <= ZOOM_MIN} aria-label="Riduci zoom">−</button>
          <button type="button" onClick={() => { setZoom(0.7); setPan({ x: 24, y: 24 }); }} aria-label="Reimposta vista">{Math.round(zoom * 100)}%</button>
          <button type="button" onClick={() => setZoom((z) => Math.min(ZOOM_MAX, Math.round((z + ZOOM_STEP) * 100) / 100))} disabled={zoom >= ZOOM_MAX} aria-label="Aumenta zoom">+</button>
        </div>
      </div>

      <div
        className={`${styles.roomCanvasViewport} ${editMode ? styles.roomCanvasViewportEdit : ''}`}
        onPointerDown={startPan}
      >
        {islets.length === 0 ? (
          <div className={styles.roomCanvasEmpty}><p className={styles.emptyText}>Nessuna isola configurata in questa sala.</p></div>
        ) : (
          <div
            className={styles.roomCanvasPlane}
            style={{ width: planeSize.width, height: planeSize.height, transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})` }}
          >
            {layouts.map((item) => (
              <div
                key={item.islet.id}
                className={styles.islandNodeWrap}
                style={{ left: item.x, top: item.y, cursor: editMode ? 'grab' : 'default' }}
                onPointerDown={(event) => startIslandDrag(event, item)}
              >
                <IslandNode
                  islet={item.islet}
                  block={item.block}
                  positions={item.positions}
                  footprint={item.footprint}
                  detail={detail}
                  editMode={editMode}
                  scoped={scopeIsletId === item.islet.id || (selection?.type === 'islet' && selection.id === item.islet.id)}
                  dimmed={filtersActive && !item.positions.some((p) => emphasizedIds.has(p.id))}
                  selectedPositionId={selection?.type === 'position' ? selection.id : null}
                  emphasizedIds={emphasizedIds}
                  filtersActive={filtersActive}
                  onSelectIslet={() => onSelectIslet(item.islet.id)}
                  onOpenDetail={() => onOpenIsletDetail(item.islet.id)}
                  onSelectPosition={onSelectPosition}
                />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
