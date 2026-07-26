import { useEffect, useLayoutEffect, useRef } from 'react';

/**
 * Restores focus to the element that opened a native `<dialog>` overlay.
 *
 * `showModal()` remembers the previously focused element and `close()` restores
 * it, but that step only runs on `close()`. A dialog unmounted while still open
 * (`{open ? <Drawer open … /> : null}`) never reaches it and drops focus to
 * `<body>`. This hook covers both paths. See docs/UI-UX.md §16.
 */
export function useFocusRestore(open: boolean) {
  const triggerRef = useRef<HTMLElement | null>(null);

  // Layout phase: capture the trigger before the passive effect that calls
  // showModal() can move focus into the dialog.
  useLayoutEffect(() => {
    if (!open) return;
    const active = document.activeElement;
    if (!(active instanceof HTMLElement) || active === document.body) return;
    // Never capture something living inside an open dialog. This effect can run
    // again while the overlay is up — React StrictMode remounts effects in dev,
    // and overlays can stack — at which point focus is already on the overlay's
    // own close button, which dies with it. Keep the earlier, valid trigger.
    if (active.closest('dialog[open]')) return;
    triggerRef.current = active;
  }, [open]);

  useEffect(() => {
    if (!open) return;
    return () => {
      // Deliberately not cleared: this cleanup also runs on the remount React
      // StrictMode simulates in dev, and dropping the trigger there would leave
      // nothing to restore on the real unmount. The next open overwrites it.
      const trigger = triggerRef.current;
      if (!trigger) return;
      // On unmount React runs this cleanup during the commit's mutation phase,
      // while the <dialog> is still mounted and modal — the rest of the page is
      // inert and focus() is silently ignored. Defer past the commit so the
      // dialog has actually left the DOM.
      queueMicrotask(() => {
        // The trigger may have unmounted with the view that owned it; leave
        // focus where the browser put it rather than guessing a replacement.
        if (!trigger.isConnected) return;
        // On the close() path the native restore already ran. Only step in when
        // focus was genuinely dropped, never steal it back from elsewhere.
        const active = document.activeElement;
        if (active && active !== document.body) return;
        trigger.focus();
      });
    };
  }, [open]);
}
