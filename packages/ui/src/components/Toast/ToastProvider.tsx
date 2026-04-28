import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import styles from './Toast.module.css';

type ToastType = 'success' | 'error' | 'warning';

interface Toast {
  id: number;
  message: string;
  type: ToastType;
}

interface ToastContextValue {
  toast: (message: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let nextId = 0;

type ToastViewportElement = HTMLDivElement & {
  showPopover?: () => void;
  hidePopover?: () => void;
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const viewportRef = useRef<ToastViewportElement | null>(null);

  const addToast = useCallback((message: string, type: ToastType = 'success') => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
  }, []);

  const portalTarget = getToastPortalTarget();

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;

    viewport.setAttribute('popover', 'manual');

    if (!viewport.showPopover || !viewport.hidePopover) return;

    if (toasts.length === 0) {
      if (isPopoverOpen(viewport)) {
        viewport.hidePopover();
      }
      return;
    }

    if (!isPopoverOpen(viewport)) {
      try {
        viewport.showPopover();
      } catch {
        // The fixed viewport still renders in browsers without stable Popover behavior.
      }
    }
  }, [portalTarget, toasts.length]);

  const toastViewport = (
    <div ref={viewportRef} className={styles.container} aria-live="polite">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`${styles.toast} ${styles[t.type]}`}
          role="alert"
        >
          {t.type === 'success' ? (
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
              <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.5" />
              <path d="M6 9l2 2 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          ) : t.type === 'warning' ? (
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
              <path d="M8.136 3.5a1 1 0 0 1 1.728 0l5.17 9A1 1 0 0 1 14.17 14H3.83a1 1 0 0 1-.864-1.5l5.17-9Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
              <path d="M9 6.8v3.3M9 12.2h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          ) : (
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
              <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.5" />
              <path d="M9 6v4M9 12.5v.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          )}
          {t.message}
        </div>
      ))}
    </div>
  );

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {portalTarget ? createPortal(toastViewport, portalTarget) : toastViewport}
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within <ToastProvider>');
  return ctx;
}

function getToastPortalTarget(): HTMLElement | null {
  if (typeof document === 'undefined') return null;

  const focusedDialog = document.activeElement instanceof Element
    ? document.activeElement.closest<HTMLDialogElement>('dialog[open]')
    : null;
  if (focusedDialog) return focusedDialog;

  const openDialogs = document.querySelectorAll<HTMLDialogElement>('dialog[open]');
  return openDialogs[openDialogs.length - 1] ?? document.body;
}

function isPopoverOpen(element: HTMLElement): boolean {
  try {
    return element.matches(':popover-open');
  } catch {
    return false;
  }
}
