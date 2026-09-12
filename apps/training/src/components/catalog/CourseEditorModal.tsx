// Editor corso di catalogo (#158, §Catalogo 4): upsert completo — erogazione
// interna/esterna, fornitore obbligatorio solo per l'esterna, compliance con
// framework in coppia (stesso idioma di ricorrenza/àncora nelle regole). Il
// toggle «Attiva» copre anche l'attivazione, dopo la revisione, dei corsi
// importati dal sync (corsi inattivi con factorialTrainingId).

import { useState } from 'react';
import { Button, Modal, MoneyInput, MultiSelect, SingleSelect, ToggleSwitch, VisuallyHidden } from '@mrsmith/ui';
import {
  useCreateCourse,
  useTrainingCertifications,
  useTrainingGroups,
  useTrainingPeople,
  useTrainingSkillAreas,
  useTrainingTeams,
  useTrainingVendors,
  useUpdateCourse,
} from '../../api/queries';
import type { CourseDetail, CourseInput, CourseVisibility } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { DELIVERY_MODE_LABELS } from '../../lib/labels';
import styles from '../requests/requestShared.module.css';

const DELIVERY_MODE_OPTIONS = Object.entries(DELIVERY_MODE_LABELS).map(([value, label]) => ({ value, label }));

interface CourseEditorModalProps {
  mode: 'create' | 'edit';
  courseId?: string;
  initial?: CourseDetail;
  open: boolean;
  onClose: () => void;
  onSaved: (id: string) => void;
}

