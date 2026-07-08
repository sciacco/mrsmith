import { useCallback, useEffect, useState } from 'react';

// Bordo inferiore "da incollata" di una barra sticky: il suo `top` CSS più
// l'altezza corrente (il rect al momento del click può non essere ancora
// nello stato stuck).
function stickyBottom(el: HTMLElement | null): number {
  if (!el) return 0;
  const top = Number.parseFloat(getComputedStyle(el).top) || 0;
  return top + el.getBoundingClientRect().height;
}

/**
 * Scroll-spy sui blocchi della scheda: osserva gli heading id già in pagina e
 * restituisce quello attivo, più uno scrollTo che rispetta prefers-reduced-motion.
 * L'osservatore si (ri)attiva quando `enabled` diventa vero, così cattura i
 * blocchi montati dopo il caricamento dei dati.
 */
export function useSectionSpy(ids: readonly string[], enabled: boolean) {
  const [activeId, setActiveId] = useState<string | undefined>(ids[0]);
  const idsKey = ids.join('|');

  useEffect(() => {
    if (!enabled) return;
    const ordered = idsKey.split('|');
    const targets = ordered
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el != null);
    if (targets.length === 0) return;

    const visible = new Set<string>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) visible.add(entry.target.id);
          else visible.delete(entry.target.id);
        }
        const current = ordered.find((id) => visible.has(id));
        if (current) setActiveId(current);
      },
      // banda stretta centro-alto per non far sfarfallare blocchi di altezza molto diversa
      { rootMargin: '-40% 0px -55% 0px', threshold: 0 },
    );
    targets.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [idsKey, enabled]);

  const scrollTo = useCallback((id: string) => {
    const el = document.getElementById(id);
    if (!el) return;
    const target = (el.closest('section') ?? el) as HTMLElement;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    // Offset misurato a runtime: la LensBar può andare su più righe e su
    // <=1000px anche la spina orizzontale resta sticky sotto di lei — un
    // margine fisso lascerebbe la prima riga del blocco coperta.
    const bar = document.querySelector<HTMLElement>('[data-scheda-sticky="bar"]');
    const spine = document.querySelector<HTMLElement>('[data-scheda-sticky="spine"]');
    const horizontalSpine = window.matchMedia('(max-width: 1000px)').matches ? spine : null;
    const offset = Math.max(stickyBottom(bar), stickyBottom(horizontalSpine), 128) + 12;
    const top = target.getBoundingClientRect().top + window.scrollY - offset;
    window.scrollTo({ top: Math.max(top, 0), behavior: reduced ? 'auto' : 'smooth' });
    setActiveId(id);
  }, []);

  return { activeId, scrollTo };
}
