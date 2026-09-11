// Dettaglio richiesta (#157, §Richieste 2/4/5/6; #171; #200): faccia
// originale modificabile (finche la richiesta non e chiusa; la persona e
// invariata) separata dalla faccia accolta, fatti come cronologia (parere,
// decisione, esito, iscrizione generata), azioni guidate per parere TL
// (consultativo, riscrivibile), decisione con accoglimento anti-doppione
// (riscrivibile anche a richiesta chiusa da decisione) e ritiro. Il parere
// TL non e prerequisito della decisione (#200): la decisione People e
// disponibile su ogni richiesta aperta senza decisione, con o senza parere
// e con o senza team. Nessuna state machine locale: la sequenzialita e
// l'override vivono nel backend, qui si offrono le azioni coerenti coi
// fatti gia caricati e si mostrano per intero i 4xx.

import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Modal, MultiSelect, Skeleton, SingleSelect, StatusBadge, useToast, VisuallyHidden } from '@mrsmith/ui';
import {
  useCreateCourse,
  useRecordRequestDecision,
  useRecordTLOpinion,
  useRequestDetail,
  useResumeRequest,
  useSuspendRequest,
  useTrainingCourses,
  useTrainingEvents,
  useTrainingLookups,
  useTrainingPeople,
  useTrainingSkillAreas,
  useTrainingTeams,
  useUpdateRequestAnnotations,
  useUpdateRequestOriginal,
  useWithdrawRequest,
} from '../../api/queries';
import type {
  RequestAcceptedInput,
  RequestAreaRef,
  RequestDetail,
  RequestOriginalDataInput,
  TLOpinionValue,
} from '../../api/types';
import { HistoryPanel } from '../audit/HistoryPanel';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly, formatInstantDate } from '../events/eventFormat';
import { REQUEST_OUTCOME_LABELS, TL_OPINION_LABELS } from '../../lib/labels';
import { LEVEL_OPTIONS } from '../../lib/levels';
import { outcomeVariant, tlOpinionVariant } from './requestVariants';
import formStyles from './requestShared.module.css';
import styles from './drawerShared.module.css';
import localStyles from './RequestDetailDrawer.module.css';

function periodLabel(start?: string, end?: string): string {
  if (!start && !end) return '—';
  return `${formatDateOnly(start)} – ${formatDateOnly(end)}`;
}

function areaLabel(area: RequestAreaRef): string {
  if (area.levelCurrent === undefined && area.levelTarget === undefined) return area.name;
  const current = area.levelCurrent ?? '—';
  const target = area.levelTarget ?? '—';
  return `${area.name} (${current} → ${target})`;
}

interface RequestDetailDrawerProps {
  id: string;
  onClose: () => void;
}

