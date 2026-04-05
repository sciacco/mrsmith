import { useState } from 'react';
import { Modal, MultiSelect, useToast } from '@mrsmith/ui';
import { useCreateGroup, useUsers } from './queries';
import { ApiError } from '@mrsmith/api-client';
import styles from './GruppiPage.module.css';

interface GroupCreateModalProps {
  open: boolean;
  onClose: () => void;
}

export function GroupCreateModal({ open, onClose }: GroupCreateModalProps) {
  const [name, setName] = useState('');
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const { toast } = useToast();
  const { data: users } = useUsers();
  const createGroup = useCreateGroup();

  const userOptions = (users ?? []).map((u) => ({
    value: u.id,
    label: `${u.first_name} ${u.last_name}`,
  }));

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    createGroup.mutate(
      { name: name.trim(), user_ids: selectedUserIds },
      {
        onSuccess: (res) => {
          toast(res.message);
          setName('');
          setSelectedUserIds([]);
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
    <Modal open={open} onClose={onClose} title="Nuovo gruppo">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder="Nome del gruppo"
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Utenti</label>
          <MultiSelect
            options={userOptions}
            selected={selectedUserIds}
            onChange={setSelectedUserIds}
            placeholder="Seleziona utenti..."
          />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={!name.trim() || createGroup.isPending}
          >
            {createGroup.isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
