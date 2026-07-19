import { useEffect, type RefObject } from 'react';
import { ACTIVE_STATES } from '../../../lib/cardStates';
import type { TerminalTarget } from './TerminalModal';

/** Scorciatoie della board (KANBAN-V2-PLAN.md §6.2). `/` focus ricerca; j/k e frecce
 *  spostano il focus fra le card ([data-flip-id], rese focusabili da @dnd-kit);
 *  Enter apre il drawer; 1..7 spostano la card a fuoco nello stato n-esimo
 *  non-terminale; w/x aprono la modale terminale; c comprime la colonna/stato; ? il
 *  popover aiuto. Il drag-da-tastiera vero (Space+frecce) è del KeyboardSensor. */
export function useBoardKeyboard(opts: {
  searchRef: RefObject<HTMLInputElement | null>;
  enabled: boolean;
  onOpenCard: (companyKey: string) => void;
  onMoveToState: (companyKey: string, stateKey: string) => void;
  onTerminal: (companyKey: string, target: TerminalTarget) => void;
  onToggleCollapse: (stateKey: string) => void;
  onToggleHelp: () => void;
}) {
  const { searchRef, enabled, onOpenCard, onMoveToState, onTerminal, onToggleCollapse, onToggleHelp } = opts;

  useEffect(() => {
    if (!enabled) return;

    function focusedCard(): HTMLElement | null {
      const el = document.activeElement as HTMLElement | null;
      return el && el.dataset && el.dataset.flipId ? el : null;
    }
    function cardList(): HTMLElement[] {
      return Array.from(document.querySelectorAll<HTMLElement>('[data-flip-id]'));
    }
    function move(delta: number) {
      const cards = cardList();
      if (cards.length === 0) return;
      const cur = focusedCard();
      const idx = cur ? cards.indexOf(cur) : -1;
      const next = cards[Math.max(0, Math.min(cards.length - 1, idx + delta))];
      next?.focus();
    }

    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null;
      const typing =
        target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable);

      if (e.key === '/' && !typing) {
        e.preventDefault();
        searchRef.current?.focus();
        return;
      }
      if (e.key === '?' && !typing) {
        e.preventDefault();
        onToggleHelp();
        return;
      }
      if (typing) return;

      const card = focusedCard();
      const companyKey = card?.dataset.flipId;

      switch (e.key) {
        case 'j':
        case 'ArrowDown':
          e.preventDefault();
          move(1);
          break;
        case 'k':
        case 'ArrowUp':
          e.preventDefault();
          move(-1);
          break;
        case 'Enter':
          if (companyKey) {
            e.preventDefault();
            onOpenCard(companyKey);
          }
          break;
        case 'w':
          if (companyKey) onTerminal(companyKey, 'won');
          break;
        case 'x':
          if (companyKey) onTerminal(companyKey, 'ko_target');
          break;
        case 'c':
          if (card?.dataset.state) onToggleCollapse(card.dataset.state);
          break;
        default:
          if (companyKey && /^[1-7]$/.test(e.key)) {
            const st = ACTIVE_STATES[Number(e.key) - 1];
            if (st) {
              onMoveToState(companyKey, st.key);
              // La card si ri-parenta fra colonne (perde il focus DOM): lo
              // ripristiniamo dopo il commit optimistic, così j/k continua da lì.
              requestAnimationFrame(() => {
                Array.from(document.querySelectorAll<HTMLElement>('[data-flip-id]'))
                  .find((n) => n.dataset.flipId === companyKey)
                  ?.focus();
              });
            }
          }
      }
    }

    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [enabled, searchRef, onOpenCard, onMoveToState, onTerminal, onToggleCollapse, onToggleHelp]);
}
