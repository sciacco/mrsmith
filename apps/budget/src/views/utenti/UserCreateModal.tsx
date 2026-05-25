import { useState } from 'react';
import { Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateUser, useRoles } from './queries';
import styles from './UtentiPage.module.css';

interface UserCreateModalProps {
  open: boolean;
  onClose: () => void;
}

const EMAIL_RE = /^.+@.+\..+$/;

export function UserCreateModal({ open, onClose }: UserCreateModalProps) {
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [email, setEmail] = useState('');
  const [roleName, setRoleName] = useState<string | null>(null);
  const { toast } = useToast();
  const { data: roles, isLoading: rolesLoading, isError: rolesError } = useRoles();
  const createUser = useCreateUser();

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
    !createUser.isPending;

  function reset() {
    setFirstName('');
    setLastName('');
    setEmail('');
    setRoleName(null);
  }

  function handleClose() {
    reset();
    onClose();
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || !roleName) return;
    createUser.mutate(
      {
        first_name: trimmedFirst,
        last_name: trimmedLast,
        email: trimmedEmail,
        role_name: roleName,
      },
      {
        onSuccess: () => {
          toast('Utente creato');
          reset();
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
    <Modal open={open} onClose={handleClose} title="Nuovo utente">
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
            placeholder="Nome"
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
            placeholder="Cognome"
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
            placeholder="email@esempio.it"
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
          <button type="button" className={styles.btnSecondary} onClick={handleClose}>
            Annulla
          </button>
          <button type="submit" className={styles.btnPrimary} disabled={!canSubmit}>
            {createUser.isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
