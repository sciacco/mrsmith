// Gruppi locali (#158, §Persone 2): platea delle regole a gruppo e
// collegamento delle aree di competenza. Il 409 di eliminazione (platea di
// regola attiva o collegato a un'area) si mostra per intero, nessuna
// previsione locale di quando un gruppo è libero.

import { useState } from 'react';
import { Button, Icon, MultiSelect, Modal, Skeleton, VisuallyHidden, useToast } from '@mrsmith/ui';
import {
  useDeleteGroup,
  useReplaceGroupMembers,
  useTrainingGroups,
  useTrainingPeople,
  useUpsertGroup,
} from '../../api/queries';
import type { GroupListRow } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { PERSON_STATUS_LABELS } from '../../lib/labels';
import formStyles from '../requests/requestShared.module.css';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './people.module.css';

export function GroupsSection() {
  const { toast } = useToast();
  const groups = useTrainingGroups();
  const [editing, setEditing] = useState<GroupListRow | 'create' | null>(null);
  const [managingMembers, setManagingMembers] = useState<GroupListRow | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<GroupListRow | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const deleteGroup = useDeleteGroup();

  function closeDeleteModal() {
    setDeleteTarget(null);
    setDeleteConfirmText('');
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    if (deleteConfirmText.trim() !== deleteTarget.name.trim()) return;
    setDeleteError(null);
    try {
      await deleteGroup.mutateAsync(deleteTarget.id);
      toast('Gruppo eliminato');
      closeDeleteModal();
    } catch (e) {
      setDeleteError(describeApiError(e, 'Eliminazione non riuscita'));
    }
  }

  return (
    <section className={styles.section}>
      <header className={listStyles.header}>
        <div>
          <h2 className={styles.sectionTitle}>Gruppi locali</h2>
          <p className={listStyles.subtitle}>Platee libere per le regole formative e collegamento alle aree.</p>
        </div>
        <Button variant="secondary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={() => setEditing('create')}>
          Nuovo gruppo
        </Button>
      </header>

      {groups.isLoading ? (
        <Skeleton rows={3} />
      ) : groups.isError ? (
        <p className={listStyles.errorNotice}>Lettura dei gruppi non riuscita. Riprovare più tardi.</p>
      ) : (groups.data ?? []).length === 0 ? (
        <div className={listStyles.empty}>
          <p className={listStyles.emptyTitle}>Nessun gruppo locale</p>
          <p className={listStyles.emptyDescription}>Crea un gruppo per governare una platea o un'area di competenza.</p>
        </div>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Descrizione</th>
                <th>Membri</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {(groups.data ?? []).map((g) => (
                <tr key={g.id}>
                  <td>{g.name}</td>
                  <td>{g.description || '—'}</td>
                  <td>{g.members.length === 0 ? 'Nessun membro' : g.members.map((m) => m.name).join(', ')}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <Button variant="ghost" size="sm" onClick={() => setManagingMembers(g)}>
                        Membri
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setEditing(g)}>
                        Modifica
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(g)}>
                        Elimina
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && <GroupEditorModal group={editing === 'create' ? null : editing} onClose={() => setEditing(null)} />}
      {managingMembers && (
        <GroupMembersModal group={managingMembers} onClose={() => setManagingMembers(null)} />
      )}

      <Modal open={deleteTarget !== null} onClose={closeDeleteModal} title="Elimina gruppo" size="sm" dismissible={!deleteGroup.isPending}>
        <div className={formStyles.body}>
          <p>Eliminare il gruppo «{deleteTarget?.name}»? L'operazione non è reversibile.</p>
          <label className={formStyles.field}>
            Digita il nome del gruppo per confermare
            <input
              className={formStyles.input}
              type="text"
              autoComplete="off"
              value={deleteConfirmText}
              onChange={(e) => setDeleteConfirmText(e.target.value)}
              disabled={deleteGroup.isPending}
            />
          </label>
          <ErrorPanel message={deleteError} onDismiss={() => setDeleteError(null)} />
          <div className={formStyles.actions}>
            <Button variant="ghost" size="md" onClick={closeDeleteModal} disabled={deleteGroup.isPending}>
              Annulla
            </Button>
            <Button
              variant="danger"
              size="md"
              loading={deleteGroup.isPending}
              disabled={deleteTarget !== null && deleteConfirmText.trim() !== deleteTarget.name.trim()}
              onClick={confirmDelete}
            >
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function GroupEditorModal({ group, onClose }: { group: GroupListRow | null; onClose: () => void }) {
  const { toast } = useToast();
  const upsertGroup = useUpsertGroup();
  const [name, setName] = useState(group?.name ?? '');
  const [description, setDescription] = useState(group?.description ?? '');
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (name.trim() === '') return;
    setError(null);
    try {
      await upsertGroup.mutateAsync({ id: group?.id, input: { name: name.trim(), description: description.trim() || undefined } });
      toast(group ? 'Gruppo aggiornato' : 'Gruppo creato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title={group ? 'Rinomina gruppo' : 'Nuovo gruppo'} size="sm">
      <div className={formStyles.body}>
        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Nome
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <input className={formStyles.input} value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className={formStyles.field}>
          Descrizione
          <textarea className={formStyles.textarea} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={upsertGroup.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={upsertGroup.isPending} disabled={name.trim() === ''} onClick={submit}>
            Salva
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function GroupMembersModal({ group, onClose }: { group: GroupListRow; onClose: () => void }) {
  const { toast } = useToast();
  const people = useTrainingPeople();
  const replaceMembers = useReplaceGroupMembers();
  const [selected, setSelected] = useState<string[]>(group.members.map((m) => m.employeeId));
  const [error, setError] = useState<string | null>(null);

  // Le opzioni includono anche i membri attuali non attivi (etichettati),
  // altrimenti restano nella selezione ma invisibili e non rimovibili: la
  // sostituzione dei membri viene poi rifiutata dal backend con 422
  // person_not_active (store_enrollments.go:377) senza che l'operatore possa
  // capire perché. Selezionabili in aggiunta restano solo le persone attive.
  const currentMemberIds = new Set(group.members.map((m) => m.employeeId));
  const memberOptions = (people.data ?? [])
    .filter((p) => p.status === 'active' || currentMemberIds.has(p.id))
    .map((p) => ({
      value: p.id,
      label:
        p.status === 'active'
          ? `${p.lastName} ${p.firstName}`
          : `${p.lastName} ${p.firstName} — ${(PERSON_STATUS_LABELS[p.status] ?? p.status).toLowerCase()}`,
    }));

  async function submit() {
    setError(null);
    try {
      await replaceMembers.mutateAsync({ id: group.id, input: { employeeIds: selected } });
      toast('Membri aggiornati');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Aggiornamento dei membri non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title={`Membri — ${group.name}`} size="md">
      <div className={formStyles.body}>
        <label className={formStyles.field}>
          Persone
          <MultiSelect<string>
            options={memberOptions}
            selected={selected}
            onChange={setSelected}
            placeholder="Seleziona persone..."
          />
          <span className={formStyles.hint}>Solo le persone attive sono selezionabili in aggiunta ai membri correnti.</span>
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={replaceMembers.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={replaceMembers.isPending} onClick={submit}>
            Salva membri
          </Button>
        </div>
      </div>
    </Modal>
  );
}
