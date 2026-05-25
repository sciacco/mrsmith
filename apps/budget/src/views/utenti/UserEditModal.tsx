import { useState, useEffect } from 'react';
import { Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useEditUser, useRoles } from './queries';
import type { ArakIntUser, ArakIntUserEdit } from '../../api/types';
import styles from './UtentiPage.module.css';

interface UserEditModalProps {
  open: boolean;
  onClose: () => void;
  user: ArakIntUser;
}

const EMAIL_RE = /^.+@.+\..+$/;

export function UserEditModal({ open, onClose, user }: UserEditModalProps) {
  const [firstName, setFirstName] = useState(user.first_name);
  const [lastName, setLastName] = useState(user.last_name);
  const [email, setEmail] = useState(user.email);
  const [roleName, setRoleName] = useState<string | null>(user.role.name);
  const { toast } = useToast();
  const { data: roles, isLoading: rolesLoading, isError: rolesError } = useRoles();
  const editUser = useEditUser();

  useEffect(() => {
    if (open) {
      setFirstName(user.first_name);
      setLastName(user.last_name);
      setEmail(user.email);
      setRoleName(user.role.name);
    }
  }, [open, user]);

  const roleOptions = (roles ?? []).map((r) => ({ value: r.name, label: r.name }));
  const rolePlaceholder = rolesError
    ? 'Ruoli non disponibili'
    : rolesLoading
    ? 'Caricamento ruoli...'
    : 'Seleziona ruolo...';

  const trimmedFirst = firstName.trim();
  const trimmedLast = lastName.trim();
  const trimmedEmail = email.trim();
  const emailValid = EMAIL_RE.test(trimmedEmail);
  const canSubmit =
    !!trimmedFirst &&
    !!trimmedLast &&
    emailValid &&
    !!roleName &&
    !rolesError &&
    !editUser.isPending;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || !roleName) return;
    const body: ArakIntUserEdit = {};
    if (trimmedFirst !== user.first_name) body.first_name = trimmedFirst;
    if (trimmedLast !== user.last_name) body.last_name = trimmedLast;
    if (trimmedEmail !== user.email) body.email = trimmedEmail;
    if (roleName !== user.role.name) body.role_name = roleName;

    if (Object.keys(body).length === 0) {
      onClose();
      return;
    }

    editUser.mutate(
      { id: user.id, body },
      {
        onSuccess: () => {
          toast('Utente aggiornato');
          onClose();
        },
        onError: (error) => {
          if (error instanceof ApiError) {
            toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
          } else {
            toast('Errore di connessione', 'error');
          }
        },
      },
    );
  }

  return (
    <Modal open={open} onClose={onClose} title="Modifica utente">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={firstName}
            onChange={(e) => setFirstName(e.target.value)}
            maxLength={255}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Cognome</label>
          <input
            className={styles.input}
            type="text"
            value={lastName}
            onChange={(e) => setLastName(e.target.value)}
            maxLength={255}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Email</label>
          <input
            className={styles.input}
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Ruolo</label>
          <SingleSelect<string>
            options={roleOptions}
            selected={roleName}
            onChange={setRoleName}
            placeholder={rolePlaceholder}
            disabled={rolesLoading || rolesError}
          />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button type="submit" className={styles.btnPrimary} disabled={!canSubmit}>
            {editUser.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
