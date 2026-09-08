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
import {
  SEGNALAZIONE_STATES,
  SEGNALAZIONE_TERMINAL_PREVIEW,
  isSegnalazioneTerminal,
  segnalazioneStateLabel,
  segnalazioneStateVars,
  segnalazioneTitle,
} from '../../lib/segnalazioneStates';
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

interface ColumnProps {
  state: SegnalazioneState;
  items: Segnalazione[];
  collapsed: boolean;
  /** Ricerca attiva: le colonne terminali mostrano tutte le corrispondenze. */
  searching: boolean;
  onToggle: (state: SegnalazioneState) => void;
  onOpen: (item: Segnalazione) => void;
  onShowAll: (state: SegnalazioneState) => void;
}

/** Colonna della lavagna. Gli stati terminali partono compressi a barra (che
 *  resta bersaglio del rilascio) e, espansi, mostrano solo le più recenti con
 *  un rimando alla tabella per lo storico completo. */
function Column({ state, items, collapsed, searching, onToggle, onOpen, onShowAll }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: `${DROP_PREFIX}${state}` });
  const vars = segnalazioneStateVars(state) as CSSProperties;
  const label = segnalazioneStateLabel(state);
  const terminal = isSegnalazioneTerminal(state);

  if (collapsed) {
    return (
      <button
        ref={setNodeRef}
        type="button"
        className={[board.krail, isOver ? board.dropTarget : ''].filter(Boolean).join(' ')}
        style={vars}
        onClick={() => onToggle(state)}
        aria-label={`Espandi ${label}, ${items.length} segnalazioni`}
      >
        <span className={board.rcount}>{items.length}</span>
        <span className={board.rlabel}>{label}</span>
        <Icon name="chevron-right" size={14} className={board.rchev} />
      </button>
    );
  }

  const limited = terminal && !searching && items.length > SEGNALAZIONE_TERMINAL_PREVIEW;
  const visible = limited ? items.slice(0, SEGNALAZIONE_TERMINAL_PREVIEW) : items;

  return (
    <div ref={setNodeRef} className={[board.kcol, isOver ? board.dropTarget : ''].filter(Boolean).join(' ')} style={vars}>
      <div className={board.kcolHead}>
        <span className={board.dot} />
        <span className={board.nm}>{label}</span>
        <span className={board.cnt} aria-label={limited ? `${visible.length} di ${items.length}` : undefined}>
          {limited ? `${visible.length} di ${items.length}` : items.length}
        </span>
        {terminal ? (
          <button type="button" className={board.railBtn} onClick={() => onToggle(state)} aria-label={`Comprimi ${label}`}>
            <Icon name="chevron-left" size={14} />
          </button>
        ) : null}
      </div>
      <div className={board.kcolBody}>
        {limited ? (
          <button type="button" className={styles.showAll} onClick={() => onShowAll(state)}>
            Vedi tutte le {items.length} in tabella
          </button>
        ) : null}
        {visible.map((item) => <DraggableCard key={item.id} item={item} onOpen={onOpen} />)}
        {items.length === 0 ? <div className={board.colEmpty}>Nessuna segnalazione</div> : null}
      </div>
    </div>
  );
}

/** Kanban a quattro colonne con trascinamento libero fra tutte. Nessuno stato
 *  ottimistico: `onMove` chiama l'API e la lista invalidata riallinea la vista. */
export function SegnalazioniBoard({ items, searching, onOpen, onMove, onShowAll }: {
  items: Segnalazione[];
  searching: boolean;
  onOpen: (item: Segnalazione) => void;
  onMove: (item: Segnalazione, state: SegnalazioneState) => void;
  onShowAll: (state: SegnalazioneState) => void;
}) {
  const [activeId, setActiveId] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState<Set<SegnalazioneState>>(
    () => new Set(SEGNALAZIONE_STATES.filter((s) => s.terminal).map((s) => s.key)),
  );
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor),
  );

  const toggle = (state: SegnalazioneState) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(state)) next.delete(state);
      else next.add(state);
      return next;
    });

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
          <Column
            key={state.key}
            state={state.key}
            items={byState.get(state.key) ?? []}
            collapsed={collapsed.has(state.key)}
            searching={searching}
            onToggle={toggle}
            onOpen={onOpen}
            onShowAll={onShowAll}
          />
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
