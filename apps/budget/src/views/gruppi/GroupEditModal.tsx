import { useState, useEffect } from 'react';
import { Modal } from '../../components/Modal/Modal';
import { MultiSelect } from '../../components/MultiSelect/MultiSelect';
import { useToast } from '../../components/Toast/ToastProvider';
import { useEditGroup, useGroupDetails, useUsers } from './queries';
import { ApiError } from '@mrsmith/api-client';
import styles from './GruppiPage.module.css';

interface GroupEditModalProps {
  open: boolean;
  onClose: () => void;
  groupName: string;
  onRenamed: (newName: string) => void;
}

export function GroupEditModal({ open, onClose, groupName, onRenamed }: GroupEditModalProps) {
  const [newName, setNewName] = useState('');
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const { toast } = useToast();
  const { data: details } = useGroupDetails(groupName);
  const { data: users } = useUsers();
  const editGroup = useEditGroup();

  useEffect(() => {
    if (details) {
      setNewName(details.name);
      setSelectedUserIds(details.users.map((u) => u.id));
    }
  }, [details]);

  const userOptions = (users ?? []).map((u) => ({
    value: u.id,
    label: `${u.first_name} ${u.last_name}`,
  }));

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const body: { new_name?: string; user_ids?: number[] } = {};
    const trimmedName = newName.trim();
    if (trimmedName && trimmedName !== groupName) {
      body.new_name = trimmedName;
    }
    body.user_ids = selectedUserIds;

    editGroup.mutate(
      { name: groupName, body },
      {
        onSuccess: (res) => {
          toast(res.message);
          if (body.new_name) {
            onRenamed(body.new_name);
          }
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
    <Modal open={open} onClose={onClose} title="Modifica gruppo">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
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
            disabled={editGroup.isPending}
          >
            {editGroup.isPending ? 'Salvataggio...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
