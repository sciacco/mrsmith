import { useEffect, useState } from 'react';
import { Modal, Button, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import type { User } from '../../api/users';
import { useDeleteUser } from '../../hooks/useDeleteUser';
import styles from './GestioneUtenti.module.css';

interface EliminaUtenteModalProps {
  user: User | null;
  customerId: number;
  onClose: () => void;
}

// Type-to-confirm dialog for user deletion. The upstream DELETE is
// irreversible, so the danger button stays disabled until the operator
// types the exact email of the user being removed.
export function EliminaUtenteModal({
  user,
  customerId,
  onClose,
}: EliminaUtenteModalProps) {
  const [confirmText, setConfirmText] = useState('');
  const { toast } = useToast();
  const deleteUser = useDeleteUser();
  const open = user != null;

  useEffect(() => {
    if (!open) setConfirmText('');
  }, [open]);

  const emailMatches =
    user != null &&
    confirmText.trim().toLocaleLowerCase() ===
      user.email.trim().toLocaleLowerCase();

  function handleConfirm() {
    if (!user || !emailMatches || deleteUser.isPending) return;
    deleteUser.mutate(
      { userId: user.id, customerId },
      {
        onSuccess: () => {
          toast('Utente eliminato');
          onClose();
        },
        onError: (error) => {
          const fallback = "Qualcosa e' andato storto";
          if (error instanceof ApiError) {
            const message = readMessage(error.body) ?? fallback;
            toast(`${error.status} — ${message}`, 'error');
          } else {
            toast(fallback, 'error');
          }
        },
      },
    );
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Elimina utente"
      size="sm"
      dismissible={!deleteUser.isPending}
    >
      <div className={styles.confirmBody}>
        <p className={styles.confirmText}>
          L&apos;utente <strong>{user?.email}</strong> sarà eliminato dal
          Customer Portal. L&apos;operazione non è reversibile.
        </p>
        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="delete-user-confirm">
            Digita l&apos;email dell&apos;utente per confermare
          </label>
          <input
            id="delete-user-confirm"
            className={styles.input}
            type="text"
            autoComplete="off"
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            disabled={deleteUser.isPending}
          />
        </div>
      </div>
      <div className={styles.actions}>
        <Button
          size="md"
          variant="secondary"
          onClick={onClose}
          disabled={deleteUser.isPending}
        >
          Indietro
        </Button>
        <Button
          size="md"
          variant="danger"
          onClick={handleConfirm}
          disabled={!emailMatches}
          loading={deleteUser.isPending}
        >
          Elimina
        </Button>
      </div>
    </Modal>
  );
}

function readMessage(body: unknown): string | undefined {
  if (typeof body === 'object' && body !== null && 'message' in body) {
    const raw = (body as { message: unknown }).message;
    if (typeof raw === 'string' && raw.trim().length > 0) return raw.trim();
  }
  return undefined;
}
