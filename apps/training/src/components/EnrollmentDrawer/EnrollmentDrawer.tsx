import { useEffect, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Drawer, Icon, useToast } from '@mrsmith/ui';
import {
  useDownloadDocument,
  useEnrollmentTransition,
  useUpdateEnrollment,
  useUploadEnrollmentDocument,
  useValidateDocument,
} from '../../api/queries';
import type { PlanEnrollment } from '../../api/types';
import { classifyAlertLevel } from '../../lib/alertLevel';
import styles from './EnrollmentDrawer.module.css';

interface EnrollmentDrawerProps {
  enrollment: PlanEnrollment | null;
  isPeopleAdmin: boolean;
  onClose: () => void;
}

const STATUS_LABELS: Record<string, string> = {
  proposed: 'Proposta',
  approved: 'Approvata',
  in_progress: 'In corso',
  completed: 'Completata',
  failed: 'Non superata',
  cancelled: 'Annullata',
  expired: 'Scaduta',
};

const ALERT_LABEL: Record<string, string> = {
  critical: 'Critico',
  warning: 'Attenzione',
  info: 'Info',
};

const emptyDraft = {
  priority: '',
  levelAsIs: '',
  levelToBe: '',
  plannedStart: '',
  plannedEnd: '',
  hoursPlanned: '',
  costPlanned: '',
  motivation: '',
  objective: '',
  notes: '',
};

function numberDraft(value: number | undefined): string {
  return value === undefined ? '' : String(value);
}

function optionalNumber(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === '') return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function formatDateValue(value: string | undefined): string {
  if (!value) return '';
  return value.slice(0, 10);
}

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
  }
  return fallback;
}

interface TransitionDef {
  transition: string;
  label: string;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  primary?: boolean;
}

function transitionsFor(enrollment: PlanEnrollment, isPeopleAdmin: boolean): TransitionDef[] {
  if (!isPeopleAdmin) {
    if (enrollment.status === 'approved') return [{ transition: 'start', label: 'Avvia', variant: 'primary', primary: true }];
    if (enrollment.status === 'in_progress')
      return [{ transition: 'complete', label: 'Completa', variant: 'primary', primary: true }];
    return [];
  }
  switch (enrollment.status) {
    case 'proposed':
      return [
        { transition: 'approve', label: 'Approva', variant: 'primary', primary: true },
        { transition: 'cancel', label: 'Annulla', variant: 'danger' },
      ];
    case 'approved':
      return [
        { transition: 'start', label: 'Avvia', variant: 'primary', primary: true },
        { transition: 'revert_to_proposed', label: 'Riporta a proposta', variant: 'secondary' },
        { transition: 'cancel', label: 'Annulla', variant: 'danger' },
      ];
    case 'in_progress':
      return [
        { transition: 'complete', label: 'Completa', variant: 'primary', primary: true },
        { transition: 'fail', label: 'Non superata', variant: 'secondary' },
        { transition: 'cancel', label: 'Annulla', variant: 'danger' },
      ];
    case 'completed':
    case 'failed':
    case 'cancelled':
    case 'expired':
      return [{ transition: 'reopen', label: 'Riapri', variant: 'secondary' }];
    default:
      return [];
  }
}

