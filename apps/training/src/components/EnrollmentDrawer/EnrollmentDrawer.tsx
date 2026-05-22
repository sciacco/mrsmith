import { useEffect, useMemo, useState } from 'react';
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
import {
  costPerHour,
  formatEuro2,
  formatEuroCompact,
  isDirty as draftIsDirty,
  type EnrollmentDraft,
} from '../../lib/enrollmentDerived';
import { enrollmentStatusLabel, enrollmentStatusTone } from '../../lib/enrollmentStatus.js';
import styles from './EnrollmentDrawer.module.css';

interface EnrollmentDrawerProps {
  enrollment: PlanEnrollment | null;
  isPeopleAdmin: boolean;
  onClose: () => void;
}

const ALERT_LABEL: Record<string, string> = {
  critical: 'Critico',
  warning: 'Attenzione',
  info: 'Info',
};

const LEVEL_OPTIONS = [0, 1, 2, 3, 4, 5];
const PRIORITY_OPTIONS = [1, 2, 3, 4, 5];

const emptyDraft: EnrollmentDraft = {
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

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0] ?? '';
  if (parts.length === 1) return first.slice(0, 2).toUpperCase() || '?';
  const last = parts[parts.length - 1] ?? '';
  return ((first[0] ?? '') + (last[0] ?? '')).toUpperCase() || '?';
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

interface SegmentedProps {
  options: number[];
  value: string;
  onChange: (next: string) => void;
  readOnly?: boolean;
  ariaLabel: string;
}

function Segmented({ options, value, onChange, readOnly, ariaLabel }: SegmentedProps) {
  const selected = value.trim() === '' ? null : Number(value);
  return (
    <div
      className={`${styles.segmented} ${readOnly ? styles.segmentedReadonly : ''}`}
      role="radiogroup"
      aria-label={ariaLabel}
    >
      {options.map((opt) => {
        const active = selected === opt;
        return (
          <button
            key={opt}
            type="button"
            role="radio"
            aria-checked={active}
            disabled={readOnly}
            className={`${styles.segmentedBtn} ${active ? styles.segmentedActive : ''}`}
            onClick={() => {
              if (readOnly) return;
              onChange(active ? '' : String(opt));
            }}
          >
            {opt}
          </button>
        );
      })}
    </div>
  );
}