export function CourseEditorModal({ mode, courseId, initial, open, onClose, onSaved }: CourseEditorModalProps) {
  const vendors = useTrainingVendors();
  const skillAreas = useTrainingSkillAreas();
  const certifications = useTrainingCertifications();
  const people = useTrainingPeople();
  const teams = useTrainingTeams();
  const groups = useTrainingGroups();
  const createCourse = useCreateCourse();
  const updateCourse = useUpdateCourse();

  const [title, setTitle] = useState(initial?.title ?? '');
  const [providerKind, setProviderKind] = useState<'internal' | 'external'>(
    (initial?.providerKind as 'internal' | 'external') ?? 'external',
  );
  const [vendorId, setVendorId] = useState(initial?.vendorId ?? '');
  const [skillAreaIds, setSkillAreaIds] = useState<string[]>((initial?.skillAreas ?? []).map((a) => a.id));
  const [leadsToCertId, setLeadsToCertId] = useState(initial?.leadsToCertId ?? '');
  const [deliveryMode, setDeliveryMode] = useState(initial?.deliveryMode || 'mixed');
  const [defaultHours, setDefaultHours] = useState(initial?.defaultHours !== undefined ? String(initial.defaultHours) : '');
  const [defaultCost, setDefaultCost] = useState(initial?.defaultCost !== undefined ? initial.defaultCost.toFixed(2) : '');
  const [courseUrl, setCourseUrl] = useState(initial?.courseUrl ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const [complianceRelated, setComplianceRelated] = useState(initial?.complianceRelated ?? false);
  const [complianceFramework, setComplianceFramework] = useState(initial?.complianceFramework ?? '');
  const [tags, setTags] = useState((initial?.tags ?? []).join(', '));
  const [active, setActive] = useState(initial?.active ?? true);
  const [notes, setNotes] = useState(initial?.notes ?? '');
  const [reminderText, setReminderText] = useState(initial?.reminderText ?? '');
  const [reminderAt, setReminderAt] = useState(initial?.reminderAt ?? '');
  const [trainerIds, setTrainerIds] = useState<string[]>((initial?.trainers ?? []).map((t) => t.employeeId));
  const [visibilityKind, setVisibilityKind] = useState<'' | CourseVisibility['kind']>(initial?.visibility?.kind ?? '');
  const [visibilityId, setVisibilityId] = useState(initial?.visibility?.id ?? '');
  const [visibilityPeople, setVisibilityPeople] = useState<string[]>(initial?.visibility?.employeeIds ?? []);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const pending = createCourse.isPending || updateCourse.isPending;
  const needsVisibilityId = visibilityKind === 'team' || visibilityKind === 'skill_area' || visibilityKind === 'custom_group';
  const canSubmit =
    title.trim() !== '' &&
    (providerKind === 'internal' || vendorId !== '') &&
    (!complianceRelated || complianceFramework.trim() !== '') &&
    (!needsVisibilityId || visibilityId !== '');

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const input: CourseInput = {
      title: title.trim(),
      providerKind,
      vendorId: providerKind === 'external' ? vendorId : undefined,
      skillAreaIds,
      leadsToCertId: leadsToCertId || undefined,
      deliveryMode,
      defaultHours: defaultHours !== '' ? Number(defaultHours) : undefined,
      defaultCost: defaultCost !== '' ? Number(defaultCost) : undefined,
      courseUrl: courseUrl.trim() || undefined,
      description: description.trim() || undefined,
      complianceRelated,
      complianceFramework: complianceRelated ? complianceFramework.trim() : undefined,
      tags: tags.split(',').map((t) => t.trim()).filter(Boolean),
      active,
      notes: notes.trim() || undefined,
      reminderText: reminderText.trim() || undefined,
      reminderAt: reminderAt || undefined,
      trainerIds,
      visibility:
        visibilityKind === ''
          ? undefined
          : {
              kind: visibilityKind,
              id: needsVisibilityId ? visibilityId : undefined,
              employeeIds: visibilityKind === 'people' ? visibilityPeople : undefined,
            },
    };
    try {
      if (mode === 'edit' && courseId) {
        await updateCourse.mutateAsync({ id: courseId, input });
        onSaved(courseId);
      } else {
        const response = await createCourse.mutateAsync(input);
        if (response.id) onSaved(response.id);
      }
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={mode === 'create' ? 'Nuovo corso' : 'Modifica corso'} size="wide">
      <div className={`${styles.body} ${styles.bodyModal}`}>
        <h4 className={styles.sectionTitle}>Informazioni corso</h4>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Titolo
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <input className={styles.input} value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>

        <div className={styles.toggleGroup}>
          <Button
            variant={providerKind === 'external' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setProviderKind('external')}
          >
            Erogazione esterna
          </Button>
          <Button
            variant={providerKind === 'internal' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => {
              setProviderKind('internal');
              setVendorId('');
            }}
          >
            Erogazione interna
          </Button>
        </div>

        <div className={styles.row}>
          {providerKind === 'external' && (
            <label className={styles.field}>
              <span className={styles.labelHead}>
                Fornitore abituale
                <span className={styles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </span>
              <SingleSelect
                options={(vendors.data ?? []).filter((v) => v.active).map((v) => ({ value: v.id, label: v.name }))}
                selected={vendorId || null}
                onChange={(v) => setVendorId(v ?? '')}
                placeholder="Seleziona fornitore..."
                searchable
              />
            </label>
          )}
          <label className={styles.field}>
            Modalità
            <SingleSelect
              options={DELIVERY_MODE_OPTIONS}
              selected={deliveryMode}
              onChange={(v) => setDeliveryMode(v ?? 'mixed')}
            />
          </label>
        </div>

        <div className={styles.row}>
          <label className={styles.field}>
            Aree di competenza
            <MultiSelect<string>
              options={(skillAreas.data ?? []).filter((a) => a.active).map((a) => ({ value: a.id, label: a.name }))}
              selected={skillAreaIds}
              onChange={setSkillAreaIds}
              placeholder="Nessuna"
            />
          </label>
          <label className={styles.field}>
            Certificazione collegata
            <SingleSelect
              options={(certifications.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.name }))}
              selected={leadsToCertId || null}
              onChange={(v) => setLeadsToCertId(v ?? '')}
              placeholder="Nessuna"
              allowClear
              searchable
            />
          </label>
        </div>

        <div className={styles.row}>
          <label className={styles.field}>
            Durata indicativa (ore)
            <input
              type="number"
              min={0}
              className={styles.input}
              value={defaultHours}
              onChange={(e) => setDefaultHours(e.target.value)}
            />
          </label>
          <MoneyInput label="Prezzo indicativo" value={defaultCost} onChange={setDefaultCost} />
        </div>

        <label className={styles.field}>
          URL del corso
          <input className={styles.input} value={courseUrl} onChange={(e) => setCourseUrl(e.target.value)} />
        </label>

        <label className={styles.field}>
          Descrizione
          <textarea className={styles.textarea} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </label>

        <label className={styles.field}>
          Tag (separati da virgola)
          <input className={styles.input} value={tags} onChange={(e) => setTags(e.target.value)} />
        </label>

        <label className={styles.field}>
          Formatori interni designati
          <MultiSelect<string>
            options={(people.data ?? [])
              .filter((p) => p.status === 'active')
              .map((p) => ({ value: p.id, label: `${p.lastName} ${p.firstName}` }))}
            selected={trainerIds}
            onChange={setTrainerIds}
            placeholder="Nessuno"
          />
          <span className={styles.hint}>Alla creazione di un evento diventano il valore di partenza dei suoi formatori.</span>
        </label>

        <h4 className={styles.sectionTitle}>Pianificazione</h4>
        <div className={styles.row}>
          <label className={styles.field}>
            Promemoria
            <input className={styles.input} value={reminderText} onChange={(e) => setReminderText(e.target.value)} placeholder="In attesa di / prossimo passo" />
          </label>
          <label className={styles.field}>
            Data di richiamo (facoltativa)
            <input type="date" className={styles.input} value={reminderAt} onChange={(e) => setReminderAt(e.target.value)} />
          </label>
        </div>

        <label className={styles.field}>
          Nota
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>

        <h4 className={styles.sectionTitle}>Visibilità e stato</h4>
        <div className={styles.row}>
          <label className={styles.field}>
            Visibilità (chi può scegliere il corso)
            <SingleSelect
              options={[
                { value: 'all', label: 'Tutti' },
                { value: 'team', label: 'Un team' },
                { value: 'skill_area', label: "Un'area di competenza" },
                { value: 'custom_group', label: 'Un gruppo' },
                { value: 'people', label: 'Persone scelte' },
              ]}
              selected={visibilityKind || null}
              onChange={(v) => {
                setVisibilityKind((v as CourseVisibility['kind']) ?? '');
                setVisibilityId('');
                setVisibilityPeople([]);
              }}
              placeholder="Riservato a People"
              allowClear
            />
            <span className={styles.hint}>Riservata al futuro self-service. People vede tutti i corsi.</span>
          </label>
          {visibilityKind === 'team' && (
            <label className={styles.field}>
              Team
              <SingleSelect
                options={(teams.data ?? []).map((t) => ({ value: t.id, label: t.name }))}
                selected={visibilityId || null}
                onChange={(v) => setVisibilityId(v ?? '')}
                placeholder="Seleziona team..."
              />
            </label>
          )}
          {visibilityKind === 'skill_area' && (
            <label className={styles.field}>
              Area
              <SingleSelect
                options={(skillAreas.data ?? []).map((a) => ({ value: a.id, label: a.name }))}
                selected={visibilityId || null}
                onChange={(v) => setVisibilityId(v ?? '')}
                placeholder="Seleziona area..."
                searchable
              />
            </label>
          )}
          {visibilityKind === 'custom_group' && (
            <label className={styles.field}>
              Gruppo
              <SingleSelect
                options={(groups.data ?? []).map((g) => ({ value: g.id, label: g.name }))}
                selected={visibilityId || null}
                onChange={(v) => setVisibilityId(v ?? '')}
                placeholder="Seleziona gruppo..."
              />
            </label>
          )}
          {visibilityKind === 'people' && (
            <label className={styles.field}>
              Persone
              <MultiSelect<string>
                options={(people.data ?? [])
                  .filter((p) => p.status === 'active')
                  .map((p) => ({ value: p.id, label: `${p.lastName} ${p.firstName}` }))}
                selected={visibilityPeople}
                onChange={setVisibilityPeople}
                placeholder="Nessuna"
              />
            </label>
          )}
        </div>

        <div className={styles.row}>
          <div className={styles.field}>
            Corso di compliance
            <ToggleSwitch id="course-compliance" checked={complianceRelated} onChange={setComplianceRelated} />
          </div>
          {complianceRelated && (
            <label className={styles.field}>
              <span className={styles.labelHead}>
                Framework compliance
                <span className={styles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </span>
              <input
                className={styles.input}
                value={complianceFramework}
                onChange={(e) => setComplianceFramework(e.target.value)}
              />
            </label>
          )}
        </div>

        <div className={styles.field}>
          Attivo
          <ToggleSwitch id="course-active" checked={active} onChange={setActive} />
        </div>

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={!canSubmit} onClick={submit}>
            {mode === 'create' ? 'Crea corso' : 'Salva modifiche'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