export function EnrollmentDrawer({ enrollment, isPeopleAdmin, onClose }: EnrollmentDrawerProps) {
  const { toast } = useToast();
  const [draft, setDraft] = useState(emptyDraft);
  const [reasonRequest, setReasonRequest] = useState<TransitionDef | null>(null);
  const [reason, setReason] = useState('');

  const transition = useEnrollmentTransition(isPeopleAdmin);
  const update = useUpdateEnrollment(isPeopleAdmin);
  const uploadDoc = useUploadEnrollmentDocument(isPeopleAdmin);
  const validateDoc = useValidateDocument(isPeopleAdmin);
  const downloadDoc = useDownloadDocument();

  useEffect(() => {
    if (!enrollment) return;
    setDraft({
      priority: numberDraft(enrollment.priority),
      levelAsIs: numberDraft(enrollment.levelAsIs),
      levelToBe: numberDraft(enrollment.levelToBe),
      plannedStart: formatDateValue(enrollment.plannedStart),
      plannedEnd: formatDateValue(enrollment.plannedEnd),
      hoursPlanned: numberDraft(enrollment.hoursPlanned),
      costPlanned: numberDraft(enrollment.costPlanned),
      motivation: enrollment.motivation ?? '',
      objective: enrollment.objective ?? '',
      notes: enrollment.notes ?? '',
    });
    setReasonRequest(null);
    setReason('');
  }, [enrollment]);

  if (!enrollment) {
    return <Drawer open={false} onClose={onClose} size="lg">{null}</Drawer>;
  }

  const alertLevel = classifyAlertLevel(enrollment);
  const transitions = transitionsFor(enrollment, isPeopleAdmin);
  const primaryTransition = transitions.find((t) => t.primary);
  const secondaryTransitions = transitions.filter((t) => !t.primary);
  const requiresReason = (t: TransitionDef) => t.transition === 'revert_to_proposed' || t.transition === 'reopen' || t.transition === 'cancel';

  function runTransition(t: TransitionDef, reasonText?: string) {
    if (!enrollment) return;
    transition.mutate(
      { id: enrollment.id, transition: t.transition, reason: reasonText },
      {
        onSuccess: () => {
          toast(`${t.label} eseguita`);
          setReasonRequest(null);
          setReason('');
        },
        onError: (error) => toast(apiErrorMessage(error, 'Transizione non riuscita'), 'error'),
      },
    );
  }

  function handleTransitionClick(t: TransitionDef) {
    if (requiresReason(t)) {
      setReasonRequest(t);
      setReason('');
      return;
    }
    runTransition(t);
  }

  function handleSave(event: React.FormEvent) {
    event.preventDefault();
    if (!enrollment) return;
    update.mutate(
      {
        id: enrollment.id,
        body: {
          priority: optionalNumber(draft.priority),
          levelAsIs: optionalNumber(draft.levelAsIs),
          levelToBe: optionalNumber(draft.levelToBe),
          plannedStart: draft.plannedStart,
          plannedEnd: draft.plannedEnd,
          hoursPlanned: optionalNumber(draft.hoursPlanned),
          costPlanned: optionalNumber(draft.costPlanned),
          motivation: draft.motivation,
          objective: draft.objective,
          notes: draft.notes,
        },
      },
      {
        onSuccess: () => toast('Iscrizione aggiornata'),
        onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento non riuscito'), 'error'),
      },
    );
  }

  function handleUpload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file || !enrollment) return;
    uploadDoc.mutate(
      { enrollmentId: enrollment.id, file },
      {
        onSuccess: () => toast('Documento caricato'),
        onError: (error) => toast(apiErrorMessage(error, 'Caricamento non riuscito'), 'error'),
      },
    );
  }

  function handleValidate() {
    if (!enrollment?.documentId) return;
    validateDoc.mutate(enrollment.documentId, {
      onSuccess: () => toast('Documento validato'),
      onError: (error) => toast(apiErrorMessage(error, 'Validazione non riuscita'), 'error'),
    });
  }

  function handleDownload() {
    if (!enrollment?.documentId) return;
    downloadDoc.mutate(
      { documentId: enrollment.documentId, filename: enrollment.documentFilename || 'documento-formazione.pdf' },
      { onError: (error) => toast(apiErrorMessage(error, 'Download non riuscito'), 'error') },
    );
  }

  const pending = transition.isPending || update.isPending || uploadDoc.isPending || validateDoc.isPending;

  return (
    <Drawer
      open={enrollment !== null}
      onClose={onClose}
      size="lg"
      title={enrollment.courseTitle}
      subtitle={`${enrollment.employeeName} · ${enrollment.employeeEmail}`}
      footer={
        <div className={styles.footer}>
          {isPeopleAdmin && (
            <Button variant="secondary" onClick={handleSave} disabled={pending}>
              Salva modifiche
            </Button>
          )}
          {primaryTransition && (
            <Button
              variant={primaryTransition.variant ?? 'primary'}
              onClick={() => handleTransitionClick(primaryTransition)}
              disabled={pending}
            >
              {primaryTransition.label}
            </Button>
          )}
        </div>
      }
    >
      <div className={styles.body}>
        <section className={styles.summary}>
          <div className={`${styles.alertBadge} ${styles[`alert_${alertLevel}`]}`}>
            <span className={styles.alertDot} aria-hidden /> {ALERT_LABEL[alertLevel]}
          </div>
          <dl className={styles.meta}>
            <div>
              <dt>Stato</dt>
              <dd>{STATUS_LABELS[enrollment.status] ?? enrollment.status}</dd>
            </div>
            <div>
              <dt>Anno</dt>
              <dd>{enrollment.year}</dd>
            </div>
            {enrollment.teamCode && (
              <div>
                <dt>Team</dt>
                <dd>{enrollment.teamCode}</dd>
              </div>
            )}
            {enrollment.vendorName && (
              <div>
                <dt>Fornitore</dt>
                <dd>{enrollment.vendorName}</dd>
              </div>
            )}
            {enrollment.skillAreaName && (
              <div>
                <dt>Area</dt>
                <dd>{enrollment.skillAreaName}</dd>
              </div>
            )}
            {enrollment.mandatory && (
              <div>
                <dt>Tipo</dt>
                <dd>Obbligatoria</dd>
              </div>
            )}
          </dl>
        </section>

        {isPeopleAdmin && (
          <section className={styles.section}>
            <h3>Pianificazione</h3>
            <div className={styles.grid}>
              <label>
                <span>Priorità</span>
                <input
                  type="number"
                  min={1}
                  value={draft.priority}
                  onChange={(e) => setDraft((d) => ({ ...d, priority: e.target.value }))}
                />
              </label>
              <label>
                <span>Ore pianificate</span>
                <input
                  type="number"
                  min={1}
                  value={draft.hoursPlanned}
                  onChange={(e) => setDraft((d) => ({ ...d, hoursPlanned: e.target.value }))}
                />
              </label>
              <label>
                <span>Costo</span>
                <input
                  type="number"
                  min={0}
                  step="0.01"
                  value={draft.costPlanned}
                  onChange={(e) => setDraft((d) => ({ ...d, costPlanned: e.target.value }))}
                />
              </label>
              <label>
                <span>Livello attuale</span>
                <input
                  type="number"
                  min={0}
                  value={draft.levelAsIs}
                  onChange={(e) => setDraft((d) => ({ ...d, levelAsIs: e.target.value }))}
                />
              </label>
              <label>
                <span>Livello obiettivo</span>
                <input
                  type="number"
                  min={0}
                  value={draft.levelToBe}
                  onChange={(e) => setDraft((d) => ({ ...d, levelToBe: e.target.value }))}
                />
              </label>
              <label>
                <span>Inizio</span>
                <input
                  type="date"
                  value={draft.plannedStart}
                  onChange={(e) => setDraft((d) => ({ ...d, plannedStart: e.target.value }))}
                />
              </label>
              <label>
                <span>Fine</span>
                <input
                  type="date"
                  value={draft.plannedEnd}
                  onChange={(e) => setDraft((d) => ({ ...d, plannedEnd: e.target.value }))}
                />
              </label>
            </div>
            <div className={styles.textStack}>
              <label>
                <span>Motivazione</span>
                <textarea
                  value={draft.motivation}
                  onChange={(e) => setDraft((d) => ({ ...d, motivation: e.target.value }))}
                />
              </label>
              <label>
                <span>Obiettivo</span>
                <textarea
                  value={draft.objective}
                  onChange={(e) => setDraft((d) => ({ ...d, objective: e.target.value }))}
                />
              </label>
              <label>
                <span>Note</span>
                <textarea
                  value={draft.notes}
                  onChange={(e) => setDraft((d) => ({ ...d, notes: e.target.value }))}
                />
              </label>
            </div>
          </section>
        )}

        <section className={styles.section}>
          <h3>Documento</h3>
          {enrollment.documentFilename ? (
            <div className={styles.docRow}>
              <strong>{enrollment.documentFilename}</strong>
              <span className={styles.docBadge}>{enrollment.documentValidated ? 'Validato' : 'Da validare'}</span>
              <div className={styles.docActions}>
                <Button variant="ghost" size="sm" leftIcon={<Icon name="download" size={14} />} disabled={pending} onClick={handleDownload}>
                  Scarica
                </Button>
                {isPeopleAdmin && !enrollment.documentValidated && (
                  <Button variant="ghost" size="sm" disabled={pending} onClick={handleValidate}>
                    Valida
                  </Button>
                )}
              </div>
            </div>
          ) : (
            <p className={styles.muted}>Nessun documento caricato.</p>
          )}
          <label className={styles.upload}>
            <span>Carica documento (PDF)</span>
            <input type="file" accept="application/pdf,.pdf" onChange={handleUpload} disabled={pending} />
          </label>
        </section>

        {secondaryTransitions.length > 0 && (
          <section className={styles.section}>
            <h3>Altre azioni</h3>
            <div className={styles.actionsRow}>
              {secondaryTransitions.map((t) => (
                <Button
                  key={t.transition}
                  variant={t.variant ?? 'ghost'}
                  size="sm"
                  disabled={pending}
                  onClick={() => handleTransitionClick(t)}
                >
                  {t.label}
                </Button>
              ))}
            </div>
          </section>
        )}

        {reasonRequest && (
          <section className={styles.reasonForm}>
            <h3>Motivazione richiesta</h3>
            <textarea
              autoFocus
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Indica la motivazione"
              rows={3}
            />
            <div className={styles.actionsRow}>
              <Button variant="ghost" size="sm" onClick={() => { setReasonRequest(null); setReason(''); }}>
                Annulla
              </Button>
              <Button
                variant={reasonRequest.variant ?? 'primary'}
                size="sm"
                disabled={pending || reason.trim() === ''}
                onClick={() => runTransition(reasonRequest, reason)}
              >
                Conferma {reasonRequest.label.toLowerCase()}
              </Button>
            </div>
          </section>
        )}
      </div>
    </Drawer>
  );
}
