import { useEffect, useRef, type ReactNode, type SyntheticEvent } from 'react';
import styles from './Modal.module.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'wide' | 'xwide' | 'fluid';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  size?: ModalSize;
  /** @deprecated Use `size="wide"` instead. Kept for backward compatibility. */
  wide?: boolean;
  dismissible?: boolean;
}

export function Modal({
  open,
  onClose,
  title,
  children,
  size,
  wide,
  dismissible = true,
}: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  const resolvedSize: ModalSize = size ?? (wide ? 'wide' : 'md');

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
    if (!dismissible) {
      e.preventDefault();
      return;
    }
    onClose();
  };

  return (
    <dialog
      ref={dialogRef}
      className={`${styles.dialog} ${styles[resolvedSize]}`}
      onClose={onClose}
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
