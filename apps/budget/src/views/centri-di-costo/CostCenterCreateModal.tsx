import { useState } from 'react';
import { Modal, SingleSelect, MultiSelect, useToast } from '@mrsmith/ui';
import { useCreateCostCenter, useUsers, useGroups } from './queries';
import { ApiError } from '@mrsmith/api-client';
import styles from './CentriDiCostoPage.module.css';

interface CostCenterCreateModalProps {
  open: boolean;
  onClose: () => void;
}

export function CostCenterCreateModal({ open, onClose }: CostCenterCreateModalProps) {
  const [name, setName] = useState('');
  const [managerId, setManagerId] = useState<number | null>(null);
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const [selectedGroupNames, setSelectedGroupNames] = useState<string[]>([]);
  const [enabled, setEnabled] = useState(true);
  const { toast } = useToast();
  const { data: users } = useUsers();
  const { data: groups } = useGroups();
  const createCC = useCreateCostCenter();

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
    if (!managerId) return;
    createCC.mutate(
      {
        name: name.trim(),
        manager_id: managerId,
        user_ids: selectedUserIds,
        group_names: selectedGroupNames,
        enabled,
      },
      {
        onSuccess: (res) => {
          toast(res.message);
          setName('');
          setManagerId(null);
          setSelectedUserIds([]);
          setSelectedGroupNames([]);
          setEnabled(true);
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
    <Modal open={open} onClose={onClose} title="Nuovo centro di costo">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
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
        <div className={styles.formGroup}>
          <div className={styles.toggle}>
            <button
              type="button"
              className={`${styles.toggleSwitch} ${enabled ? styles.toggleActive : ''}`}
              onClick={() => setEnabled(!enabled)}
            />
            <span className={styles.toggleLabel}>{enabled ? 'Attivo' : 'Disabilitato'}</span>
          </div>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={!name.trim() || !managerId || createCC.isPending}
          >
            {createCC.isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
