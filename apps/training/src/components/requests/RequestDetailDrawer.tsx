// Dettaglio richiesta (#157, §Richieste 2/4/5/6): faccia originale immutabile
// separata dalla faccia accolta, fatti come cronologia (parere, decisione,
// esito, iscrizione generata), azioni guidate per parere TL, decisione con
// accoglimento anti-doppione e ritiro. Nessuna state machine locale: la
// sequenzialita e l'override vivono nel backend, qui si offrono le azioni
// coerenti coi fatti gia caricati e si mostrano per intero i 4xx.

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Modal, Skeleton, SingleSelect, StatusBadge, useToast, VisuallyHidden } from '@mrsmith/ui';
import {
  useCreateCourse,
  useRecordRequestDecision,
  useRecordTLOpinion,
  useRequestDetail,
  useTrainingCourses,
  useTrainingEvents,
  useTrainingLookups,
  useTrainingTeams,
  useWithdrawRequest,
} from '../../api/queries';
import type { RequestAcceptedInput, RequestDetail, TLOpinionValue } from '../../api/types';
import { HistoryPanel } from '../audit/HistoryPanel';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly, formatInstantDate } from '../events/eventFormat';
import { REQUEST_OUTCOME_LABELS, TL_OPINION_LABELS } from '../../lib/labels';
import { outcomeVariant, tlOpinionVariant } from './requestVariants';
import formStyles from './requestShared.module.css';
import styles from './drawerShared.module.css';
import localStyles from './RequestDetailDrawer.module.css';

function periodLabel(start?: string, end?: string): string {
  if (!start && !end) return '—';
  return `${formatDateOnly(start)} – ${formatDateOnly(end)}`;
}

interface RequestDetailDrawerProps {
  id: string;
  onClose: () => void;
}

