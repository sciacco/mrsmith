import { forwardRef, useState, type HTMLAttributes } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { DndContext, DragOverlay, KeyboardSensor, PointerSensor, pointerWithin, rectIntersection, useDraggable, useDroppable, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import { Button, Icon, SearchInput, Skeleton } from '@mrsmith/ui';
import { formatLocalDate, formatNumber } from '@mrsmith/format';
import { useNeeds, useSaveNeed } from '../../api/queries';
import type { Need } from '../../api/types';
import { NEED_STATES, needInput } from '../../lib/needs';
import { NeedEditorModal } from '../../components/needs/NeedEditorModal';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { describeApiError } from '../../components/events/apiErrors';
// Struttura, stili e drag-and-drop della board di Binocolo.
import board from '../../../../binocolo/src/pages/iniziative/board/board.module.css';
import styles from './NeedsPage.module.css';

const DROP_PREFIX = 'need-state:';

const NeedCard = forwardRef<HTMLElement, { need: Need; overlay?: boolean } & HTMLAttributes<HTMLElement>>(
  function NeedCard({ need, overlay, className = '', ...props }, ref) {
    return (
      <article ref={ref} className={`${board.kcard} ${styles.card} ${styles[need.status]} ${className}`} {...props}>
        <div className={board.cardName}>
          {overlay ? <span className={styles.cardTitle}>{need.description}</span> : <Link to={`/esigenze/${need.id}`} className={styles.cardTitle} draggable={false}>{need.description}</Link>}
          <Icon name="grip-vertical" size={14} className={board.grab} />
        </div>
        <p className={styles.meta}>{formatNumber(need.candidatesCount)} candidati · {formatNumber(need.requestsCount)} richieste</p>
        {need.finalCourseTitle && <p className={styles.meta}>Definitivo: {need.finalCourseTitle}</p>}
        {need.reminderText && <p className={styles.meta}>{need.reminderAt ? `${formatLocalDate(need.reminderAt)} · ` : ''}{need.reminderText}</p>}
      </article>
    );
  },
);

function DraggableNeed({ need, disabled }: { need: Need; disabled: boolean }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: need.id, disabled });
  return <NeedCard ref={setNodeRef} id={`need-${need.id}`} need={need} className={isDragging ? board.dragging : ''} {...attributes} {...listeners} aria-label={`Sposta ${need.description}`} />;
}

function NeedColumn({ state, items, disabled }: { state: typeof NEED_STATES[number]; items: Need[]; disabled: boolean }) {
  const { setNodeRef, isOver } = useDroppable({ id: `${DROP_PREFIX}${state.value}` });
  return (
    <section ref={setNodeRef} className={`${board.kcol} ${styles.column} ${styles[state.value]} ${isOver ? board.dropTarget : ''}`} aria-label={state.label}>
      <div className={board.kcolHead}>
        <span className={board.dot} aria-hidden="true" />
        <span className={board.nm}>{state.label}</span>
        <span className={board.cnt}>{formatNumber(items.length)}</span>
      </div>
      <div className={board.kcolBody}>
        {items.map((need) => <DraggableNeed key={need.id} need={need} disabled={disabled} />)}
        {!items.length && <p className={styles.meta}>Nessuna esigenza</p>}
      </div>
    </section>
  );
}

export function NeedsPage() {
  const needs = useNeeds();
  const save = useSaveNeed();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor),
  );
  const activeNeed = (needs.data ?? []).find((need) => need.id === activeId);
  const stateLabel = (id: string | number) => NEED_STATES.find((state) => `${DROP_PREFIX}${state.value}` === String(id))?.label ?? '';
  const visible = (needs.data ?? []).filter((n) =>
    `${n.description} ${n.notes ?? ''} ${n.skillAreas.map((a) => a.name).join(' ')}`.toLocaleLowerCase('it').includes(query.toLocaleLowerCase('it')),
  );

  async function move({ active, over }: DragEndEvent) {
    setActiveId(null);
    const need = (needs.data ?? []).find((item) => item.id === active.id);
    const state = NEED_STATES.find((state) => `${DROP_PREFIX}${state.value}` === over?.id);
    if (!need || !state || state.value === need.status || save.isPending) return;
    setError(null);
    try { await save.mutateAsync({ id: need.id, input: { ...needInput(need), status: state.value } }); }
    catch (e) { setError(describeApiError(e, 'Cambio stato non riuscito')); }
    requestAnimationFrame(() => document.getElementById(`need-${need.id}`)?.focus());
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>Esigenze formative</h1>
        <Button onClick={() => setCreating(true)} leftIcon={<Icon name="plus" size={16} />}>Nuova esigenza</Button>
      </header>
      {needs.isPending ? <Skeleton rows={5} /> : needs.isError ? (
        <div><ErrorPanel message={describeApiError(needs.error, 'Esigenze non disponibili')} /><Button variant="secondary" onClick={() => void needs.refetch()}>Riprova</Button></div>
      ) : (needs.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}><Icon name="target" size={32} /></div>
          <strong>Nessuna esigenza formativa</strong>
          <p className={styles.meta}>Crea un’esigenza e raccogli i corsi candidati. Puoi collegare le richieste in seguito.</p>
          <Button onClick={() => setCreating(true)}>Nuova esigenza</Button>
        </div>
      ) : (
        <>
          <SearchInput value={query} onChange={setQuery} placeholder="Cerca esigenza o area…" />
          {error && <ErrorPanel message={error} />}
          <DndContext
            sensors={sensors}
            collisionDetection={(args) => args.pointerCoordinates ? pointerWithin(args) : rectIntersection(args)}
            onDragStart={({ active }) => setActiveId(String(active.id))}
            onDragEnd={(event) => void move(event)}
            onDragCancel={() => setActiveId(null)}
            accessibility={{
              screenReaderInstructions: { draggable: 'Premi Spazio per prendere l’esigenza, le frecce per spostarla, Spazio per rilasciarla, Escape per annullare.' },
              announcements: {
                onDragStart: ({ active }) => `Presa ${(needs.data ?? []).find((need) => need.id === active.id)?.description ?? 'esigenza'}.`,
                onDragOver: ({ over }) => over ? `Sopra ${stateLabel(over.id)}.` : 'Fuori da ogni colonna.',
                onDragEnd: ({ over }) => over ? `Rilascio in ${stateLabel(over.id)}.` : 'Nessuna modifica.',
                onDragCancel: () => 'Spostamento annullato.',
              },
            }}
          >
            <div className={`${board.cols} ${styles.columns}`}>
              {NEED_STATES.map((state) => <NeedColumn key={state.value} state={state} items={visible.filter((need) => need.status === state.value)} disabled={save.isPending} />)}
            </div>
            <DragOverlay dropAnimation={null}>
              {activeNeed && <div className={board.dragOverlay} aria-hidden="true"><NeedCard need={activeNeed} overlay /></div>}
            </DragOverlay>
          </DndContext>
          {query && !visible.length && <Button variant="secondary" onClick={() => setQuery('')}>Cancella ricerca</Button>}
        </>
      )}
      {creating && <NeedEditorModal onClose={() => setCreating(false)} onCreated={(id) => navigate(`/esigenze/${id}`)} />}
    </div>
  );
}