export function RequestDetailDrawer({ id, onClose }: RequestDetailDrawerProps) {
  const { toast } = useToast();
  const detail = useRequestDetail(id);
  const teams = useTrainingTeams();
  const withdrawRequest = useWithdrawRequest();

  const suspendRequest = useSuspendRequest();
  const resumeRequest = useResumeRequest();

  const [showTLOpinion, setShowTLOpinion] = useState(false);
  const [showDecision, setShowDecision] = useState(false);
  const [showWithdraw, setShowWithdraw] = useState(false);
  const [showSuspend, setShowSuspend] = useState(false);
  const [showAnnotations, setShowAnnotations] = useState(false);
  const [showOriginal, setShowOriginal] = useState(false);
  const [suspendReason, setSuspendReason] = useState('');
  const [suspendError, setSuspendError] = useState<string | null>(null);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  const request = detail.data;
  const isOpen = !!request && !request.outcome;
  const isSuspended = !!request?.suspendedAt;
  const needsOpinion = isOpen && !request?.tlOpinion;
  const needsDecision = isOpen && !request?.decision;
  // Riscrittura decisione su richiesta chiusa da decisione (accepted|rejected);
  // withdrawn resta terminale (#171).
  const canRewriteDecision =
    !!request && (request.outcome === 'accepted' || request.outcome === 'rejected');
  // Il parere TL si offre solo quando la richiesta ha un team con almeno un
  // lead attivo (#200). Team assente, team senza lead o anagrafica team non
  // ancora caricata: il bottone resta nascosto, senza mai bloccare la
  // decisione People, che non dipende ne dal parere ne dal team.
  const teamWithLeads = useMemo(() => {
    const teamId = request?.requested.selectedTeamId;
    if (!teamId) return null;
    return (teams.data ?? []).find((t) => t.id === teamId) ?? null;
  }, [teams.data, request]);
  const canRecordTLOpinion = (teamWithLeads?.leads.length ?? 0) > 0;

  async function handleWithdraw() {
    setWithdrawError(null);
    try {
      await withdrawRequest.mutateAsync(id);
      setShowWithdraw(false);
      toast('Richiesta ritirata');
    } catch (e) {
      setWithdrawError(describeApiError(e, 'Ritiro non riuscito'));
    }
  }

  async function handleSuspend() {
    setSuspendError(null);
    try {
      await suspendRequest.mutateAsync({ id, input: { reason: suspendReason.trim() } });
      setShowSuspend(false);
      setSuspendReason('');
      toast('Richiesta sospesa');
    } catch (e) {
      setSuspendError(describeApiError(e, 'Sospensione non riuscita'));
    }
  }

  async function handleResume() {
    try {
      await resumeRequest.mutateAsync(id);
      toast('Richiesta riattivata');
    } catch (e) {
      toast(describeApiError(e, 'Riattivazione non riuscita'));
    }
  }

  return (
    <>
      <Drawer
        open
        onClose={onClose}
        title={request?.requested.employeeName ?? 'Richiesta'}
        subtitle={request ? request.requested.courseTitle : undefined}
        size="lg"
        footer={
          isOpen ? (
            <div className={styles.footerActions}>
              <Button variant="ghost" size="md" onClick={() => setShowWithdraw(true)}>
                Ritira richiesta
              </Button>
              {isSuspended ? (
                <Button variant="secondary" size="md" loading={resumeRequest.isPending} onClick={handleResume}>
                  Riattiva
                </Button>
              ) : (
                <Button variant="ghost" size="md" onClick={() => setShowSuspend(true)}>
                  Sospendi
                </Button>
              )}
              {canRecordTLOpinion && (
                <Button variant="secondary" size="md" onClick={() => setShowTLOpinion(true)}>
                  {needsOpinion ? 'Registra parere TL' : 'Riscrivi parere TL'}
                </Button>
              )}
              {needsDecision && (
                <Button variant="primary" size="md" onClick={() => setShowDecision(true)}>
                  Registra decisione
                </Button>
              )}
            </div>
          ) : canRewriteDecision ? (
            <div className={styles.footerActions}>
              <Button variant="primary" size="md" onClick={() => setShowDecision(true)}>
                Riscrivi decisione
              </Button>
            </div>
          ) : undefined
        }
      >
        <div className={styles.body}>
          {detail.isLoading && <Skeleton rows={6} />}
          {detail.isError && (
            <p className={styles.errorNotice}>
              {describeApiError(detail.error, 'Lettura della richiesta non riuscita')}
            </p>
          )}
          {request && (
            <>
              <section className={styles.card}>
                <h3 className={styles.cardTitle}>
                  Richiesta originale
                  {isOpen && (
                    <Button variant="ghost" size="sm" onClick={() => setShowOriginal(true)}>
                      Modifica
                    </Button>
                  )}
                </h3>
                <dl className={styles.grid}>
                  <div className={styles.item}>
                    <dt>Persona</dt>
                    <dd>{request.requested.employeeName}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Team</dt>
                    <dd>
                      {request.requested.selectedTeamName ?? (
                        <span className={styles.muted}>Senza team</span>
                      )}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Corso o titolo</dt>
                    <dd>{request.requested.courseTitle}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Aree di competenza (attuale → atteso)</dt>
                    <dd>{request.requested.skillAreas.length > 0 ? request.requested.skillAreas.map(areaLabel).join(', ') : '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Periodo desiderato</dt>
                    <dd>{periodLabel(request.requested.desiredStart, request.requested.desiredEnd)}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Motivazione</dt>
                    <dd>{request.requested.motivation}</dd>
                  </div>
                </dl>
              </section>

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>
                  Pianificazione
                  {isOpen && (
                    <Button variant="ghost" size="sm" onClick={() => setShowAnnotations(true)}>
                      Modifica
                    </Button>
                  )}
                </h3>
                <dl className={styles.grid}>
                  <div className={styles.item}>
                    <dt>Priorità</dt>
                    <dd>{request.priority ?? '—'}</dd>
                  </div>
                  <div className={`${styles.item} ${styles.full}`}>
                    <dt>Promemoria</dt>
                    <dd className={styles.preWrap}>
                      {request.reminderText
                        ? `${request.reminderText}${request.reminderAt ? ` · richiamo ${formatDateOnly(request.reminderAt)}` : ''}`
                        : '—'}
                    </dd>
                  </div>
                  <div className={`${styles.item} ${styles.full}`}>
                    <dt>Nota</dt>
                    <dd className={styles.preWrap}>{request.notes || '—'}</dd>
                  </div>
                  {isSuspended && (
                    <div className={`${styles.item} ${styles.full}`}>
                      <dt>Sospesa</dt>
                      <dd>
                        <StatusBadge value="suspended" label="Sospesa" variant="warning" />
                        {` ${formatInstantDate(request.suspendedAt)}`}
                        {request.suspendedByName ? ` · ${request.suspendedByName}` : ''}
                        {request.suspensionReason ? ` · ${request.suspensionReason}` : ''}
                      </dd>
                    </div>
                  )}
                </dl>
              </section>

              {request.accepted && (
                <section className={styles.card}>
                  <h3 className={styles.cardTitle}>Formazione accolta</h3>
                  <dl className={styles.grid}>
                    <div className={styles.item}>
                      <dt>Corso effettivo</dt>
                      <dd>{request.accepted.courseTitle}</dd>
                    </div>
                    <div className={styles.item}>
                      <dt>Evento</dt>
                      <dd>
                        {request.accepted.eventId ? (
                          <Link to={`/eventi/${request.accepted.eventId}?highlight=enrollments`}>Apri evento</Link>
                        ) : (
                          '—'
                        )}
                      </dd>
                    </div>
                    <div className={styles.item}>
                      <dt>Fornitore</dt>
                      <dd>{request.accepted.vendorName || '—'}</dd>
                    </div>
                    <div className={styles.item}>
                      <dt>Periodo</dt>
                      <dd>{periodLabel(request.accepted.periodStart, request.accepted.periodEnd)}</dd>
                    </div>
                    <div className={styles.item}>
                      <dt>Note</dt>
                      <dd>{request.accepted.notes || '—'}</dd>
                    </div>
                  </dl>
                </section>
              )}

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>Cronologia</h3>
                <ul className={localStyles.timeline}>
                  <li>
                    <span className={localStyles.timelineLabel}>Parere TL</span>
                    {request.tlOpinion ? (
                      <span className={localStyles.timelineValue}>
                        <StatusBadge
                          value={request.tlOpinion.opinion}
                          label={TL_OPINION_LABELS[request.tlOpinion.opinion]}
                          variant={tlOpinionVariant(request.tlOpinion.opinion)}
                        />
                        {request.tlOpinion.byName} · {formatInstantDate(request.tlOpinion.at)}
                        {request.tlOpinion.reason && ` · ${request.tlOpinion.reason}`}
                      </span>
                    ) : (
                      <span className={localStyles.timelineMuted}>Non registrato</span>
                    )}
                  </li>
                  <li>
                    <span className={localStyles.timelineLabel}>Decisione</span>
                    {request.decision ? (
                      <span className={localStyles.timelineValue}>
                        <StatusBadge
                          value={request.decision.decision}
                          label={REQUEST_OUTCOME_LABELS[request.decision.decision] ?? request.decision.decision}
                          variant={outcomeVariant(request.decision.decision)}
                        />
                        {request.decision.byName} · {formatInstantDate(request.decision.at)} ·{' '}
                        {request.decision.reason}
                      </span>
                    ) : (
                      <span className={localStyles.timelineMuted}>Non registrata</span>
                    )}
                  </li>
                  <li>
                    <span className={localStyles.timelineLabel}>Iscrizione generata</span>
                    {request.resultingEnrollmentId ? (
                      <span className={localStyles.timelineValue}>
                        {request.accepted?.eventId ? (
                          <Link to={`/eventi/${request.accepted.eventId}?highlight=enrollments`}>Apri evento</Link>
                        ) : (
                          request.resultingEnrollmentId
                        )}
                      </span>
                    ) : (
                      <span className={localStyles.timelineMuted}>Nessuna iscrizione generata</span>
                    )}
                  </li>
                  <li>
                    <span className={localStyles.timelineLabel}>Esito finale</span>
                    {request.outcome ? (
                      <span className={localStyles.timelineValue}>
                        <StatusBadge
                          value={request.outcome}
                          label={REQUEST_OUTCOME_LABELS[request.outcome] ?? request.outcome}
                          variant={outcomeVariant(request.outcome)}
                        />
                        {formatInstantDate(request.closedAt)}
                      </span>
                    ) : (
                      <span className={localStyles.timelineMuted}>Richiesta aperta</span>
                    )}
                  </li>
                </ul>
              </section>

              <HistoryPanel selector={{ kind: 'entity', entityType: 'training_request', entityId: request.id }} />
            </>
          )}
        </div>
      </Drawer>

      {request && showTLOpinion && <TLOpinionForm request={request} onClose={() => setShowTLOpinion(false)} />}
      {request && showDecision && <DecisionForm request={request} onClose={() => setShowDecision(false)} />}
      {request && showAnnotations && <AnnotationsForm request={request} onClose={() => setShowAnnotations(false)} />}
      {request && showOriginal && <OriginalDataForm request={request} onClose={() => setShowOriginal(false)} />}
      {showSuspend && (
        <Modal open onClose={() => setShowSuspend(false)} title="Sospendi richiesta" size="sm">
          <div className={`${formStyles.body} ${formStyles.bodyModal}`}>
            <p>L'esigenza resta aperta ma esce dalle viste operative finché non viene riattivata.</p>
            <label className={formStyles.field}>
              Motivo (facoltativo)
              <textarea
                className={formStyles.textarea}
                value={suspendReason}
                onChange={(e) => setSuspendReason(e.target.value)}
                rows={2}
                placeholder="Es. fino a fine università"
              />
            </label>
            <ErrorPanel message={suspendError} onDismiss={() => setSuspendError(null)} />
            <div className={formStyles.actions}>
              <Button variant="ghost" size="md" onClick={() => setShowSuspend(false)} disabled={suspendRequest.isPending}>
                Annulla
              </Button>
              <Button variant="primary" size="md" loading={suspendRequest.isPending} onClick={handleSuspend}>
                Sospendi
              </Button>
            </div>
          </div>
        </Modal>
      )}
      {showWithdraw && (
        <Modal open onClose={() => setShowWithdraw(false)} title="Ritira richiesta" size="sm">
          <div className={formStyles.body}>
            <p>La richiesta verrà chiusa con esito «ritirata»; parere e decisione restano quelli già registrati.</p>
            <ErrorPanel message={withdrawError} onDismiss={() => setWithdrawError(null)} />
            <div className={formStyles.actions}>
              <Button
                variant="ghost"
                size="md"
                onClick={() => setShowWithdraw(false)}
                disabled={withdrawRequest.isPending}
              >
                Annulla
              </Button>
              <Button variant="danger" size="md" loading={withdrawRequest.isPending} onClick={handleWithdraw}>
                Ritira richiesta
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
}

// ── Annotazioni di pianificazione: nota, promemoria, priorità ──

function AnnotationsForm({ request, onClose }: { request: RequestDetail; onClose: () => void }) {
  const { toast } = useToast();
  const updateAnnotations = useUpdateRequestAnnotations();

  const [notes, setNotes] = useState(request.notes ?? '');
  const [reminderText, setReminderText] = useState(request.reminderText ?? '');
  const [reminderAt, setReminderAt] = useState(request.reminderAt ?? '');
  const [priority, setPriority] = useState(request.priority !== undefined ? String(request.priority) : '');
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    try {
      await updateAnnotations.mutateAsync({
        id: request.id,
        input: {
          notes: notes.trim() || undefined,
          reminderText: reminderText.trim() || undefined,
          reminderAt: reminderAt || undefined,
          priority: priority !== '' ? Number(priority) : undefined,
        },
      });
      toast('Annotazioni aggiornate');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Aggiornamento non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Annotazioni di pianificazione" size="sm">
      <div className={`${formStyles.body} ${formStyles.bodyModal}`}>
        <label className={formStyles.field}>
          Priorità (1 = più importante)
          <input
            type="number"
            min={1}
            className={formStyles.input}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
        </label>
        <label className={formStyles.field}>
          Promemoria (in attesa di / prossimo passo)
          <input
            className={formStyles.input}
            value={reminderText}
            onChange={(e) => setReminderText(e.target.value)}
          />
        </label>
        <label className={formStyles.field}>
          Data di richiamo
          <input
            type="date"
            className={formStyles.input}
            value={reminderAt}
            onChange={(e) => setReminderAt(e.target.value)}
          />
        </label>
        <label className={formStyles.field}>
          Nota
          <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={updateAnnotations.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={updateAnnotations.isPending} onClick={submit}>
            Salva
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Parere del lead (#157, §Richieste 4) ──

function TLOpinionForm({ request, onClose }: { request: RequestDetail; onClose: () => void }) {
  const { toast } = useToast();
  const teams = useTrainingTeams();
  const recordTLOpinion = useRecordTLOpinion();

  const existing = request.tlOpinion;
  const [leadEmployeeId, setLeadEmployeeId] = useState(existing?.byEmployeeId ?? '');
  const [opinion, setOpinion] = useState<TLOpinionValue>(existing?.opinion ?? 'favorable');
  const [reason, setReason] = useState(existing?.reason ?? '');
  const [error, setError] = useState<string | null>(null);

  const team = (teams.data ?? []).find((t) => t.id === request.requested.selectedTeamId);
  const leads = team?.leads ?? [];
  const canSubmit = leadEmployeeId !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      await recordTLOpinion.mutateAsync({ id: request.id, input: { leadEmployeeId, opinion, reason: reason.trim() || undefined } });
      toast(existing ? 'Parere TL aggiornato' : 'Parere TL registrato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Registrazione del parere non riuscita'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Parere del lead" size="sm">
      <div className={formStyles.body}>
        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Lead del team {request.requested.selectedTeamName ?? ''}
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={leads.map((l) => ({ value: l.employeeId, label: l.name }))}
            selected={leadEmployeeId || null}
            onChange={(v) => setLeadEmployeeId(v ?? '')}
            placeholder={leads.length === 0 ? 'Nessun lead attivo per il team' : 'Seleziona lead...'}
          />
        </label>
        <div className={formStyles.toggleGroup}>
          <Button
            variant={opinion === 'favorable' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setOpinion('favorable')}
          >
            Favorevole
          </Button>
          <Button
            variant={opinion === 'unfavorable' ? 'danger' : 'secondary'}
            size="sm"
            onClick={() => setOpinion('unfavorable')}
          >
            Sfavorevole
          </Button>
        </div>
        <label className={formStyles.field}>
          Motivazione (facoltativa)
          <textarea className={formStyles.textarea} value={reason} onChange={(e) => setReason(e.target.value)} rows={3} />
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={recordTLOpinion.isPending}>
            Annulla
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={recordTLOpinion.isPending}
            disabled={!canSubmit}
            onClick={submit}
          >
            {existing ? 'Riscrivi parere' : 'Registra parere'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Decisione People (#157, §Richieste 5): respinta con motivazione,
// accolta con pannello guidato — corso a catalogo o fuori catalogo,
// evento esistente o nuovo, copertura esistente per evitare doppioni. Un
// parere sfavorevole non blocca l'accoglimento: resta visibile come
// promemoria, l'override si motiva nella decisione. ──

function DecisionForm({ request, onClose }: { request: RequestDetail; onClose: () => void }) {
  const { toast } = useToast();
  const courses = useTrainingCourses();
  const events = useTrainingEvents();
  const lookups = useTrainingLookups();
  const recordDecision = useRecordRequestDecision();
  const createCourse = useCreateCourse();

  const existing = request.decision;
  const accepted = request.accepted;
  // Riscrittura di un accoglimento (#175): parte precompilata dall'accoglimento
  // precedente (corso, evento, fornitore, periodo, note) e con l'iscrizione
  // risultante agganciata. Prima decisione e riscrittura da respinto: vuoto.
  const prefill = existing?.decision === 'accepted' && !!accepted;
  const [decisionKind, setDecisionKind] = useState<'accepted' | 'rejected'>(existing?.decision ?? 'accepted');
  const [reason, setReason] = useState(existing?.reason ?? '');
  const [existingEnrollmentId, setExistingEnrollmentId] = useState(prefill ? request.resultingEnrollmentId ?? '' : '');
  const [courseMode, setCourseMode] = useState<'catalog' | 'new'>('catalog');
  const [courseId, setCourseId] = useState(prefill ? accepted?.courseId ?? '' : '');
  const [newCourseTitle, setNewCourseTitle] = useState('');
  const [newCourseProviderKind, setNewCourseProviderKind] = useState<'internal' | 'external'>('external');
  const [newCourseVendorId, setNewCourseVendorId] = useState('');
  const [eventMode, setEventMode] = useState<'existing' | 'new'>(prefill && !!accepted?.eventId ? 'existing' : 'new');
  const [eventId, setEventId] = useState(prefill ? accepted?.eventId ?? '' : '');
  const [vendorId, setVendorId] = useState(prefill ? accepted?.vendorId ?? '' : '');
  const [periodStart, setPeriodStart] = useState(prefill ? accepted?.periodStart ?? '' : '');
  const [periodEnd, setPeriodEnd] = useState(prefill ? accepted?.periodEnd ?? '' : '');
  const [notes, setNotes] = useState(prefill ? accepted?.notes ?? '' : '');
  const [error, setError] = useState<string | null>(null);

  const coverage = request.existingCoverage;
  const hasCoverage = coverage.completedEnrollments.length > 0 || coverage.validAwards.length > 0;
  const linkingCoverage = existingEnrollmentId !== '';
  const pending = recordDecision.isPending || createCourse.isPending;
  const eventsForCourse = (events.data ?? []).filter((e) => e.courseId === courseId && !e.flags.cancelled);
  // L'iscrizione risultante dell'accoglimento precedente è tipicamente «planned» e
  // non compare tra le iscrizioni completate della copertura: la proponiamo come
  // opzione dedicata, così la riconferma dello stesso accoglimento non produce
  // il 409 già_iscritta (il backend la accetta: valida persona/corso, non lo stato).
  const resultingEnrollment = request.resultingEnrollmentId
    ? {
        value: request.resultingEnrollmentId,
        label: 'Iscrizione già collegata a questa richiesta',
      }
    : null;
  const enrollmentOptions = [
    ...coverage.completedEnrollments.map((en) => ({
      value: en.enrollmentId,
      label: `Completata il ${formatDateOnly(en.completedOn)}`,
    })),
    ...(resultingEnrollment && !coverage.completedEnrollments.some((en) => en.enrollmentId === resultingEnrollment.value)
      ? [resultingEnrollment]
      : []),
  ];
  // Il pannello di aggancio deve comparire anche quando c'è solo l'iscrizione
  // risultante da riagganciare: altrimenti l'aggancio preselezionato nasconderebbe
  // la sezione corso/evento senza un'UI per sganciarlo.
  const showCoverageLink = hasCoverage || !!request.resultingEnrollmentId;
  // Corso prefillato anche se non più attivo a catalogo: lo aggiungiamo alle
  // opzioni con la label dell'accoglimento, così il prefill resta leggibile.
  const catalogCourseOptions = (courses.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.title }));
  if (prefill && accepted && accepted.courseId && !catalogCourseOptions.some((c) => c.value === accepted.courseId)) {
    catalogCourseOptions.unshift({ value: accepted.courseId, label: accepted.courseTitle });
  }

  // I due reset di evento non devono clobberare il prefill al primo render:
  // confrontiamo i valori precedenti (robusto anche con StrictMode) invece di
  // una flag one-shot, così un cambio di corso da parte dell'utente resetta
  // ancora l'evento come prima.
  const prevCourse = useRef({ courseId, courseMode });
  const prevEvent = useRef({ eventMode, eventsCount: eventsForCourse.length });

  useEffect(() => {
    const prev = prevCourse.current;
    prevCourse.current = { courseId, courseMode };
    if (prev.courseId === courseId && prev.courseMode === courseMode) return;
    setEventMode('new');
    setEventId('');
  }, [courseId, courseMode]);

  useEffect(() => {
    const prev = prevEvent.current;
    prevEvent.current = { eventMode, eventsCount: eventsForCourse.length };
    if (prev.eventMode === eventMode && prev.eventsCount === eventsForCourse.length) return;
    if (eventMode === 'existing' && eventsForCourse.length === 0) {
      setEventMode('new');
      setEventId('');
    }
  }, [eventsForCourse.length, eventMode]);

  const canSubmit =
    decisionKind === 'rejected' ||
    linkingCoverage ||
    ((courseMode === 'catalog'
      ? courseId !== ''
      : newCourseTitle.trim() !== '' && (newCourseProviderKind === 'internal' || newCourseVendorId !== '')) &&
      (eventMode !== 'existing' || eventId !== ''));

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      if (decisionKind === 'rejected') {
        await recordDecision.mutateAsync({ id: request.id, input: { decision: 'rejected', reason: reason.trim() || undefined } });
        toast(existing ? 'Decisione aggiornata' : 'Richiesta respinta');
        onClose();
        return;
      }
      let resolvedCourseId = courseId;
      if (linkingCoverage) {
        resolvedCourseId = coverage.courseId ?? '';
      } else if (courseMode === 'new') {
        const created = await createCourse.mutateAsync({
          title: newCourseTitle.trim(),
          providerKind: newCourseProviderKind,
          vendorId: newCourseProviderKind === 'external' ? newCourseVendorId || undefined : undefined,
        });
        resolvedCourseId = created.id ?? '';
      }
      const accepted: RequestAcceptedInput = {
        courseId: resolvedCourseId,
        eventId: !linkingCoverage && eventMode === 'existing' ? eventId || undefined : undefined,
        vendorId: vendorId || undefined,
        periodStart: periodStart || undefined,
        periodEnd: periodEnd || undefined,
        notes: notes.trim() || undefined,
        existingEnrollmentId: existingEnrollmentId || undefined,
      };
      await recordDecision.mutateAsync({
        id: request.id,
        input: { decision: 'accepted', reason: reason.trim() || undefined, accepted },
      });
      toast(existing ? 'Decisione aggiornata' : 'Richiesta accolta');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Registrazione della decisione non riuscita'));
    }
  }

  return (
    <Modal open onClose={onClose} title={existing ? 'Riscrivi decisione' : 'Decisione People'} size="wide">
      <div className={formStyles.body}>
        <div className={formStyles.toggleGroup}>
          <Button
            variant={decisionKind === 'accepted' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setDecisionKind('accepted')}
          >
            Accogli
          </Button>
          <Button
            variant={decisionKind === 'rejected' ? 'danger' : 'secondary'}
            size="sm"
            onClick={() => setDecisionKind('rejected')}
          >
            Respingi
          </Button>
        </div>

        {request.tlOpinion && (
          <p className={formStyles.hint}>
            Parere TL: {TL_OPINION_LABELS[request.tlOpinion.opinion]}
            {request.tlOpinion.reason ? ` — ${request.tlOpinion.reason}` : ''}
            {request.tlOpinion.opinion === 'unfavorable' && decisionKind === 'accepted'
              ? ' · accogliere scavalca il parere: la motivazione qui sotto vale anche come motivazione dell\'override.'
              : ''}
          </p>
        )}

        <label className={formStyles.field}>
          Motivazione (facoltativa)
          <textarea className={formStyles.textarea} value={reason} onChange={(e) => setReason(e.target.value)} rows={2} />
        </label>

        {decisionKind === 'accepted' && (
          <>
            {showCoverageLink && (
              <section className={localStyles.coveragePanel}>
                <h4 className={localStyles.coverageTitle}>
                  Copertura esistente{coverage.courseTitle ? ` per ${coverage.courseTitle}` : ''}
                </h4>
                {(coverage.completedEnrollments.length > 0 || !!request.resultingEnrollmentId) && (
                  <label className={formStyles.field}>
                    Collega iscrizione esistente (evita doppioni)
                    <SingleSelect
                      options={enrollmentOptions}
                      selected={existingEnrollmentId || null}
                      onChange={(v) => setExistingEnrollmentId(v ?? '')}
                      placeholder="Nessuna: nuova assegnazione"
                      allowClear
                    />
                  </label>
                )}
                {coverage.validAwards.map((award) => (
                  <p key={`${award.certificationName}-${award.awardedOn}`} className={formStyles.hint}>
                    Certificazione valida: {award.certificationName} · conseguita {formatDateOnly(award.awardedOn)}
                    {award.expiresOn && ` · scade ${formatDateOnly(award.expiresOn)}`}
                  </p>
                ))}
              </section>
            )}

            {!linkingCoverage && (
              <>
                <div className={formStyles.toggleGroup}>
                  <Button
                    variant={courseMode === 'catalog' ? 'primary' : 'secondary'}
                    size="sm"
                    onClick={() => setCourseMode('catalog')}
                  >
                    Corso a catalogo
                  </Button>
                  <Button
                    variant={courseMode === 'new' ? 'primary' : 'secondary'}
                    size="sm"
                    onClick={() => setCourseMode('new')}
                  >
                    Corso fuori catalogo
                  </Button>
                </div>
                {courseMode === 'catalog' ? (
                  <label className={formStyles.field}>
                    <span className={formStyles.labelHead}>
                      Corso effettivo
                      <span className={formStyles.requiredMarker} aria-hidden="true" />
                      <VisuallyHidden>obbligatorio</VisuallyHidden>
                    </span>
                    <SingleSelect
                      options={catalogCourseOptions}
                      selected={courseId || null}
                      onChange={(v) => setCourseId(v ?? '')}
                      placeholder="Seleziona corso..."
                      searchable
                    />
                  </label>
                ) : (
                  <>
                    <label className={formStyles.field}>
                      <span className={formStyles.labelHead}>
                        Titolo del corso
                        <span className={formStyles.requiredMarker} aria-hidden="true" />
                        <VisuallyHidden>obbligatorio</VisuallyHidden>
                      </span>
                      <input
                        className={formStyles.input}
                        value={newCourseTitle}
                        onChange={(e) => setNewCourseTitle(e.target.value)}
                      />
                    </label>
                    <div className={formStyles.row}>
                      <label className={formStyles.field}>
                        Erogazione
                        <SingleSelect
                          options={[
                            { value: 'external', label: 'Esterna' },
                            { value: 'internal', label: 'Interna' },
                          ]}
                          selected={newCourseProviderKind}
                          onChange={(v) => setNewCourseProviderKind((v as 'internal' | 'external') ?? 'external')}
                        />
                      </label>
                      {newCourseProviderKind === 'external' && (
                        <label className={formStyles.field}>
                          <span className={formStyles.labelHead}>
                            Fornitore
                            <span className={formStyles.requiredMarker} aria-hidden="true" />
                            <VisuallyHidden>obbligatorio</VisuallyHidden>
                          </span>
                          <SingleSelect
                            options={(lookups.data?.vendors ?? [])
                              .filter((v) => v.active)
                              .map((v) => ({ value: v.id, label: v.label }))}
                            selected={newCourseVendorId || null}
                            onChange={(v) => setNewCourseVendorId(v ?? '')}
                            placeholder="Seleziona fornitore..."
                          />
                        </label>
                      )}
                    </div>
                  </>
                )}

                <div className={formStyles.toggleGroup}>
                  <Button variant={eventMode === 'new' ? 'primary' : 'secondary'} size="sm" onClick={() => setEventMode('new')}>
                    Nuovo evento
                  </Button>
                  <Button
                    variant={eventMode === 'existing' ? 'primary' : 'secondary'}
                    size="sm"
                    disabled={courseMode !== 'catalog' || courseId === ''}
                    onClick={() => setEventMode('existing')}
                  >
                    Evento esistente
                  </Button>
                </div>
                {eventMode === 'existing' ? (
                  <label className={formStyles.field}>
                    Evento
                    <SingleSelect
                      options={eventsForCourse.map((e) => ({ value: e.id, label: formatInstantDate(e.createdAt) }))}
                      selected={eventId || null}
                      onChange={(v) => setEventId(v ?? '')}
                      placeholder={eventsForCourse.length === 0 ? 'Nessun evento esistente per il corso' : 'Seleziona evento...'}
                    />
                  </label>
                ) : (
                  <p className={formStyles.hint}>Verrà creato un nuovo evento per questa tornata.</p>
                )}
              </>
            )}

            <label className={formStyles.field}>
              Fornitore
              <SingleSelect
                options={(lookups.data?.vendors ?? []).filter((v) => v.active).map((v) => ({ value: v.id, label: v.label }))}
                selected={vendorId || null}
                onChange={(v) => setVendorId(v ?? '')}
                placeholder="Nessun fornitore"
                allowClear
              />
            </label>
            <div className={formStyles.row}>
              <label className={formStyles.field}>
                Periodo — inizio
                <input
                  type="date"
                  className={formStyles.input}
                  value={periodStart}
                  onChange={(e) => setPeriodStart(e.target.value)}
                />
              </label>
              <label className={formStyles.field}>
                Periodo — fine
                <input
                  type="date"
                  className={formStyles.input}
                  value={periodEnd}
                  onChange={(e) => setPeriodEnd(e.target.value)}
                />
              </label>
            </div>
            <label className={formStyles.field}>
              Note
              <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
            </label>
          </>
        )}

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button
            variant={decisionKind === 'accepted' ? 'primary' : 'danger'}
            size="md"
            loading={pending}
            disabled={!canSubmit}
            onClick={submit}
          >
            {decisionKind === 'accepted'
              ? existing
                ? 'Riscrivi accoglimento'
                : 'Accogli richiesta'
              : existing
                ? 'Riscrivi in respinto'
                : 'Respingi richiesta'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Modifica dei dati originali (#171): corso o titolo nuovo, aree con
// livelli, motivazione, team (persona invariata) e date desiderate. Reusa i
// pezzi della creazione: select del team tra le appartenenze attive della
// persona, editor aree/livelli. Ammessa solo a richiesta aperta. ──

function OriginalDataForm({ request, onClose }: { request: RequestDetail; onClose: () => void }) {
  const { toast } = useToast();
  const people = useTrainingPeople();
  const lookups = useTrainingLookups();
  const skillAreas = useTrainingSkillAreas();
  const updateOriginal = useUpdateRequestOriginal();

  const requested = request.requested;
  const person = (people.data ?? []).find((p) => p.id === requested.employeeId);
  const activeTeams = person?.teams ?? [];

  const [courseMode, setCourseMode] = useState<'catalog' | 'new'>(requested.courseId ? 'catalog' : 'new');
  const [courseId, setCourseId] = useState(requested.courseId ?? '');
  const [newCourseTitle, setNewCourseTitle] = useState(requested.courseTitle ?? '');
  const [skillAreaIds, setSkillAreaIds] = useState<string[]>(requested.skillAreas.map((a) => a.id));
  const [areaLevels, setAreaLevels] = useState<Record<string, { current: string; target: string }>>(
    Object.fromEntries(
      requested.skillAreas.map((a) => [
        a.id,
        { current: a.levelCurrent !== undefined ? String(a.levelCurrent) : '', target: a.levelTarget !== undefined ? String(a.levelTarget) : '' },
      ]),
    ),
  );
  const [motivation, setMotivation] = useState(requested.motivation);
  const [teamId, setTeamId] = useState(requested.selectedTeamId ?? '');
  const [desiredStart, setDesiredStart] = useState(requested.desiredStart ?? '');
  const [desiredEnd, setDesiredEnd] = useState(requested.desiredEnd ?? '');
  const [error, setError] = useState<string | null>(null);

  // Team obbligatorio solo quando la persona ha appartenenze attive (#200);
  // prima del caricamento delle persone si resta sul caso conservativo.
  const teamOptional = people.isSuccess && activeTeams.length === 0;

  const canSubmit =
    motivation.trim() !== '' &&
    (teamOptional || teamId !== '') &&
    (courseMode === 'catalog' ? courseId !== '' : newCourseTitle.trim() !== '');

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      const input: RequestOriginalDataInput = {
        courseId: courseMode === 'catalog' ? courseId : undefined,
        newCourseTitle: courseMode === 'new' ? newCourseTitle.trim() : undefined,
        skillAreas: skillAreaIds.map((areaId) => ({
          id: areaId,
          levelCurrent:
            areaLevels[areaId]?.current !== undefined && areaLevels[areaId]?.current !== ''
              ? Number(areaLevels[areaId]?.current)
              : undefined,
          levelTarget:
            areaLevels[areaId]?.target !== undefined && areaLevels[areaId]?.target !== ''
              ? Number(areaLevels[areaId]?.target)
              : undefined,
        })),
        motivation: motivation.trim(),
        selectedTeamId: teamId || undefined,
        desiredStart: desiredStart || undefined,
        desiredEnd: desiredEnd || undefined,
      };
      await updateOriginal.mutateAsync({ id: request.id, input });
      toast('Dati originali aggiornati');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Aggiornamento dei dati originali non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Modifica richiesta originale" size="md">
      <div className={`${formStyles.body} ${formStyles.bodyModal}`}>
        <label className={formStyles.field}>
          Persona
          <input className={formStyles.input} value={requested.employeeName} disabled readOnly />
          <span className={formStyles.hint}>
            La persona non si cambia: per correggere l'identità ritira la richiesta e registrane una nuova.
          </span>
        </label>
        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Team
            {!teamOptional && (
              <>
                <span className={formStyles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </>
            )}
          </span>
          <SingleSelect
            options={activeTeams.map((t) => ({ value: t.id, label: t.name }))}
            selected={teamId || null}
            onChange={(v) => setTeamId(v ?? '')}
            placeholder={teamOptional ? 'Senza team' : 'Seleziona team...'}
            disabled={activeTeams.length === 0}
          />
          {teamOptional && (
            <span className={formStyles.hint}>
              La persona non ha appartenenze attive: la richiesta si salva senza team.
            </span>
          )}
        </label>

        <div className={formStyles.toggleGroup}>
          <Button
            variant={courseMode === 'catalog' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setCourseMode('catalog')}
          >
            Corso a catalogo
          </Button>
          <Button
            variant={courseMode === 'new' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setCourseMode('new')}
          >
            Nuovo corso
          </Button>
        </div>
        {courseMode === 'catalog' ? (
          <label className={formStyles.field}>
            Corso
            <SingleSelect
              options={(lookups.data?.courses ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.label }))}
              selected={courseId || null}
              onChange={(v) => setCourseId(v ?? '')}
              placeholder="Seleziona corso..."
              searchable
            />
          </label>
        ) : (
          <label className={formStyles.field}>
            Titolo
            <input
              className={formStyles.input}
              value={newCourseTitle}
              onChange={(e) => setNewCourseTitle(e.target.value)}
              placeholder="Titolo della formazione desiderata"
            />
            <span className={formStyles.hint}>
              Il titolo entra a catalogo come corso da completare; se esiste già un corso con lo stesso nome, la richiesta si aggancia a quello.
            </span>
          </label>
        )}

        <label className={formStyles.field}>
          Aree di competenza
          <MultiSelect<string>
            options={(skillAreas.data ?? []).map((a) => ({ value: a.id, label: a.name }))}
            selected={skillAreaIds}
            onChange={setSkillAreaIds}
            placeholder="Nessuna area"
          />
        </label>
        {skillAreaIds.map((areaId) => {
          const area = (skillAreas.data ?? []).find((a) => a.id === areaId);
          const areaName = area?.name ?? 'Area';
          const levels = areaLevels[areaId] ?? { current: '', target: '' };
          return (
            <div className={formStyles.field} key={areaId}>
              <span className={formStyles.labelHead}>{areaName}</span>
              <div className={formStyles.row}>
                <label className={formStyles.field}>
                  <span className={formStyles.labelHead}>Attuale</span>
                  <SingleSelect<number>
                    options={LEVEL_OPTIONS}
                    selected={levels.current !== '' ? Number(levels.current) : null}
                    onChange={(v) => setAreaLevels({ ...areaLevels, [areaId]: { ...levels, current: v !== null ? String(v) : '' } })}
                    placeholder="Non indicato"
                    allowClear
                    clearLabel="Non indicato"
                    ariaLabel={`${areaName} — livello attuale`}
                  />
                </label>
                <label className={formStyles.field}>
                  <span className={formStyles.labelHead}>Atteso</span>
                  <SingleSelect<number>
                    options={LEVEL_OPTIONS}
                    selected={levels.target !== '' ? Number(levels.target) : null}
                    onChange={(v) => setAreaLevels({ ...areaLevels, [areaId]: { ...levels, target: v !== null ? String(v) : '' } })}
                    placeholder="Non indicato"
                    allowClear
                    clearLabel="Non indicato"
                    ariaLabel={`${areaName} — livello atteso`}
                  />
                </label>
              </div>
            </div>
          );
        })}

        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Motivazione
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <textarea
            className={formStyles.textarea}
            value={motivation}
            onChange={(e) => setMotivation(e.target.value)}
            rows={3}
          />
        </label>

        <div className={formStyles.row}>
          <label className={formStyles.field}>
            Inizio desiderato
            <input
              type="date"
              className={formStyles.input}
              value={desiredStart}
              onChange={(e) => setDesiredStart(e.target.value)}
            />
          </label>
          <label className={formStyles.field}>
            Fine desiderata
            <input
              type="date"
              className={formStyles.input}
              value={desiredEnd}
              onChange={(e) => setDesiredEnd(e.target.value)}
            />
          </label>
        </div>

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={updateOriginal.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={updateOriginal.isPending} disabled={!canSubmit} onClick={submit}>
            Salva
          </Button>
        </div>
      </div>
    </Modal>
  );
}