export function RequestDetailDrawer({ id, onClose }: RequestDetailDrawerProps) {
  const { toast } = useToast();
  const detail = useRequestDetail(id);
  const withdrawRequest = useWithdrawRequest();

  const [showTLOpinion, setShowTLOpinion] = useState(false);
  const [showDecision, setShowDecision] = useState(false);
  const [showWithdraw, setShowWithdraw] = useState(false);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  const request = detail.data;
  const isOpen = !!request && !request.outcome;
  const needsOpinion = isOpen && !request?.tlOpinion;
  const needsDecision = isOpen && !!request?.tlOpinion && !request?.decision;

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
              {needsOpinion && (
                <Button variant="secondary" size="md" onClick={() => setShowTLOpinion(true)}>
                  Registra parere TL
                </Button>
              )}
              {needsDecision && (
                <Button variant="primary" size="md" onClick={() => setShowDecision(true)}>
                  Registra decisione
                </Button>
              )}
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
                <h3 className={styles.cardTitle}>Richiesta originale</h3>
                <dl className={styles.grid}>
                  <div className={styles.item}>
                    <dt>Persona</dt>
                    <dd>{request.requested.employeeName}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Team</dt>
                    <dd>{request.requested.selectedTeamName}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Corso o titolo</dt>
                    <dd>{request.requested.courseTitle}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Aree di competenza</dt>
                    <dd>{request.requested.skillAreas.length > 0 ? request.requested.skillAreas.map((a) => a.name).join(', ') : '—'}</dd>
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
                      <span className={localStyles.timelineMuted}>Non ancora registrato</span>
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
                      <span className={localStyles.timelineMuted}>Non ancora registrata</span>
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

// ── Parere del lead (#157, §Richieste 4) ──

function TLOpinionForm({ request, onClose }: { request: RequestDetail; onClose: () => void }) {
  const { toast } = useToast();
  const teams = useTrainingTeams();
  const recordTLOpinion = useRecordTLOpinion();

  const [leadEmployeeId, setLeadEmployeeId] = useState('');
  const [opinion, setOpinion] = useState<TLOpinionValue>('favorable');
  const [reason, setReason] = useState('');
  const [error, setError] = useState<string | null>(null);

  const team = (teams.data ?? []).find((t) => t.id === request.requested.selectedTeamId);
  const leads = team?.leads ?? [];
  const canSubmit = leadEmployeeId !== '' && reason.trim() !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      await recordTLOpinion.mutateAsync({ id: request.id, input: { leadEmployeeId, opinion, reason: reason.trim() } });
      toast('Parere TL registrato');
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
            Lead del team {request.requested.selectedTeamName}
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
          <span className={formStyles.labelHead}>
            Motivazione
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
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
            Registra parere
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

  const [decisionKind, setDecisionKind] = useState<'accepted' | 'rejected'>('accepted');
  const [reason, setReason] = useState('');
  const [existingEnrollmentId, setExistingEnrollmentId] = useState('');
  const [courseMode, setCourseMode] = useState<'catalog' | 'new'>('catalog');
  const [courseId, setCourseId] = useState('');
  const [newCourseTitle, setNewCourseTitle] = useState('');
  const [newCourseProviderKind, setNewCourseProviderKind] = useState<'internal' | 'external'>('external');
  const [newCourseVendorId, setNewCourseVendorId] = useState('');
  const [eventMode, setEventMode] = useState<'existing' | 'new'>('new');
  const [eventId, setEventId] = useState('');
  const [vendorId, setVendorId] = useState('');
  const [periodStart, setPeriodStart] = useState('');
  const [periodEnd, setPeriodEnd] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState<string | null>(null);

  const coverage = request.existingCoverage;
  const hasCoverage = coverage.completedEnrollments.length > 0 || coverage.validAwards.length > 0;
  const linkingCoverage = existingEnrollmentId !== '';
  const pending = recordDecision.isPending || createCourse.isPending;
  const eventsForCourse = (events.data ?? []).filter((e) => e.courseId === courseId && !e.flags.cancelled);

  useEffect(() => {
    setEventMode('new');
    setEventId('');
  }, [courseId, courseMode]);

  useEffect(() => {
    if (eventMode === 'existing' && eventsForCourse.length === 0) {
      setEventMode('new');
      setEventId('');
    }
  }, [eventsForCourse.length, eventMode]);

  const canSubmit =
    reason.trim() !== '' &&
    (decisionKind === 'rejected' ||
      linkingCoverage ||
      ((courseMode === 'catalog'
        ? courseId !== ''
        : newCourseTitle.trim() !== '' && (newCourseProviderKind === 'internal' || newCourseVendorId !== '')) &&
        (eventMode !== 'existing' || eventId !== '')));

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      if (decisionKind === 'rejected') {
        await recordDecision.mutateAsync({ id: request.id, input: { decision: 'rejected', reason: reason.trim() } });
        toast('Richiesta respinta');
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
        input: { decision: 'accepted', reason: reason.trim(), accepted },
      });
      toast('Richiesta accolta');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Registrazione della decisione non riuscita'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Decisione People" size="wide">
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
          <span className={formStyles.labelHead}>
            Motivazione
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <textarea className={formStyles.textarea} value={reason} onChange={(e) => setReason(e.target.value)} rows={2} />
        </label>

        {decisionKind === 'accepted' && (
          <>
            {hasCoverage && (
              <section className={localStyles.coveragePanel}>
                <h4 className={localStyles.coverageTitle}>
                  Copertura esistente{coverage.courseTitle ? ` per ${coverage.courseTitle}` : ''}
                </h4>
                {coverage.completedEnrollments.length > 0 && (
                  <label className={formStyles.field}>
                    Collega iscrizione esistente (evita doppioni)
                    <SingleSelect
                      options={coverage.completedEnrollments.map((en) => ({
                        value: en.enrollmentId,
                        label: `Completata il ${formatDateOnly(en.completedOn)}`,
                      }))}
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
                      options={(courses.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.title }))}
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
            {decisionKind === 'accepted' ? 'Accogli richiesta' : 'Respingi richiesta'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
