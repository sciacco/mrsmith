import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import {
  useBatchUpdateCustomerGroups,
  useCreateCustomerGroup,
  useCustomerGroups,
} from '../../api/queries';
import styles from './SettingsPage.module.css';

export function CustomerGroupsPage() {
  const { toast } = useToast();
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<'create' | 'edit'>('create');
  const [editingId, setEditingId] = useState<number | null>(null);
  const [draft, setDraft] = useState({ name: '', is_partner: false });

  const { data, isLoading, error } = useCustomerGroups();
  const createCustomerGroup = useCreateCustomerGroup();
  const batchUpdate = useBatchUpdateCustomerGroups();

  const groups = data ?? [];

  function openCreate() {
    setModalMode('create');
    setDraft({ name: '', is_partner: false });
    setEditingId(null);
    setModalOpen(true);
  }

  function openEdit(id: number) {
    const group = groups.find((g) => g.id === id);
    if (!group || group.read_only) return;
    setModalMode('edit');
    setDraft({ name: group.name, is_partner: group.is_partner });
    setEditingId(id);
    setModalOpen(true);
  }

  async function handleSave() {
    if (!draft.name.trim()) {
      toast('Il nome gruppo e obbligatorio', 'error');
      return;
    }

    try {
      if (modalMode === 'create') {
        await createCustomerGroup.mutateAsync({ name: draft.name.trim(), is_partner: draft.is_partner });
        toast('Gruppo creato', 'success');
      } else if (editingId != null) {
        await batchUpdate.mutateAsync({ items: [{ id: editingId, name: draft.name.trim(), is_partner: draft.is_partner }] });
        toast('Gruppo aggiornato', 'success');
      }
      setModalOpen(false);
    } catch (err) {
      const message = getErrorMessage(err, 'Impossibile salvare il gruppo');
      toast(message === 'read_only_group' ? 'Questo gruppo e protetto in lettura' : message, 'error');
    }
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={6} />
      </section>
    );
  }

  const selectedGroup = groups.find((g) => g.id === selectedId);
  const canEdit = selectedGroup != null && !selectedGroup.read_only;

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1>Gruppi cliente</h1>
          <p className={styles.subtitle}>{groups.length} gruppi</p>
        </div>
        <button type="button" className={styles.primaryButton} onClick={openCreate}>
          Nuovo gruppo
        </button>
      </header>

      <section className={styles.card}>
        <div className={styles.cardToolbar}>
          <button
            type="button"
            className={styles.secondaryButton}
            disabled={!canEdit}
            onClick={() => { if (selectedId != null) openEdit(selectedId); }}
          >
            Modifica
          </button>
        </div>

        {error ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Impossibile caricare i gruppi</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : groups.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessun gruppo</p>
            <p className={styles.emptyText}>Crea il primo gruppo cliente per iniziare.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Base discount</th>
                  <th>Partner</th>
                  <th>Stato</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((group, index) => (
                  <tr
                    key={group.id}
                    className={`${selectedId === group.id ? styles.rowSelected : ''} ${group.read_only ? styles.rowMuted : ''}`}
                    style={{ animationDelay: `${index * 0.03}s` }}
                    onClick={() => setSelectedId(group.id)}
                    onDoubleClick={() => openEdit(group.id)}
                  >
                    <td>{group.name}</td>
                    <td><span className={styles.mono}>{group.base_discount == null ? 'n/d' : `${(group.base_discount * 100).toFixed(1)}%`}</span></td>
                    <td>{group.is_partner ? 'Si' : 'No'}</td>
                    <td>
                      <div className={styles.badgeRow}>
                        {group.is_default ? <span className={styles.badge}>Default</span> : null}
                        {group.read_only ? <span className={styles.badgeMuted}>Read only</span> : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={modalMode === 'create' ? 'Nuovo gruppo' : 'Modifica gruppo'}>
        <div className={styles.modalBody}>
          <label className={styles.field}>
            <span>Nome</span>
            <input value={draft.name} onChange={(e) => setDraft((c) => ({ ...c, name: e.target.value }))} placeholder="Nome gruppo" />
          </label>
          <ToggleSwitch
            id="group-partner"
            checked={draft.is_partner}
            onChange={(v) => setDraft((c) => ({ ...c, is_partner: v }))}
            label="Partner"
          />
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setModalOpen(false)}>Annulla</button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleSave()} disabled={createCustomerGroup.isPending || batchUpdate.isPending}>Salva</button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') return message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}
