import { useEffect, useMemo, useRef, useState } from 'react';
import { Button } from '@mrsmith/ui';
import type { Islet, LayoutGridBlock, Position } from '../../api/types';
import { autoArrangePositions, glyphSize, isletCells, orientCells } from './layoutData';
import { IsletGlyph } from './IsletGlyph';
import { IsletDetailPanel } from './IsletDetailPanel';
import styles from './workspace.module.css';

const ZOOM_MIN = 0.5;
const ZOOM_MAX = 2.4;
const ZOOM_STEP = 0.15;
const FIT_PAD = 32;
const FIT_MAX = 1.6;

export function RoomCanvas({
  islets,
  blocks,
  positions,
  canOperate,
  selection,
  emphasizedIds,
  filtersActive,
  onSelectPosition,
  onOpenIsletActions,
  onMoveIslet,
}: {
  islets: Islet[];
  blocks: LayoutGridBlock[];
  positions: Position[];
  canOperate: boolean;
  selection: { type: 'islet' | 'position'; id: number } | null;
  emphasizedIds: Set<number>;
  filtersActive: boolean;
  onSelectPosition: (id: number) => void;
  onOpenIsletActions: (id: number) => void;
  onMoveIslet: (id: number, x: number, y: number, rotation: number) => void;
}) {
  const [zoom, setZoom] = useState(1);
  const [arrangeMode, setArrangeMode] = useState(false);
  const [focusIsletId, setFocusIsletId] = useState<number | null>(null);
  const [overrides, setOverrides] = useState<Record<number, { x: number; y: number }>>({});
  const [viewport, setViewport] = useState({ w: 800, h: 560 });

  const viewportRef = useRef<HTMLDivElement | null>(null);
  const scaleRef = useRef(1);
  const gesture = useRef<{ id: number; startX: number; startY: number; baseX: number; baseY: number; lastX: number; lastY: number } | null>(null);

  useEffect(() => {
    const node = viewportRef.current;
    if (!node) return undefined;
    const observer = new ResizeObserver(() => {
      const rect = node.getBoundingClientRect();
      setViewport({ w: Math.max(320, rect.width), h: Math.max(320, rect.height) });
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

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

  const layouts = useMemo(() =>
    islets.map((islet) => {
      const block = blockByIslet.get(islet.id);
      const isletPositions = positionsByIslet.get(islet.id) ?? [];
      const baseCells = isletCells(block, isletPositions);
      const oriented = orientCells(baseCells, islet.canvasRotation ?? 0);
      return { islet, baseCells, oriented, size: glyphSize(oriented), positions: isletPositions };
    }),
  [islets, blockByIslet, positionsByIslet]);

  const autoPos = useMemo(
    () => autoArrangePositions(layouts.map((l) => ({ id: l.islet.id, width: l.size.width, height: l.size.height }))),
    [layouts],
  );

  const placed = layouts.map((l) => {
    const override = overrides[l.islet.id];
    const saved = l.islet.canvasX != null && l.islet.canvasY != null ? { x: l.islet.canvasX, y: l.islet.canvasY } : null;
    const pos = override ?? saved ?? autoPos[l.islet.id] ?? { x: 0, y: 0 };
    return { ...l, x: pos.x, y: pos.y };
  });

  const plane = placed.reduce((acc, p) => ({ w: Math.max(acc.w, p.x + p.size.width), h: Math.max(acc.h, p.y + p.size.height) }), { w: 1, h: 1 });
  const fitScale = Math.min((viewport.w - FIT_PAD) / plane.w, (viewport.h - FIT_PAD) / plane.h, FIT_MAX);
  const scale = Math.max(0.1, fitScale) * zoom;
  scaleRef.current = scale;

  const focusLayout = focusIsletId != null ? layouts.find((l) => l.islet.id === focusIsletId) ?? null : null;
  const selectedPositionId = selection?.type === 'position' ? selection.id : null;

  function endGesture() {
    const active = gesture.current;
    gesture.current = null;
    window.removeEventListener('pointermove', handlePointerMove);
    window.removeEventListener('pointerup', endGesture);
    if (active) {
      const islet = islets.find((i) => i.id === active.id);
      onMoveIslet(active.id, Math.round(active.lastX), Math.round(active.lastY), islet?.canvasRotation ?? 0);
    }
  }

  function handlePointerMove(event: PointerEvent) {
    const active = gesture.current;
    if (!active) return;
    const z = scaleRef.current || 1;
    const nextX = Math.max(0, active.baseX + (event.clientX - active.startX) / z);
    const nextY = Math.max(0, active.baseY + (event.clientY - active.startY) / z);
    active.lastX = nextX;
    active.lastY = nextY;
    setOverrides((prev) => ({ ...prev, [active.id]: { x: nextX, y: nextY } }));
  }

  function startDrag(event: React.PointerEvent, item: (typeof placed)[number]) {
    if (!arrangeMode) return;
    event.stopPropagation();
    gesture.current = { id: item.islet.id, startX: event.clientX, startY: event.clientY, baseX: item.x, baseY: item.y, lastX: item.x, lastY: item.y };
    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', endGesture);
  }

  return (
    <div className={styles.mapWorkspace}>
      <div className={styles.roomOverview}>
        <div className={styles.roomOverviewToolbar}>
          {canOperate ? (
            <Button variant={arrangeMode ? 'primary' : 'secondary'} size="sm" onClick={() => setArrangeMode((v) => !v)}>
              {arrangeMode ? 'Fine disposizione' : 'Disponi isole'}
            </Button>
          ) : null}
          {arrangeMode ? <span className={styles.roomOverviewHint}>Trascina le isole; ⟳ ruota di 90°</span> : null}
          <div className={styles.roomOverviewZoom} role="group" aria-label="Zoom mappa">
            <button type="button" onClick={() => setZoom((z) => Math.max(ZOOM_MIN, Math.round((z - ZOOM_STEP) * 100) / 100))} disabled={zoom <= ZOOM_MIN} aria-label="Riduci zoom">−</button>
            <button type="button" onClick={() => setZoom(1)} disabled={zoom === 1} aria-label="Adatta alla sala">{Math.round(zoom * 100)}%</button>
            <button type="button" onClick={() => setZoom((z) => Math.min(ZOOM_MAX, Math.round((z + ZOOM_STEP) * 100) / 100))} disabled={zoom >= ZOOM_MAX} aria-label="Aumenta zoom">+</button>
          </div>
        </div>
        <div className={`${styles.roomOverviewViewport} ${arrangeMode ? styles.roomOverviewViewportEdit : ''}`} ref={viewportRef}>
          {islets.length === 0 ? (
            <div className={styles.roomCanvasEmpty}><p className={styles.emptyText}>Nessuna isola configurata in questa sala.</p></div>
          ) : (
            <div className={styles.roomOverviewPlane} style={{ width: plane.w, height: plane.h, transform: `scale(${scale})` }}>
              {placed.map((item) => (
                <div
                  key={item.islet.id}
                  className={styles.glyphWrap}
                  style={{ left: item.x, top: item.y, cursor: arrangeMode ? 'grab' : 'default' }}
                  onPointerDown={(event) => startDrag(event, item)}
                >
                  <IsletGlyph
                    islet={item.islet}
                    orientedCells={item.oriented}
                    size={item.size}
                    focused={focusIsletId === item.islet.id}
                    dimmed={filtersActive && !item.positions.some((p) => emphasizedIds.has(p.id))}
                    arrangeMode={arrangeMode}
                    selectedPositionId={selectedPositionId}
                    emphasizedIds={emphasizedIds}
                    filtersActive={filtersActive}
                    onFocus={() => setFocusIsletId(item.islet.id)}
                    onSelectPosition={onSelectPosition}
                  />
                  {arrangeMode ? (
                    <button
                      type="button"
                      className={styles.glyphRotate}
                      aria-label={`Ruota isola ${item.islet.name}`}
                      title="Ruota 90°"
                      onPointerDown={(event) => event.stopPropagation()}
                      onClick={(event) => {
                        event.stopPropagation();
                        onMoveIslet(item.islet.id, Math.round(item.x), Math.round(item.y), ((item.islet.canvasRotation ?? 0) + 90) % 360);
                      }}
                    >
                      ⟳
                    </button>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <IsletDetailPanel
        islet={focusLayout?.islet ?? null}
        cells={focusLayout?.baseCells ?? []}
        canOperate={canOperate}
        selectedPositionId={selectedPositionId}
        emphasizedIds={emphasizedIds}
        filtersActive={filtersActive}
        onSelectPosition={onSelectPosition}
        onOpenActions={() => focusIsletId != null && onOpenIsletActions(focusIsletId)}
        onClose={() => setFocusIsletId(null)}
      />
    </div>
  );
}
