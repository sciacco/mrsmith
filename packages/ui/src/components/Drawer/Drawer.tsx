import { useEffect, useRef, type ReactNode, type SyntheticEvent } from 'react';
import styles from './Drawer.module.css';

export type DrawerSize = 'sm' | 'md' | 'lg' | 'xl';
export type DrawerSide = 'right' | 'left';

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  /**
   * Intercept dismiss attempts (ESC, backdrop click, explicit close button).
   * Return `false` (or a Promise resolving to `false`) to cancel the close.
   * When omitted, dismissals call `onClose` directly.
   */
  onDismissAttempt?: () => boolean | Promise<boolean>;
  title?: ReactNode;
  subtitle?: ReactNode;
  headerExtra?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  size?: DrawerSize;
  side?: DrawerSide;
  ariaLabel?: string;
  closeOnBackdrop?: boolean;
  closeOnEsc?: boolean;
  hideCloseButton?: boolean;
}

export function Drawer({
  open,
  onClose,
  onDismissAttempt,
  title,
  subtitle,
  headerExtra,
  footer,
  children,
  size = 'md',
  side = 'right',
  ariaLabel,
  closeOnBackdrop = true,
  closeOnEsc = true,
  hideCloseButton = false,
}: DrawerProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (open && !dialog.open) {
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  const attemptDismiss = async () => {
    if (!onDismissAttempt) {
      onClose();
      return;
    }
    const result = await onDismissAttempt();
    if (result) onClose();
  };

  const handleCancel = (e: SyntheticEvent<HTMLDialogElement>) => {
    e.preventDefault();
    if (!closeOnEsc) return;
    void attemptDismiss();
  };

  const handleBackdropClick = (e: React.MouseEvent<HTMLDialogElement>) => {
    if (!closeOnBackdrop) return;
    if (e.target === dialogRef.current) void attemptDismiss();
  };

  const handleCloseButton = () => {
    void attemptDismiss();
  };

  return (
    <dialog
      ref={dialogRef}
      className={`${styles.dialog} ${styles[size]} ${styles[side]}`}
      aria-label={ariaLabel ?? (typeof title === 'string' ? title : undefined)}
      onCancel={handleCancel}
      onClick={handleBackdropClick}
    >
      <div className={styles.panel} onClick={e => e.stopPropagation()}>
        {(title || subtitle || headerExtra || !hideCloseButton) && (
          <header className={styles.header}>
            <div className={styles.headerText}>
              {title && <h2 className={styles.title}>{title}</h2>}
              {subtitle && <div className={styles.subtitle}>{subtitle}</div>}
            </div>
            {headerExtra && <div className={styles.headerExtra}>{headerExtra}</div>}
            {!hideCloseButton && (
              <button
                type="button"
                className={styles.close}
                onClick={handleCloseButton}
                aria-label="Chiudi"
              >
                &times;
              </button>
            )}
          </header>
        )}
        <div className={styles.body}>{children}</div>
        {footer && <footer className={styles.footer}>{footer}</footer>}
      </div>
    </dialog>
  );
}
