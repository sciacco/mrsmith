// FLIP (First · Last · Invert · Play) su Web Animations API. Nessuna libreria di
// animazione (il monorepo non ne ha; i token motion di UI-UX §8.1 bastano).
// Misura la posizione dei nodi PRIMA di una mutazione del layout, poi anima il
// delta con `transform` dopo il commit. Usi: cambio vista Macrofasi↔Tutti gli
// stati (il momento-firma), collasso colonna/stato, riordino post-optimistic.
// Rispetta `prefers-reduced-motion` (nessuna animazione).

import { useLayoutEffect, useRef, type RefObject } from 'react';

export type FlipSnapshot = Map<string, DOMRect>;

// cubic-bezier di --ease-out (clean theme). WAAPI non legge le CSS custom props,
// quindi il valore è replicato qui (unica ricetta motion fuori dai token, §8.1).
const EASE_OUT = 'cubic-bezier(0.16, 1, 0.3, 1)';

function reducedMotion(): boolean {
  return typeof window !== 'undefined' && Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches);
}

/** Misura la posizione dei nodi flippabili (`[data-flip-id]`) dentro `root`. */
export function measureFlip(root: ParentNode): FlipSnapshot {
  const snap: FlipSnapshot = new Map();
  root.querySelectorAll<HTMLElement>('[data-flip-id]').forEach((el) => {
    const id = el.dataset.flipId;
    if (id) snap.set(id, el.getBoundingClientRect());
  });
  return snap;
}

/** Dopo che il DOM è passato alla nuova posizione (Last), inverte dal First e
 *  riproduce la transizione su ogni nodo ancora presente. I nodi nuovi (assenti
 *  dallo snapshot) NON vengono flippati: il loro enter è gestito via CSS. */
export function playFlip(first: FlipSnapshot, root: ParentNode, duration = 400): void {
  if (reducedMotion()) return;
  root.querySelectorAll<HTMLElement>('[data-flip-id]').forEach((el) => {
    const id = el.dataset.flipId;
    if (!id) return;
    const prev = first.get(id);
    if (!prev) return;
    const next = el.getBoundingClientRect();
    const dx = prev.left - next.left;
    const dy = prev.top - next.top;
    // Morph di forma: sul cambio vista la card cambia larghezza (macro ~370px ↔
    // stato 258px). Oltre a translate, compensiamo la scala così il box si
    // trasforma invece di scattare (transform-origin top-left, sotto).
    const sx = next.width > 0 ? prev.width / next.width : 1;
    const sy = next.height > 0 ? prev.height / next.height : 1;
    const moved = Math.abs(dx) >= 0.5 || Math.abs(dy) >= 0.5;
    const scaled = Math.abs(sx - 1) >= 0.01 || Math.abs(sy - 1) >= 0.01;
    if (!moved && !scaled) return;
    el.animate(
      [
        { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})`, transformOrigin: 'top left' },
        { transform: 'translate(0, 0) scale(1, 1)', transformOrigin: 'top left' },
      ],
      { duration, easing: EASE_OUT, fill: 'none' },
    );
  });
}

/** Anima con FLIP i nodi `[data-flip-id]` dentro `containerRef` ogni volta che
 *  `key` cambia (es. la vista attiva, o una signature di ordine/conteggi). La
 *  misura "First" è lo snapshot preso al termine dell'effetto precedente: quando
 *  l'effetto rigira dopo il commit della nuova `key`, il DOM è già nella nuova
 *  posizione e `first` porta ancora la vecchia → invert corretto. */
export function useFlip(containerRef: RefObject<HTMLElement | null>, key: unknown, duration = 400): void {
  const firstRef = useRef<FlipSnapshot | null>(null);
  const keyRef = useRef<unknown>(key);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (firstRef.current && keyRef.current !== key) {
      playFlip(firstRef.current, container, duration);
    }
    // Snapshot della posizione corrente per il prossimo cambio.
    firstRef.current = measureFlip(container);
    keyRef.current = key;
  });
}
