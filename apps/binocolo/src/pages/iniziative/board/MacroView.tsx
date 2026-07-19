import { type CSSProperties } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { Icon } from '@mrsmith/ui';
import type { MAInitiativeCardView } from '../../../api/types';
import { type MacrofaseMeta, CARD_STATES, stateVars, macroVars } from '../../../lib/cardStates';
import { BoardCard, DraggableCard } from './BoardCard';
import type { BoardViewProps } from './StatesView';
import styles from './board.module.css';

export interface MacroViewProps extends Omit<BoardViewProps, 'visibleStates'> {
  visibleMacrofasi: MacrofaseMeta[];
  /** Isola lo stato in una colonna dedicata (→ vista Tutti gli stati a fuoco). */
  onIsolate?: (state: string) => void;
}

function StateSection({
  stateKey,
  cards,
  collapsed,
  onToggleCollapse,
  onCardClick,
  onOpenDossier,
  onAdd,
  onIsolate,
  readOnly,
  initiativeChip,
}: {
  stateKey: string;
  cards: MAInitiativeCardView[];
  collapsed: boolean;
} & Pick<MacroViewProps, 'onToggleCollapse' | 'onCardClick' | 'onOpenDossier' | 'onAdd' | 'onIsolate' | 'readOnly' | 'initiativeChip'>) {
  const meta = CARD_STATES.find((s) => s.key === stateKey);
  const { setNodeRef, isOver } = useDroppable({ id: `state:${stateKey}` });
  if (!meta) return null;
  // I terminali si raggiungono da /close e si escono da /reopen: le loro card non
  // sono trascinabili (un drag fallirebbe la guardia terminale del backend).
  const draggable = !readOnly && !meta.terminal;

  return (
    <div
      ref={setNodeRef}
      className={[styles.substate, collapsed ? styles.collapsed : '', isOver ? styles.dropTarget : ''].filter(Boolean).join(' ')}
      style={stateVars(stateKey) as CSSProperties}
    >
      <div className={[styles.sh, isOver ? styles.dropTarget : ''].filter(Boolean).join(' ')}>
        <span className={styles.snm}>{meta.label}</span>
        <span className={styles.scnt}>{cards.length}</span>
        <span className={styles.shActs}>
          <button
            type="button"
            className={[styles.iconbtn, styles.chev].join(' ')}
            onClick={() => onToggleCollapse(stateKey)}
            aria-label={collapsed ? `Espandi ${meta.label}` : `Comprimi ${meta.label}`}
          >
            <Icon name="chevron-down" size={13} />
          </button>
          {!readOnly && onAdd ? (
            <button type="button" className={styles.iconbtn} onClick={() => onAdd(stateKey)} aria-label="Aggiungi azienda in questo stato">
              <Icon name="plus" size={13} />
            </button>
          ) : null}
          {onIsolate ? (
            <button type="button" className={styles.iconbtn} onClick={() => onIsolate(stateKey)} aria-label="Isola questo stato in una colonna">
              <Icon name="external-link" size={13} />
            </button>
          ) : null}
        </span>
      </div>
      {!collapsed ? (
        <>
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
          {cards.length === 0 && !isOver ? <div className={styles.substateEmpty}>Nessuna azienda in questo stato</div> : null}
        </>
      ) : null}
    </div>
  );
}

/** Vista Macrofasi = kanban, colonne = macrofasi a tutta larghezza (flex:1). Dentro
 *  ogni colonna gli stati come sotto-sezioni collassabili (accordion verticale),
 *  ciascuna drop-zone. È la vista di lavoro principale (default d'ingresso). */
export function MacroView({
  visibleMacrofasi,
  cardsByState,
  collapsed,
  onToggleCollapse,
  onCardClick,
  onOpenDossier,
  onAdd,
  onIsolate,
  readOnly,
  initiativeChip,
}: MacroViewProps) {
  return (
    <div className={styles.mcols}>
      {visibleMacrofasi.map((macro) => {
        const count = macro.states.reduce((sum, sk) => sum + (cardsByState.get(sk)?.length ?? 0), 0);
        return (
          <div key={macro.key} className={styles.mcol}>
            <div className={styles.mcolHead} style={macroVars(macro.key) as CSSProperties}>
              <span className={styles.num}>{macro.num}</span>
              <div>
                <div className={styles.mt}>{macro.label}</div>
              </div>
              <span className={styles.mcnt}>{count}</span>
            </div>
            <div className={styles.mcolBody}>
              {macro.states.map((sk) => (
                <StateSection
                  key={sk}
                  stateKey={sk}
                  cards={cardsByState.get(sk) ?? []}
                  collapsed={collapsed.has(sk)}
                  onToggleCollapse={onToggleCollapse}
                  onCardClick={onCardClick}
                  onOpenDossier={onOpenDossier}
                  onAdd={onAdd}
                  onIsolate={onIsolate}
                  readOnly={readOnly}
                  initiativeChip={initiativeChip}
                />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
