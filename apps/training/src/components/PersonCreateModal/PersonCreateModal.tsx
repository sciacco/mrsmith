import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { useCreatePerson } from '../../api/queries';
import type { LookupItem, PersonStatus } from '../../api/types';
import styles from '../PersonEditModal/PersonEditModal.module.css';

interface PersonCreateModalProps {
  open: boolean;
  teams: LookupItem[];
  onClose: () => void;
  onCreated: (id: string) => void;
}

const STATUS_OPTIONS: Array<{ value: PersonStatus; label: string }> = [
  { value: 'active', label: 'Attiva' },
  { value: 'on_leave', label: 'In aspettativa' },
  { value: 'terminated', label: 'Terminata' },
];

const emptyDraft = {
  firstName: '',
  lastName: '',
  email: '',
  status: 'active' as PersonStatus,
  teamId: null as string | null,
  notes: '',
};

function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  }
  return 'Creazione non riuscita';
}

function validEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

export function PersonCreateModal({ open, teams, onClose, onCreated }: PersonCreateModalProps) {
  const { toast } = useToast();
  const createPerson = useCreatePerson();
  const [draft, setDraft] = useState({ ...emptyDraft });
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) return;
    setDraft({ ...emptyDraft });
    setSubmitted(false);
  }, [open]);

  const teamOptions = useMemo(
    () => teams.filter((team) => team.active).map((team) => ({ value: team.id, label: team.label })),
    [teams],
  );

  const formValid =
    draft.firstName.trim().length > 0 &&
    draft.lastName.trim().length > 0 &&
    validEmail(draft.email) &&
    STATUS_OPTIONS.some((option) => option.value === draft.status);
  const canSubmit = formValid && !createPerson.isPending;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitted(true);
    if (!canSubmit) return;

    try {
      const response = await createPerson.mutateAsync({
        firstName: draft.firstName.trim(),
        lastName: draft.lastName.trim(),
        email: draft.email.trim(),
        status: draft.status,
        teamId: draft.teamId,
        notes: draft.notes.trim() || undefined,
      });
      toast('Persona creata');
      onClose();
      if (response.id) onCreated(response.id);
    } catch (error) {
      toast(apiErrorMessage(error), 'error');
    }
  }

  const showValidation = submitted && !formValid;

  return (
    <Modal open={open} onClose={onClose} title="Nuova persona" size="lg">
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label htmlFor="person-create-first-name" className={styles.label}>Nome</label>
            <input
              id="person-create-first-name"
              className={styles.input}
              value={draft.firstName}
              onChange={(event) => setDraft((current) => ({ ...current, firstName: event.target.value }))}
              autoComplete="given-name"
              required
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="person-create-last-name" className={styles.label}>Cognome</label>
            <input
              id="person-create-last-name"
              className={styles.input}
              value={draft.lastName}
              onChange={(event) => setDraft((current) => ({ ...current, lastName: event.target.value }))}
              autoComplete="family-name"
              required
            />
          </div>
        </div>

        <div className={styles.field}>
          <label htmlFor="person-create-email" className={styles.label}>Email</label>
          <input
            id="person-create-email"
            className={styles.input}
            type="email"
            value={draft.email}
            onChange={(event) => setDraft((current) => ({ ...current, email: event.target.value }))}
            autoComplete="email"
            required
          />
        </div>

        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label className={styles.label}>Stato</label>
            <SingleSelect
              options={STATUS_OPTIONS}
              selected={draft.status}
              onChange={(value) => setDraft((current) => ({ ...current, status: (value ?? 'active') as PersonStatus }))}
            />
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Team</label>
            <SingleSelect
              options={teamOptions}
              selected={draft.teamId}
              onChange={(value) => setDraft((current) => ({ ...current, teamId: value ? String(value) : null }))}
              placeholder="Senza team"
              allowClear
              clearLabel="Senza team"
              searchable
            />
          </div>
        </div>

        <div className={styles.field}>
          <label htmlFor="person-create-notes" className={styles.label}>Note</label>
          <textarea
            id="person-create-notes"
            className={styles.textarea}
            rows={4}
            value={draft.notes}
            onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))}
          />
        </div>

        {showValidation && <p className={styles.error}>Compila nome, cognome, email e stato.</p>}

        <div className={styles.footer}>
          <Button type="button" variant="ghost" size="md" onClick={onClose}>Annulla</Button>
          <Button type="submit" variant="primary" size="md" loading={createPerson.isPending}>
            Crea persona
          </Button>
        </div>
      </form>
    </Modal>
  );
}