export function EnrollmentDrawer({ enrollment, isPeopleAdmin, onClose }: EnrollmentDrawerProps) {
  const { toast } = useToast();
  const [draft, setDraft] = useState<EnrollmentDraft>(emptyDraft);
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

  const dirty = useMemo(() => (enrollment ? draftIsDirty(draft, enrollment) : false), [draft, enrollment]);

  if (!enrollment) {
    return <Drawer open={false} onClose={onClose} size="lg">{null}</Drawer>;
  }

  const alertLevel = classifyAlertLevel(enrollment);
  const transitions = transitionsFor(enrollment, isPeopleAdmin);
  const primaryTransition = transitions.find((t) => t.primary);
  const secondaryTransitions = transitions.filter((t) => !t.primary);
  const requiresReason = (t: TransitionDef) =>
    t.transition === 'revert_to_proposed' || t.transition === 'reopen' || t.transition === 'cancel';

  const perHour = costPerHour(draft.costPlanned, draft.hoursPlanned);

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

  function handleSave(event?: React.FormEvent | React.MouseEvent) {
    event?.preventDefault();
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
  const statusLabel = enrollmentStatusLabel(enrollment.status);
  const tone = enrollmentStatusTone(enrollment.status);
  const statusClass = tone ? styles[`status_${tone}`] ?? '' : '';

  const subtitle = (
    <div className={styles.subtitle}>
      <span className={styles.subtitleAvatar} aria-hidden>{initials(enrollment.employeeName)}</span>
      <span className={styles.subtitleName}>{enrollment.employeeName}</span>
      <span className={styles.subtitleEmail}>{enrollment.employeeEmail}</span>
    </div>
  );

  const headerExtra = (
    <span className={`${styles.statusPill} ${statusClass}`}>{statusLabel}</span>
  );

  return (
    <Drawer
      open={enrollment !== null}
      onClose={onClose}
      size="lg"
      title={enrollment.courseTitle}
      subtitle={subtitle}
      headerExtra={headerExtra}
      footer={
        <div className={styles.footer}>
          {isPeopleAdmin && dirty && (
            <span className={styles.footerLeft}>Modifiche non salvate</span>
          )}
          {isPeopleAdmin && (
            <Button variant="secondary" onClick={handleSave} disabled={pending || !dirty}>
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
        {/* Meta chips — anno / team / area / tipo + alert */}
        <div className={styles.metaChips}>
          <span className={styles.chip}><span className={styles.chipKey}>Anno</span> {enrollment.year}</span>
          {enrollment.teamCode && (
            <span className={styles.chip}><span className={styles.chipKey}>Team</span> {enrollment.teamCode}</span>
          )}
          {enrollment.skillAreaName && (
            <span className={styles.chip}><span className={styles.chipKey}>Area</span> {enrollment.skillAreaName}</span>
          )}
          {enrollment.mandatory && (
            <span className={`${styles.chip} ${styles.chipMandatory}`}>Obbligatoria</span>
          )}
          <span className={`${styles.alertChip} ${styles[`alert_${alertLevel}`]}`}>
            {ALERT_LABEL[alertLevel]}
          </span>
        </div>

        {/* Sticky action ribbon */}
        {(primaryTransition || secondaryTransitions.length > 0) && (
          <div className={styles.ribbon}>
            <span className={styles.ribbonHint}>
              {enrollment.status === 'proposed' && 'Decidi se approvare o annullare la proposta.'}
              {enrollment.status === 'approved' && 'Quando il corso parte, avvialo per registrare le ore.'}
              {enrollment.status === 'in_progress' && 'Completa quando il corso è terminato.'}
            </span>
            <div className={styles.ribbonActions}>
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
              {primaryTransition && (
                <Button
                  variant={primaryTransition.variant ?? 'primary'}
                  size="sm"
                  disabled={pending}
                  onClick={() => handleTransitionClick(primaryTransition)}
                >
                  {primaryTransition.label}
                </Button>
              )}
            </div>
          </div>
        )}

        {/* Card 1: Sintesi formativa */}
        <section className={styles.card}>
          <div className={styles.cardHeader}>
            <h3 className={styles.cardTitle}>Sintesi formativa</h3>
            {enrollment.vendorName && (
              <span className={styles.cardHint}>Fornitore · <strong>{enrollment.vendorName}</strong></span>
            )}
          </div>

          {/* Competence delta */}
          <div className={styles.competence}>
            <div className={styles.competenceLabel}>
              Gap di competenza{enrollment.skillAreaName && (
                <> su <span className={styles.competenceArea}>{enrollment.skillAreaName}</span></>
              )}
            </div>
            <div className={styles.levelRow}>
              <div className={styles.levelGroup}>
                <span className={styles.levelGroupLabel}>Livello attuale</span>
                <Segmented
                  options={LEVEL_OPTIONS}
                  value={draft.levelAsIs}
                  onChange={(v) => setDraft((d) => ({ ...d, levelAsIs: v }))}
                  readOnly={!isPeopleAdmin}
                  ariaLabel="Livello attuale"
                />
              </div>
              <span className={styles.levelArrow} aria-hidden>→</span>
              <div className={styles.levelGroup}>
                <span className={styles.levelGroupLabel}>Livello obiettivo</span>
                <Segmented
                  options={LEVEL_OPTIONS}
                  value={draft.levelToBe}
                  onChange={(v) => setDraft((d) => ({ ...d, levelToBe: v }))}
                  readOnly={!isPeopleAdmin}
                  ariaLabel="Livello obiettivo"
                />
              </div>
            </div>
          </div>

          {/* Motivazione + Obiettivo */}
          {(isPeopleAdmin || draft.motivation || draft.objective) && (
            <div className={styles.editorialStack}>
              {(isPeopleAdmin || draft.motivation) && (
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>Motivazione</span>
                  <textarea
                    className={styles.textarea}
                    readOnly={!isPeopleAdmin}
                    value={draft.motivation}
                    placeholder={isPeopleAdmin ? 'Perché questa iscrizione è necessaria?' : ''}
                    onChange={(e) => setDraft((d) => ({ ...d, motivation: e.target.value }))}
                  />
                </label>
              )}
              {(isPeopleAdmin || draft.objective) && (
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>Obiettivo</span>
                  <textarea
                    className={`${styles.textarea} ${styles.textareaLg}`}
                    readOnly={!isPeopleAdmin}
                    value={draft.objective}
                    placeholder={isPeopleAdmin ? 'Cosa la persona saprà fare al termine?' : ''}
                    onChange={(e) => setDraft((d) => ({ ...d, objective: e.target.value }))}
                  />
                </label>
              )}
            </div>
          )}
        </section>

        {/* Card 2: Pianificazione operativa (admin only) */}
        {isPeopleAdmin && (
          <section className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>Pianificazione operativa</h3>
            </div>

            {/* Schedulazione */}
            <div className={styles.planGroup}>
              <span className={styles.planGroupTitle}>Schedulazione</span>
              <div className={styles.dateRange}>
                <label className={`${styles.field} ${styles.dateField}`}>
                  <span className={styles.fieldLabel}>Inizio</span>
                  <input
                    type="date"
                    value={draft.plannedStart}
                    onChange={(e) => setDraft((d) => ({ ...d, plannedStart: e.target.value }))}
                  />
                </label>
                <span className={styles.dateArrow} aria-hidden>→</span>
                <label className={`${styles.field} ${styles.dateField}`}>
                  <span className={styles.fieldLabel}>Fine</span>
                  <input
                    type="date"
                    value={draft.plannedEnd}
                    onChange={(e) => setDraft((d) => ({ ...d, plannedEnd: e.target.value }))}
                  />
                </label>
              </div>
            </div>

            {/* Impegno & Budget */}
            <div className={styles.planGroup}>
              <span className={styles.planGroupTitle}>Impegno &amp; budget</span>
              <div className={styles.budgetGrid}>
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>Ore pianificate</span>
                  <span className={styles.inputAddon}>
                    <input
                      type="number"
                      min={1}
                      value={draft.hoursPlanned}
                      onChange={(e) => setDraft((d) => ({ ...d, hoursPlanned: e.target.value }))}
                    />
                    <span className={styles.inputAddonSuffix}>h</span>
                  </span>
                </label>
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>Costo previsto</span>
                  <span className={styles.inputAddon}>
                    <span className={styles.inputAddonPrefix}>€</span>
                    <input
                      type="number"
                      min={0}
                      step="0.01"
                      value={draft.costPlanned}
                      onChange={(e) => setDraft((d) => ({ ...d, costPlanned: e.target.value }))}
                    />
                  </span>
                </label>
              </div>
              {perHour !== undefined && (
                <div className={styles.derivedRow}>
                  Costo orario derivato <strong>€ {formatEuro2(perHour)} / h</strong>
                  {draft.costPlanned && (
                    <> · totale <strong>€ {formatEuroCompact(Number(draft.costPlanned))}</strong></>
                  )}
                </div>
              )}
            </div>

            {/* Priorità */}
            <div className={styles.planGroup}>
              <span className={styles.planGroupTitle}>Priorità in coda</span>
              <Segmented
                options={PRIORITY_OPTIONS}
                value={draft.priority}
                onChange={(v) => setDraft((d) => ({ ...d, priority: v }))}
                ariaLabel="Priorità"
              />
            </div>
          </section>
        )}

        {/* Card 3: Documento */}
        <section className={styles.card}>
          <div className={styles.cardHeader}>
            <h3 className={styles.cardTitle}>Documento</h3>
          </div>
          {enrollment.documentFilename ? (
            <div className={styles.docRow}>
              <span className={styles.docIcon} aria-hidden><Icon name="download" size={14} /></span>
              <span className={styles.docName}>{enrollment.documentFilename}</span>
              <span className={`${styles.docBadge} ${enrollment.documentValidated ? styles.docBadgeValid : styles.docBadgePending}`}>
                {enrollment.documentValidated ? 'Validato' : 'Da validare'}
              </span>
              <div className={styles.docActions}>
                <Button variant="ghost" size="sm" disabled={pending} onClick={handleDownload}>
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
            <div className={styles.docEmpty}>
              <span>Nessun documento caricato.</span>
            </div>
          )}
          <label className={styles.upload}>
            <span className={styles.fieldLabel}>Carica documento (PDF)</span>
            <input type="file" accept="application/pdf,.pdf" onChange={handleUpload} disabled={pending} />
          </label>
        </section>

        {/* Note (admin only, collapsible) */}
        {isPeopleAdmin && (
          <section className={styles.card}>
            <details className={styles.notes} open={Boolean(draft.notes)}>
              <summary>Note interne{draft.notes && ' (compilate)'}</summary>
              <textarea
                className={styles.textarea}
                value={draft.notes}
                placeholder="Annotazioni operative non visibili alla persona."
                onChange={(e) => setDraft((d) => ({ ...d, notes: e.target.value }))}
              />
            </details>
          </section>
        )}

        {/* Reason form */}
        {reasonRequest && (
          <section className={styles.reasonForm}>
            <h3>Motivazione richiesta · {reasonRequest.label.toLowerCase()}</h3>
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
