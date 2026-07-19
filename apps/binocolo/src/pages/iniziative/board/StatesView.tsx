import { type CSSProperties } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { Icon } from '@mrsmith/ui';
import type { MAInitiativeCardView } from '../../../api/types';
import { type CardStateMeta, stateVars } from '../../../lib/cardStates';
import { BoardCard, DraggableCard } from './BoardCard';
import styles from './board.module.css';

export interface BoardViewProps {
  visibleStates: CardStateMeta[];
  cardsByState: Map<string, MAInitiativeCardView[]>;
  collapsed: Set<string>;
  onToggleCollapse: (state: string) => void;
  onCardClick: (card: MAInitiativeCardView) => void;
  onOpenDossier: (card: MAInitiativeCardView) => void;
  onAdd?: (state: string) => void;
  readOnly?: boolean;
  initiativeChip?: (card: MAInitiativeCardView) => string | undefined;
}

function Column({
  state,
  cards,
  collapsed,
  onToggleCollapse,
  onCardClick,
  onOpenDossier,
  onAdd,
  readOnly,
  initiativeChip,
}: {
  state: CardStateMeta;
  cards: MAInitiativeCardView[];
} & Omit<BoardViewProps, 'visibleStates' | 'cardsByState' | 'collapsed'> & { collapsed: boolean }) {
  const { setNodeRef, isOver } = useDroppable({ id: `state:${state.key}` });
  const vars = stateVars(state.key) as CSSProperties;
  // I terminali non si trascinano (uscita solo via /reopen).
  const draggable = !readOnly && !state.terminal;

  if (collapsed) {
    return (
      <button
        ref={setNodeRef}
        className={[styles.krail, isOver ? styles.dropTarget : ''].filter(Boolean).join(' ')}
        style={vars}
        onClick={() => onToggleCollapse(state.key)}
        aria-label={`Espandi ${state.label}`}
      >
        <span className={styles.rcount}>{cards.length}</span>
        <span className={styles.rlabel}>{state.label}</span>
        <Icon name="chevron-right" size={14} className={styles.rchev} />
      </button>
    );
  }

  return (
    <div ref={setNodeRef} className={[styles.kcol, isOver ? styles.dropTarget : ''].filter(Boolean).join(' ')} style={vars}>
      <div className={styles.kcolHead}>
        <span className={styles.dot} />
        <span className={styles.nm}>{state.label}</span>
        <span className={styles.cnt}>{cards.length}</span>
        <button
          type="button"
          className={styles.railBtn}
          onClick={() => onToggleCollapse(state.key)}
          aria-label={`Comprimi ${state.label}`}
        >
          <Icon name="chevron-left" size={14} />
        </button>
      </div>
      <div className={styles.kcolBody}>
        {cards.map((card) =>
          !draggable ? (
            <BoardCard
              key={`${card.initiativeId}:${card.companyKey}`}
              card={card}
              onClick={() => onCardClick(card)}
              onOpenDossier={onOpenDossier}
              initiativeChip={initiativeChip?.(card)}
              style={{ cursor: 'pointer' }}
            />
          ) : (
            <DraggableCard
              key={`${card.initiativeId}:${card.companyKey}`}
              card={card}
              onClick={() => onCardClick(card)}
              onOpenDossier={onOpenDossier}
              initiativeChip={initiativeChip?.(card)}
            />
          ),
        )}
        {isOver && !readOnly ? <div className={styles.placeholder} /> : null}
        {cards.length === 0 && !isOver ? <div className={styles.colEmpty}>Nessuna azienda</div> : null}
        {!readOnly && !state.terminal && onAdd ? (
          <button type="button" className={styles.kadd} style={vars} onClick={() => onAdd(state.key)}>
            <Icon name="plus" size={13} />
            Aggiungi azienda
          </button>
        ) : null}
      </div>
    </div>
  );
}

/** Vista "Tutti gli stati": una colonna per stato, collassabile a rail (anche il
 *  rail è drop-zone). Droppable id = `state:<chiave>`. */
export function StatesView({
  visibleStates,
  cardsByState,
  collapsed,
  onToggleCollapse,
  onCardClick,
  onOpenDossier,
  onAdd,
  readOnly,
  initiativeChip,
}: BoardViewProps) {
  return (
    <div className={styles.cols}>
      {visibleStates.map((state) => (
        <Column
          key={state.key}
          state={state}
          cards={cardsByState.get(state.key) ?? []}
          collapsed={collapsed.has(state.key)}
          onToggleCollapse={onToggleCollapse}
          onCardClick={onCardClick}
          onOpenDossier={onOpenDossier}
          onAdd={onAdd}
          readOnly={readOnly}
          initiativeChip={initiativeChip}
        />
      ))}
    </div>
  );
}
