import { useEffect, useRef, type ReactNode, type SyntheticEvent } from 'react';
import { useFocusRestore } from '../../hooks/useFocusRestore';
import styles from './Modal.module.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'wide' | 'xwide' | 'fluid';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children: ReactNode;
  size?: ModalSize;
  /** @deprecated Use `size="wide"` instead. Kept for backward compatibility. */
  wide?: boolean;
  dismissible?: boolean;
  /** When false, pressing Escape will not close the modal. Useful when the modal contains a file input. */
  closeOnEscape?: boolean;
}

export function Modal({
  open,
  onClose,
  title,
  children,
  size,
  wide,
  dismissible = true,
  closeOnEscape = true,
}: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  const resolvedSize: ModalSize = size ?? (wide ? 'wide' : 'md');

  useFocusRestore(open);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (open && !dialog.open) {
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  const handleCancel = (e: SyntheticEvent<HTMLDialogElement>) => {
    // Keep closure controlled by React: otherwise the native close event would
    // call onClose a second time after this handler.
    e.preventDefault();
    if (dismissible && closeOnEscape) onClose();
  };

  return (
    <dialog
      ref={dialogRef}
      className={`${styles.dialog} ${styles[resolvedSize]}`}
      onCancel={handleCancel}
      onClick={(e) => {
        if (!dismissible) return;
        if (e.target === dialogRef.current) onClose();
      }}
    >
      <div className={styles.content}>
        <div className={styles.header}>
          <h2 className={styles.title}>{title}</h2>
          {dismissible && (
            <button className={styles.close} onClick={onClose} aria-label="Chiudi">
              &times;
            </button>
          )}
        </div>
        {children}
      </div>
    </dialog>
  );
}
