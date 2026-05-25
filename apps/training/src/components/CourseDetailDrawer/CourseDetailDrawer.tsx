import { useEffect, useState } from 'react';
import { Button, Drawer, SingleSelect, useToast } from '@mrsmith/ui';
import { useArchiveCourse, useTrainingLookups, useUpdateCourse } from '../../api/queries';
import type { CatalogCourseWithCounts } from '../../api/types';
import styles from './CourseDetailDrawer.module.css';

interface CourseDetailDrawerProps {
  course: CatalogCourseWithCounts | null;
  isPeopleAdmin: boolean;
  currentYear: number;
  onClose: () => void;
}

const DELIVERY_OPTIONS = [
  { value: 'classroom', label: 'Aula' },
  { value: 'online_live', label: 'Online live' },
  { value: 'online_self', label: 'Self-paced' },
  { value: 'on_the_job', label: 'On the job' },
  { value: 'mixed', label: 'Misto' },
];

const PROVIDER_OPTIONS = [
  { value: 'internal', label: 'Interna' },
  { value: 'external', label: 'Esterna' },
];

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

export function CourseDetailDrawer({ course, isPeopleAdmin, currentYear, onClose }: CourseDetailDrawerProps) {
  const { toast } = useToast();
  const lookups = useTrainingLookups(isPeopleAdmin);
  const update = useUpdateCourse(isPeopleAdmin);
  const archive = useArchiveCourse();

  const [title, setTitle] = useState('');
  const [vendorId, setVendorId] = useState<string | null>(null);
  const [skillAreaId, setSkillAreaId] = useState<string | null>(null);
  const [deliveryMode, setDeliveryMode] = useState('mixed');
  const [providerKind, setProviderKind] = useState('external');
  const [defaultHours, setDefaultHours] = useState('');
  const [defaultCost, setDefaultCost] = useState('');
  const [complianceRelated, setComplianceRelated] = useState(false);
  const [complianceFramework, setComplianceFramework] = useState('');
  const [description, setDescription] = useState('');

  useEffect(() => {
    if (!course) return;
    setTitle(course.title);
    setVendorId(course.vendorId ?? null);
    setSkillAreaId(course.skillAreaId ?? null);
    setDeliveryMode(course.deliveryMode);
    setProviderKind(course.providerKind);
    setDefaultHours(course.defaultHours !== undefined ? String(course.defaultHours) : '');
    setDefaultCost(course.defaultCost !== undefined ? String(course.defaultCost) : '');
    setComplianceRelated(course.complianceRelated);
    setComplianceFramework(course.complianceFramework ?? '');
    setDescription(course.description ?? '');
  }, [course]);

  if (!course) {
    return (
      <Drawer open={false} onClose={onClose} size="lg">
        {null}
      </Drawer>
    );
  }

  const vendorOptions = (lookups.data?.vendors ?? []).filter((v) => v.active).map((v) => ({ value: v.id, label: v.label }));
  const skillOptions = (lookups.data?.skillAreas ?? []).filter((s) => s.active).map((s) => ({ value: s.id, label: s.label }));
  const vendorRequired = providerKind === 'external';
  const frameworkRequired = complianceRelated;
  const canSave =
    isPeopleAdmin &&
    title.trim().length > 0 &&
    (!vendorRequired || Boolean(vendorId)) &&
    (!frameworkRequired || complianceFramework.trim().length > 0) &&
    !update.isPending;

  async function handleSave() {
    if (!isPeopleAdmin || !course) return;
    if (!canSave) {
      toast('Completa i campi obbligatori', 'warning');
      return;
    }
    try {
      await update.mutateAsync({
        id: course.id,
        body: {
          title: title.trim(),
          vendorId: vendorId ?? undefined,
          skillAreaId: skillAreaId ?? undefined,
          deliveryMode,
          providerKind,
          defaultHours: defaultHours ? Number(defaultHours) : undefined,
          defaultCost: defaultCost ? Number(defaultCost.replace(',', '.')) : undefined,
          description: description.trim() || undefined,
          complianceRelated,
          complianceFramework: complianceRelated ? complianceFramework.trim() : undefined,
          active: course.active,
        },
      });
      toast('Corso aggiornato');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Errore nel salvataggio', 'error');
    }
  }

  async function handleArchive() {
    if (!course) return;
    if (!window.confirm(`Disattivare il corso "${course.title}"? Le iscrizioni storiche restano leggibili.`)) {
      return;
    }
    try {
      await archive.mutateAsync(course.id);
      toast('Corso disattivato');
      onClose();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Errore nella disattivazione', 'error');
    }
  }

  return (
    <Drawer
      open
      onClose={onClose}
      size="lg"
      title={course.title}
      subtitle={
        <span className={styles.subtitle}>
          {course.active ? 'Attivo' : 'Disattivato'} ·{' '}
          {course.enrollments_current_year} iscritti {currentYear} · {course.enrollments_completed_historical} completati
        </span>
      }
      footer={
        <div className={styles.footer}>
          {isPeopleAdmin && course.active && (
            <Button variant="danger" size="md" onClick={handleArchive} loading={archive.isPending}>
              Disattiva corso
            </Button>
          )}
          <div className={styles.footerRight}>
            <Button variant="ghost" size="md" onClick={onClose}>Chiudi</Button>
            {isPeopleAdmin && (
              <Button variant="primary" size="md" onClick={handleSave} loading={update.isPending} disabled={!canSave}>
                Salva
              </Button>
            )}
          </div>
        </div>
      }
    >
      <div className={styles.body}>
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Metadati</h3>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="cd-title">Titolo</label>
            <input
              id="cd-title"
              className={styles.input}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              disabled={!isPeopleAdmin}
            />
          </div>
          <div className={styles.gridTwo}>
            <div className={styles.field}>
              <label className={styles.label}>Skill area</label>
              <SingleSelect
                options={skillOptions}
                selected={skillAreaId}
                onChange={(v) => setSkillAreaId(v ?? null)}
                allowClear
                placeholder="Seleziona"
                disabled={!isPeopleAdmin}
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>Modalità</label>
              <SingleSelect
                options={DELIVERY_OPTIONS}
                selected={deliveryMode}
                onChange={(v) => setDeliveryMode(v ?? 'mixed')}
                disabled={!isPeopleAdmin}
              />
            </div>
          </div>
          <div className={styles.gridTwo}>
            <div className={styles.field}>
              <label className={styles.label}>Erogazione</label>
              <SingleSelect
                options={PROVIDER_OPTIONS}
                selected={providerKind}
                onChange={(v) => setProviderKind(v ?? 'external')}
                disabled={!isPeopleAdmin}
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>
                Fornitore {vendorRequired && <span className={styles.req} aria-hidden="true" />}
              </label>
              <SingleSelect
                options={vendorOptions}
                selected={vendorId}
                onChange={(v) => setVendorId(v ?? null)}
                allowClear
                placeholder="Seleziona"
                disabled={!isPeopleAdmin}
              />
            </div>
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Durata · Costo</label>
            <div className={styles.inlineInputs}>
              <input
                className={styles.input}
                type="number"
                min={1}
                value={defaultHours}
                onChange={(e) => setDefaultHours(e.target.value)}
                placeholder="ore"
                disabled={!isPeopleAdmin}
              />
              <input
                className={styles.input}
                inputMode="decimal"
                value={defaultCost}
                onChange={(e) => setDefaultCost(e.target.value)}
                placeholder="€"
                disabled={!isPeopleAdmin}
              />
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Compliance</h3>
          <div className={styles.complianceGrid}>
            <label className={styles.toggle}>
              <input
                type="checkbox"
                checked={complianceRelated}
                onChange={(e) => setComplianceRelated(e.target.checked)}
                disabled={!isPeopleAdmin}
              />
              <span>Corso compliance</span>
            </label>
            <div className={styles.field}>
              <label htmlFor="cd-framework" className={styles.label}>
                Framework compliance {frameworkRequired && <span className={styles.req} aria-hidden="true" />}
              </label>
              <input
                id="cd-framework"
                className={styles.input}
                placeholder="es. GDPR, ISO 27001"
                value={complianceFramework}
                onChange={(e) => setComplianceFramework(e.target.value)}
                disabled={!isPeopleAdmin || !complianceRelated}
                required={frameworkRequired}
              />
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <div className={styles.field}>
            <label htmlFor="cd-desc" className={styles.label}>Descrizione</label>
            <textarea
              id="cd-desc"
              className={styles.textarea}
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={!isPeopleAdmin}
            />
          </div>
        </section>

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Storico iscrizioni</h3>
          <div className={styles.historyRow}>
            <div className={styles.historyMetric}>
              <span className={styles.historyValue}>{course.enrollments_current_year}</span>
              <span className={styles.historyLabel}>iscritti {currentYear}</span>
            </div>
            <div className={styles.historyMetric}>
              <span className={styles.historyValue}>{course.enrollments_completed_historical}</span>
              <span className={styles.historyLabel}>completati storico</span>
            </div>
            {course.defaultCost !== undefined && (
              <div className={styles.historyMetric}>
                <span className={styles.historyValue}>{formatEuro(course.defaultCost)}</span>
                <span className={styles.historyLabel}>costo medio iscrizione</span>
              </div>
            )}
          </div>
        </section>
      </div>
    </Drawer>
  );
}
