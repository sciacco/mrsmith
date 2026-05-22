import { useEffect, useState } from 'react';
import { Button, Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { useCreateCourse, useTrainingLookups } from '../../api/queries';
import styles from './NewCourseModal.module.css';

interface NewCourseModalProps {
  open: boolean;
  isPeopleAdmin: boolean;
  onClose: () => void;
  onCreated?: () => void;
}

const DELIVERY_OPTIONS = [
  { value: 'classroom', label: 'Aula' },
  { value: 'online_live', label: 'Online live' },
  { value: 'online_self', label: 'Self-paced' },
  { value: 'on_the_job', label: 'On the job' },
  { value: 'mixed', label: 'Misto' },
];

const PROVIDER_OPTIONS = [
  { value: 'internal', label: 'Interno' },
  { value: 'external', label: 'Esterno' },
];

const emptyDraft = {
  title: '',
  vendorId: '',
  skillAreaId: '',
  deliveryMode: 'mixed',
  providerKind: 'external',
  defaultHours: '',
  defaultCost: '',
  description: '',
  complianceFramework: '',
};

function parseOptionalHours(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === '') return undefined;
  return Number(trimmed);
}

function parseOptionalCost(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === '') return undefined;
  return Number(trimmed.replace(',', '.'));
}

function validateOptionalHours(value: string): string {
  const parsed = parseOptionalHours(value);
  if (parsed === undefined) return '';
  return Number.isInteger(parsed) && parsed > 0
    ? ''
    : 'Inserisci un numero intero maggiore di 0.';
}

function validateOptionalCost(value: string): string {
  const parsed = parseOptionalCost(value);
  if (parsed === undefined) return '';
  return Number.isFinite(parsed) && parsed >= 0
    ? ''
    : 'Inserisci un costo valido maggiore o uguale a 0.';
}

export function NewCourseModal({ open, isPeopleAdmin, onClose, onCreated }: NewCourseModalProps) {
  const { toast } = useToast();
  const lookups = useTrainingLookups(isPeopleAdmin && open);
  const create = useCreateCourse(isPeopleAdmin);
  const [draft, setDraft] = useState({ ...emptyDraft });

  useEffect(() => {
    if (open) setDraft({ ...emptyDraft });
  }, [open]);

  const vendorOptions = (lookups.data?.vendors ?? []).filter((v) => v.active).map((v) => ({ value: v.id, label: v.label }));
  const skillOptions = (lookups.data?.skillAreas ?? []).filter((s) => s.active).map((s) => ({ value: s.id, label: s.label }));
  const hoursError = validateOptionalHours(draft.defaultHours);
  const costError = validateOptionalCost(draft.defaultCost);

  const canSubmit =
    draft.title.trim().length > 0 &&
    draft.vendorId.length > 0 &&
    draft.skillAreaId.length > 0 &&
    !hoursError &&
    !costError &&
    !create.isPending;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    const defaultHours = parseOptionalHours(draft.defaultHours);
    const defaultCost = parseOptionalCost(draft.defaultCost);
    try {
      await create.mutateAsync({
        title: draft.title.trim(),
        vendorId: draft.vendorId,
        skillAreaId: draft.skillAreaId,
        deliveryMode: draft.deliveryMode,
        providerKind: draft.providerKind,
        defaultHours,
        defaultCost,
        description: draft.description.trim() || undefined,
        complianceFramework: draft.complianceFramework.trim() || undefined,
        mandatory: draft.complianceFramework.trim().length > 0,
        active: true,
      });
      toast('Corso creato');
      onCreated?.();
      onClose();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Errore nella creazione del corso', 'error');
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuovo corso" size="wide">
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.field}>
          <label htmlFor="nc-title" className={styles.label}>
            Titolo <span className={styles.req} aria-hidden="true" />
            <span className="sr-only"> (obbligatorio)</span>
          </label>
          <input
            id="nc-title"
            className={styles.input}
            value={draft.title}
            onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
            required
          />
        </div>

        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label className={styles.label}>
              Skill area <span className={styles.req} aria-hidden="true" />
            </label>
            <SingleSelect
              options={skillOptions}
              selected={draft.skillAreaId || null}
              onChange={(v) => setDraft((d) => ({ ...d, skillAreaId: v ?? '' }))}
              placeholder="Seleziona skill area"
            />
          </div>
          <div className={styles.field}>
            <label className={styles.label}>
              Fornitore <span className={styles.req} aria-hidden="true" />
            </label>
            <SingleSelect
              options={vendorOptions}
              selected={draft.vendorId || null}
              onChange={(v) => setDraft((d) => ({ ...d, vendorId: v ?? '' }))}
              placeholder="Seleziona fornitore"
            />
          </div>
        </div>

        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label className={styles.label}>Modalità</label>
            <SingleSelect
              options={DELIVERY_OPTIONS}
              selected={draft.deliveryMode}
              onChange={(v) => setDraft((d) => ({ ...d, deliveryMode: v ?? 'mixed' }))}
            />
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Provider</label>
            <SingleSelect
              options={PROVIDER_OPTIONS}
              selected={draft.providerKind}
              onChange={(v) => setDraft((d) => ({ ...d, providerKind: v ?? 'external' }))}
            />
          </div>
        </div>

        <div className={styles.gridTwo}>
          <div className={styles.field}>
            <label htmlFor="nc-hours" className={styles.label}>
              Durata stimata (h)
            </label>
            <input
              id="nc-hours"
              type="number"
              min={1}
              className={styles.input}
              placeholder="Da definire"
              value={draft.defaultHours}
              onChange={(e) => setDraft((d) => ({ ...d, defaultHours: e.target.value }))}
              aria-invalid={Boolean(hoursError)}
            />
            {hoursError && <p className={styles.errorText}>{hoursError}</p>}
          </div>
          <div className={styles.field}>
            <label htmlFor="nc-cost" className={styles.label}>
              Costo standard (€)
            </label>
            <input
              id="nc-cost"
              type="text"
              inputMode="decimal"
              className={styles.input}
              placeholder="Da definire"
              value={draft.defaultCost}
              onChange={(e) => setDraft((d) => ({ ...d, defaultCost: e.target.value }))}
              aria-invalid={Boolean(costError)}
            />
            {costError && <p className={styles.errorText}>{costError}</p>}
          </div>
        </div>

        <div className={styles.field}>
          <label htmlFor="nc-framework" className={styles.label}>Mandatory framework</label>
          <input
            id="nc-framework"
            className={styles.input}
            placeholder="es. GDPR, ISO 27001"
            value={draft.complianceFramework}
            onChange={(e) => setDraft((d) => ({ ...d, complianceFramework: e.target.value }))}
          />
          <p className={styles.hint}>Compila se il corso è obbligatorio e deve essere coperto da una rule.</p>
        </div>

        <div className={styles.field}>
          <label htmlFor="nc-description" className={styles.label}>Descrizione</label>
          <textarea
            id="nc-description"
            className={styles.textarea}
            rows={3}
            value={draft.description}
            onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))}
          />
        </div>

        <div className={styles.footer}>
          <Button type="button" variant="ghost" size="md" onClick={onClose}>Annulla</Button>
          <Button type="submit" variant="primary" size="md" loading={create.isPending} disabled={!canSubmit}>
            Crea corso
          </Button>
        </div>
      </form>
    </Modal>
  );
}
