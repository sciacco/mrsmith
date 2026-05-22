import { useEffect, useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { useUpdatePerson } from '../../api/queries';
import type { LookupItem, PersonProfile, PersonStatus } from '../../api/types';
import styles from './PersonEditModal.module.css';

interface PersonEditModalProps {
  open: boolean;
  profile: PersonProfile;
  teams: LookupItem[];
  onClose: () => void;
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
  return 'Salvataggio non riuscito';
}

function validEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

export function PersonEditModal({ open, profile, teams, onClose }: PersonEditModalProps) {
  const { toast } = useToast();
  const updatePerson = useUpdatePerson();
  const [draft, setDraft] = useState({ ...emptyDraft });
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) return;
    const identity = profile.identity_min;
    setDraft({
      firstName: identity.first_name,
      lastName: identity.last_name,
      email: identity.email,
      status: identity.status,
      teamId: identity.team_id || null,
      notes: identity.notes ?? '',
    });
    setSubmitted(false);
  }, [open, profile]);

  const teamOptions = useMemo(
    () => {
      const options = teams.filter((team) => team.active).map((team) => ({ value: team.id, label: team.label }));
      const identity = profile.identity_min;
      if (identity.team_id && !options.some((option) => option.value === identity.team_id)) {
        const label = [identity.team_code, identity.team_name].filter(Boolean).join(' - ') || 'Team attuale';
        options.push({ value: identity.team_id, label });
      }
      return options;
    },
    [profile.identity_min, teams],
  );

  const formValid =
    draft.firstName.trim().length > 0 &&
    draft.lastName.trim().length > 0 &&
    validEmail(draft.email) &&
    STATUS_OPTIONS.some((option) => option.value === draft.status);
  const canSubmit = formValid && !updatePerson.isPending;

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setSubmitted(true);
    if (!canSubmit) return;

    try {
      await updatePerson.mutateAsync({
        id: profile.identity_min.id,
        body: {
          firstName: draft.firstName.trim(),
          lastName: draft.lastName.trim(),
          email: draft.email.trim(),
          status: draft.status,
          teamId: draft.teamId,
          notes: draft.notes.trim() || undefined,
        },
      });
      toast('Persona aggiornata');
      onClose();
    } catch (error) {
      toast(apiErrorMessage(error), 'error');
    }
  }

  const showValidation = submitted && !formValid;

  return (
    <Modal open={open} onClose={onClose} title="Modifica persona" size="lg">
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label htmlFor="person-first-name" className={styles.label}>Nome</label>
            <input
              id="person-first-name"
              className={styles.input}
              value={draft.firstName}
              onChange={(event) => setDraft((current) => ({ ...current, firstName: event.target.value }))}
              autoComplete="given-name"
              required
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="person-last-name" className={styles.label}>Cognome</label>
            <input
              id="person-last-name"
              className={styles.input}
              value={draft.lastName}
              onChange={(event) => setDraft((current) => ({ ...current, lastName: event.target.value }))}
              autoComplete="family-name"
              required
            />
          </div>
        </div>

        <div className={styles.field}>
          <label htmlFor="person-email" className={styles.label}>Email</label>
          <input
            id="person-email"
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
          <label htmlFor="person-notes" className={styles.label}>Note</label>
          <textarea
            id="person-notes"
            className={styles.textarea}
            rows={4}
            value={draft.notes}
            onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))}
          />
        </div>

        {showValidation && <p className={styles.error}>Compila nome, cognome, email e stato.</p>}

        <div className={styles.footer}>
          <Button type="button" variant="ghost" size="md" onClick={onClose}>Annulla</Button>
          <Button type="submit" variant="primary" size="md" loading={updatePerson.isPending}>
            Salva modifiche
          </Button>
        </div>
      </form>
    </Modal>
  );
}
