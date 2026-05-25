import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useDeleteUser } from './queries';
import type { ArakIntUser } from '../../api/types';
import styles from './UtentiPage.module.css';

interface UserDisableConfirmProps {
  open: boolean;
  onClose: () => void;
  user: ArakIntUser;
}

const CONFIRM_WORD = 'DISATTIVA';

export function UserDisableConfirm({ open, onClose, user }: UserDisableConfirmProps) {
  const [step, setStep] = useState<'confirm' | 'type'>('confirm');
  const [typed, setTyped] = useState('');
  const { toast } = useToast();
  const deleteUser = useDeleteUser();

  useEffect(() => {
    if (open) {
      setStep('confirm');
      setTyped('');
    }
  }, [open]);

  function handleConfirm() {
    if (typed !== CONFIRM_WORD) return;
    deleteUser.mutate(user.id, {
      onSuccess: () => {
        toast('Utente disattivato');
        onClose();
      },
      onError: (error) => {
        if (error instanceof ApiError) {
          toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    });
  }

  return (
    <Modal open={open} onClose={onClose} title="Disattiva utente">
      {step === 'confirm' ? (
        <>
          <div className={styles.confirmMessage}>
            Disattivare <strong>{user.first_name} {user.last_name}</strong> ({user.email})?
            <br />
            <br />
            L&apos;utente perderà l&apos;accesso e verrà nascosto dai selettori (gruppi,
            centri di costo, allocazioni). L&apos;operazione non può essere annullata da
            questa pagina.
          </div>
          <div className={styles.actions}>
            <button type="button" className={styles.btnSecondary} onClick={onClose}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.btnDanger}
              onClick={() => setStep('type')}
            >
              Continua
            </button>
          </div>
        </>
      ) : (
        <>
          <div className={styles.confirmMessage}>
            Per confermare, scrivi <strong>{CONFIRM_WORD}</strong> qui sotto.
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label} htmlFor="disable-confirm-input">
              Per confermare, scrivi {CONFIRM_WORD}
            </label>
            <input
              id="disable-confirm-input"
              className={styles.input}
              type="text"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoFocus
              autoComplete="off"
              spellCheck={false}
            />
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.btnSecondary}
              onClick={() => {
                setStep('confirm');
                setTyped('');
              }}
            >
              Indietro
            </button>
            <button
              type="button"
              className={styles.btnDanger}
              onClick={handleConfirm}
              disabled={typed !== CONFIRM_WORD || deleteUser.isPending}
            >
              {deleteUser.isPending ? 'Disattivazione...' : 'Disattiva'}
            </button>
          </div>
        </>
      )}
    </Modal>
  );
}
