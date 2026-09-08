import { forwardRef, useMemo, useState, type CSSProperties, type HTMLAttributes } from 'react';
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  pointerWithin,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
  type DragStartEvent,
  type ScreenReaderInstructions,
} from '@dnd-kit/core';
import { Icon } from '@mrsmith/ui';
import { formatInstant } from '@mrsmith/format';
import type { Segnalazione, SegnalazioneState } from '../../api/types';
import { SEGNALAZIONE_STATES, segnalazioneStateLabel, segnalazioneStateVars, segnalazioneTitle } from '../../lib/segnalazioneStates';
import board from '../iniziative/board/board.module.css';
import styles from './Segnalazioni.module.css';

const DROP_PREFIX = 'segnalazione-state:';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  item: Segnalazione;
  dragging?: boolean;
}

/** Card riconoscibile anche senza nome: titolo di ripiego, riga secondaria con
 *  località e sito, piede con autore e ultimo aggiornamento. */
export const SegnalazioneCard = forwardRef<HTMLDivElement, CardProps>(function SegnalazioneCard({ item, dragging, className, style, ...rest }, ref) {
  const title = segnalazioneTitle(item);
  const secondary = [item.location.trim(), item.name.trim() ? item.website.trim() : ''].filter(Boolean);
  return (
    <div
      ref={ref}
      className={[board.kcard, dragging ? board.dragging : '', className ?? ''].filter(Boolean).join(' ')}
      style={{ ...segnalazioneStateVars(item.state), ...style } as CSSProperties}
      role="button"
      tabIndex={0}
      aria-label={`${title}, ${segnalazioneStateLabel(item.state)}`}
      {...rest}
    >
      <div className={board.cardName}>
        <span className={board.nm}>{title}</span>
        <Icon name="grip-vertical" size={14} className={board.grab} />
      </div>
      {secondary.length ? (
        <div className={styles.cardSub}>
          {secondary.map((text) => <span key={text}>{text}</span>)}
        </div>
      ) : null}
      <div className={styles.cardFoot}>
        <span className={styles.who} title={item.createdByEmail || undefined}>{item.createdByEmail || 'Autore non disponibile'}</span>
        <span className={styles.when}>{formatInstant(item.updatedAt)}</span>
      </div>
    </div>
  );
});

function DraggableCard({ item, onOpen }: { item: Segnalazione; onOpen: (item: Segnalazione) => void }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: item.id });
  return (
    <SegnalazioneCard
      ref={setNodeRef}
      item={item}
      dragging={isDragging}
      {...attributes}
      {...listeners}
      onClick={() => onOpen(item)}
      // Invio apre il dettaglio; Spazio e frecce restano al KeyboardSensor.
      onKeyDown={(event) => {
        if (event.key === 'Enter') {
          event.preventDefault();
          onOpen(item);
          return;
        }
        listeners?.onKeyDown?.(event);
      }}
    />
  );
}

function Column({ state, items, onOpen }: { state: SegnalazioneState; items: Segnalazione[]; onOpen: (item: Segnalazione) => void }) {
  const { setNodeRef, isOver } = useDroppable({ id: `${DROP_PREFIX}${state}` });
  const vars = segnalazioneStateVars(state) as CSSProperties;
  return (
    <div ref={setNodeRef} className={[board.kcol, isOver ? board.dropTarget : ''].filter(Boolean).join(' ')} style={vars}>
      <div className={board.kcolHead}>
        <span className={board.dot} />
        <span className={board.nm}>{segnalazioneStateLabel(state)}</span>
        <span className={board.cnt}>{items.length}</span>
      </div>
      <div className={board.kcolBody}>
        {items.map((item) => <DraggableCard key={item.id} item={item} onOpen={onOpen} />)}
        {items.length === 0 ? <div className={board.colEmpty}>Nessuna segnalazione</div> : null}
      </div>
    </div>
  );
}

/** Kanban a quattro colonne con trascinamento libero fra tutte. Nessuno stato
 *  ottimistico: `onMove` chiama l'API e la lista invalidata riallinea la vista. */
export function SegnalazioniBoard({ items, onOpen, onMove }: {
  items: Segnalazione[];
  onOpen: (item: Segnalazione) => void;
  onMove: (item: Segnalazione, state: SegnalazioneState) => void;
}) {
  const [activeId, setActiveId] = useState<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor),
  );

  const byState = useMemo(() => {
    const map = new Map<SegnalazioneState, Segnalazione[]>(SEGNALAZIONE_STATES.map((s) => [s.key, []]));
    for (const item of items) map.get(item.state)?.push(item);
    return map;
  }, [items]);

  const active = activeId ? items.find((item) => item.id === activeId) ?? null : null;

  const a11y = useMemo(() => {
    const nameOf = (rawId: unknown) => {
      const found = items.find((item) => item.id === String(rawId));
      return found ? segnalazioneTitle(found) : 'segnalazione';
    };
    const stateOf = (rawId: unknown) => segnalazioneStateLabel(String(rawId).replace(DROP_PREFIX, ''));
    const announcements: Announcements = {
      onDragStart: ({ active }) => `Presa ${nameOf(active.id)}.`,
      onDragOver: ({ active, over }) => (over ? `${nameOf(active.id)} sopra ${stateOf(over.id)}.` : `${nameOf(active.id)} fuori da ogni colonna.`),
      onDragEnd: ({ active, over }) => (over ? `${nameOf(active.id)} rilasciata in ${stateOf(over.id)}.` : `${nameOf(active.id)} rilasciata fuori: nessuna modifica.`),
      onDragCancel: ({ active }) => `Spostamento di ${nameOf(active.id)} annullato.`,
    };
    const screenReaderInstructions: ScreenReaderInstructions = {
      draggable: 'Premi Spazio per prendere la segnalazione, le frecce per spostarla fra le colonne, Spazio per rilasciare, Esc per annullare. Invio apre il dettaglio.',
    };
    return { announcements, screenReaderInstructions };
  }, [items]);

  const onDragStart = (event: DragStartEvent) => setActiveId(String(event.active.id));
  const onDragEnd = (event: DragEndEvent) => {
    setActiveId(null);
    const { active, over } = event;
    if (!over) return;
    const overId = String(over.id);
    if (!overId.startsWith(DROP_PREFIX)) return;
    const target = overId.slice(DROP_PREFIX.length) as SegnalazioneState;
    const item = items.find((candidate) => candidate.id === String(active.id));
    if (!item || item.state === target) return;
    onMove(item, target);
  };

  return (
    <DndContext sensors={sensors} accessibility={a11y} collisionDetection={pointerWithin} onDragStart={onDragStart} onDragEnd={onDragEnd} onDragCancel={() => setActiveId(null)}>
      <div className={board.cols}>
        {SEGNALAZIONE_STATES.map((state) => (
          <Column key={state.key} state={state.key} items={byState.get(state.key) ?? []} onOpen={onOpen} />
        ))}
      </div>
      <DragOverlay dropAnimation={null}>
        {active ? (
          <div className={board.dragOverlay}>
            <SegnalazioneCard item={active} />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
