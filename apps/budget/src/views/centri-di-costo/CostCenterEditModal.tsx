import { useState, useEffect } from 'react';
import { Modal, SingleSelect, MultiSelect, useToast } from '@mrsmith/ui';
import { useCostCenterDetails, useEditCostCenter, useUsers, useGroups } from './queries';
import { ApiError } from '@mrsmith/api-client';
import type { CostCenterEdit } from '../../api/types';
import styles from './CentriDiCostoPage.module.css';

interface CostCenterEditModalProps {
  open: boolean;
  onClose: () => void;
  costCenterName: string;
  onRenamed: (newName: string) => void;
}

export function CostCenterEditModal({
  open,
  onClose,
  costCenterName,
  onRenamed,
}: CostCenterEditModalProps) {
  const [newName, setNewName] = useState('');
  const [managerId, setManagerId] = useState<number | null>(null);
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const [selectedGroupNames, setSelectedGroupNames] = useState<string[]>([]);
  const { toast } = useToast();
  const { data: details } = useCostCenterDetails(costCenterName);
  const { data: users } = useUsers();
  const { data: groups } = useGroups();
  const editCC = useEditCostCenter();

  useEffect(() => {
    if (details) {
      setNewName(details.name);
      setManagerId(details.manager.id);
      setSelectedUserIds((details.users ?? []).map((u) => u.id));
      setSelectedGroupNames((details.groups ?? []).map((g) => g.name));
    }
  }, [details]);

  const userOptions = (users ?? []).map((u) => ({
    value: u.id,
    label: `${u.first_name} ${u.last_name}`,
  }));

  const groupOptions = (groups ?? []).map((g) => ({
    value: g.name,
    label: g.name,
  }));

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const body: CostCenterEdit = {};
    const trimmedName = newName.trim();
    if (trimmedName && trimmedName !== costCenterName) {
      body.new_name = trimmedName;
    }
    if (managerId !== null) {
      body.manager_id = managerId;
    }
    body.user_ids = selectedUserIds;
    body.group_names = selectedGroupNames;

    editCC.mutate(
      { name: costCenterName, body },
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
    <Modal open={open} onClose={onClose} title="Modifica centro di costo">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Nome del centro di costo"
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Manager</label>
          <SingleSelect
            options={userOptions}
            selected={managerId}
            onChange={setManagerId}
            placeholder="Seleziona manager..."
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
        <div className={styles.formGroup}>
          <label className={styles.label}>Gruppi</label>
          <MultiSelect<string>
            options={groupOptions}
            selected={selectedGroupNames}
            onChange={setSelectedGroupNames}
            placeholder="Seleziona gruppi..."
          />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={editCC.isPending}
          >
            {editCC.isPending ? 'Salvataggio...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
