import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatCurrency, formatInstant, formatLocalDate } from '@mrsmith/format';
import { Button, Drawer, Modal, Skeleton, StatusBadge, useToast } from '@mrsmith/ui';
import {
  useDirectorySyncRuns,
  useRunDirectorySync,
  type DirectorySyncRun,
} from '../api/directorySync';
import {
  useFactorialEmployees,
  useFactorialSessionParticipants,
  useFactorialStatus,
  useFactorialTeams,
  useFactorialTrainingMemberships,
  useFactorialTrainingStructure,
  useFactorialTrainings,
  type FactorialEmployee,
  type FactorialMembership,
  type FactorialSession,
  type FactorialTeam,
  type FactorialTraining,
} from '../api/factorialDiag';
import styles from './FactorialPage.module.css';

type View = 'persone' | 'team' | 'formazione' | 'sincronizzazione';

const VIEWS: { key: View; label: string }[] = [
  { key: 'persone', label: 'Persone' },
  { key: 'team', label: 'Team' },
  { key: 'formazione', label: 'Formazione' },
  { key: 'sincronizzazione', label: 'Sincronizzazione' },
];

const ACTION_LABELS: Record<string, string> = {
  adopt_person: 'Persone da agganciare',
  create_person: 'Persone da creare',
  update_person: 'Persone da aggiornare',
  terminate_person: 'Persone da cessare',
  unlink_person: 'Cessate da sganciare',
  adopt_team: 'Team da agganciare',
  create_team: 'Team da creare',
  rename_team: 'Team da rinominare',
  open_membership: 'Appartenenze da aprire',
  close_membership: 'Appartenenze da chiudere',
  set_lead: 'Lead da assegnare',
  unset_lead: 'Lead da rimuovere',
};

const ACTION_ORDER = Object.keys(ACTION_LABELS);

const ATTENDANCE_STATUS_ORDER = ['pending', 'missing', 'inprogress', 'completed'];

const SCHEDULE_LABELS: Record<string, string> = {
  scheduled: 'Programmato',
  selfpaced: 'Autonomo',
};

const MODALITY_LABELS: Record<string, string> = {
  online: 'Online',
  inperson: 'In presenza',
  mixed: 'Mista',
};

const ATTENDANCE_LABELS: Record<string, string> = {
  pending: 'In attesa',
  missing: 'Assente',
  inprogress: 'In corso',
  completed: 'Completata',
};

function labeled(value: string | undefined, labels: Record<string, string>): string {
  if (!value) return '—';
  return labels[value] ?? value;
}

function formatCost(value: string | undefined, currency: string | undefined): string {
  if (!value) return '—';
  const formatted = formatCurrency(Number(value), currency || 'EUR');
  if (formatted !== null) return formatted;
  return currency ? `${value} ${currency}` : value;
}

function personKey(primary: string | undefined, fallback: string | undefined): string | null {
  const key = (primary ?? '').trim() || (fallback ?? '').trim();
  return key || null;
}

function sessionPeriod(session: FactorialSession): string {
  if (session.startsAt) {
    const start = formatInstant(session.startsAt);
    if (start === null) return '—';
    if (session.endsAt) {
      const end = formatInstant(session.endsAt);
      return end !== null ? `${start} – ${end}` : start;
    }
    return start;
  }
  if (session.dueDate) return formatLocalDate(session.dueDate) ?? '—';
  return '—';
}

function formatRunInstant(value: string | undefined): string {
  if (!value) return '—';
  return formatInstant(value, { format: { dateStyle: 'medium', timeStyle: 'short' } }) ?? '—';
}

function formatEuro(decimal: string | undefined): string {
  if (!decimal) return '—';
  const value = Number(decimal);
  if (Number.isNaN(value)) return decimal;
  return value.toLocaleString('it-IT', { style: 'currency', currency: 'EUR' });
}

export function FactorialPage() {
  const [params, setParams] = useSearchParams();
  const rawView = params.get('vista');
  const view: View =
    rawView === 'team' || rawView === 'formazione' || rawView === 'sincronizzazione' ? rawView : 'persone';

  const status = useFactorialStatus(true);
  const configured = status.data?.configured === true;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Factorial</h1>
          <p className={styles.subtitle}>
            Dati presenti in Factorial: anagrafica, team e formazione. Consultazione in sola lettura;
            la sincronizzazione allinea l'anagrafica del tool.
          </p>
        </div>
        <nav className={styles.viewSwitch} aria-label="Vista dati Factorial">
          {VIEWS.map((v) => (
            <button
              key={v.key}
              type="button"
              className={v.key === view ? styles.viewButtonActive : styles.viewButton}
              aria-current={v.key === view ? 'true' : undefined}
              onClick={() => {
                const next = new URLSearchParams(params);
                next.set('vista', v.key);
                setParams(next, { replace: true });
              }}
            >
              {v.label}
            </button>
          ))}
        </nav>
      </header>

      {status.isLoading ? (
        <Skeleton rows={6} />
      ) : !configured ? (
        <p className={styles.notice}>
          Connessione a Factorial non configurata. Impostare la chiave API nel backend per abilitare la
          consultazione.
        </p>
      ) : status.data?.error ? (
        <p className={styles.errorNotice}>Factorial non raggiungibile: {status.data.error}</p>
      ) : (
        <>
          {view === 'persone' && <EmployeesView />}
          {view === 'team' && <TeamsView />}
          {view === 'formazione' && <TrainingsView />}
          {view === 'sincronizzazione' && <SyncView />}
        </>
      )}
    </main>
  );
}

function QueryStates({
  isLoading,
  isError,
  children,
}: {
  isLoading: boolean;
  isError: boolean;
  children: React.ReactNode;
}) {
  if (isLoading) return <Skeleton rows={8} />;
  if (isError) {
    return <p className={styles.errorNotice}>Lettura da Factorial non riuscita. Riprovare o verificare la chiave API.</p>;
  }
  return <>{children}</>;
}

function EmployeesView() {
  const employees = useFactorialEmployees(true);
  const rows = employees.data ?? [];
  const active = rows.filter((e) => e.active);
  const withoutLoginEmail = active.filter((e) => !e.loginEmail).length;
  const withoutManager = active.filter((e) => !e.manager).length;

  return (
    <QueryStates isLoading={employees.isLoading} isError={employees.isError}>
      <div className={styles.facts}>
        <Fact label="attive" value={active.length} />
        <Fact label="cessate" value={rows.length - active.length} />
        <Fact label="senza email di login" value={withoutLoginEmail} warnWhenPositive />
        <Fact label="senza responsabile" value={withoutManager} warnWhenPositive />
      </div>
      {rows.length === 0 ? (
        <p className={styles.empty}>Nessuna persona presente in Factorial.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>ID</th>
                <th>Nome</th>
                <th>Email di login</th>
                <th>Responsabile</th>
                <th>Stato</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((e: FactorialEmployee) => (
                <tr key={e.id}>
                  <td className={styles.idCell}>{e.id}</td>
                  <td>{e.fullName}</td>
                  <td>{e.loginEmail || <span className={styles.missing}>mancante</span>}</td>
                  <td>{e.manager?.name || <span className={styles.missing}>mancante</span>}</td>
                  <td>
                    {e.active ? (
                      <StatusBadge value="attiva" label="Attiva" variant="success" />
                    ) : (
                      <StatusBadge
                        value="cessata"
                        label={`Cessata${e.terminatedOn ? ` il ${formatLocalDate(e.terminatedOn) ?? ''}` : ''}`}
                        variant="neutral"
                      />
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </QueryStates>
  );
}

function TeamsView() {
  const teams = useFactorialTeams(true);
  const rows = teams.data ?? [];
  const withoutLead = rows.filter((t) => t.leads.length === 0).length;
  const memberCounts = new Map<string, number>();
  for (const t of rows) {
    for (const m of t.members) memberCounts.set(m.id, (memberCounts.get(m.id) ?? 0) + 1);
  }
  const inMultipleTeams = [...memberCounts.values()].filter((n) => n > 1).length;

  return (
    <QueryStates isLoading={teams.isLoading} isError={teams.isError}>
      <div className={styles.facts}>
        <Fact label="team" value={rows.length} />
        <Fact label="senza lead" value={withoutLead} warnWhenPositive />
        <Fact label="persone in più team" value={inMultipleTeams} />
      </div>
      {rows.length === 0 ? (
        <p className={styles.empty}>Nessun team presente in Factorial.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>ID</th>
                <th>Team</th>
                <th>Lead</th>
                <th>Membri</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((t: FactorialTeam) => (
                <tr key={t.id}>
                  <td className={styles.idCell}>{t.id}</td>
                  <td>{t.name}</td>
                  <td>
                    {t.leads.length > 0 ? (
                      t.leads.map((l) => l.name || l.id).join(', ')
                    ) : (
                      <span className={styles.missing}>mancante</span>
                    )}
                  </td>
                  <td>
                    {t.members.length > 0 ? (
                      <ul className={styles.personList}>
                        {t.members.map((m) => (
                          <li key={m.id}>{m.name || m.id}</li>
                        ))}
                      </ul>
                    ) : (
                      <span className={styles.mutedCell}>Nessun membro</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </QueryStates>
  );
}

function TrainingsView() {
  const trainings = useFactorialTrainings(true);
  const [selected, setSelected] = useState<FactorialTraining | null>(null);
  const rows = trainings.data ?? [];
  const years = [...new Set(rows.map((t) => t.year).filter((y): y is number => y != null))].sort();

  return (
    <QueryStates isLoading={trainings.isLoading} isError={trainings.isError}>
      <div className={styles.facts}>
        <Fact label="corsi" value={rows.length} />
        <Fact label="a catalogo" value={rows.filter((t) => t.catalog).length} />
        {years.length > 0 && (
          <span className={styles.fact}>
            anni coperti
            <span className={styles.factValue}>
              {years.length === 1 ? years[0] : `${years[0]}–${years[years.length - 1]}`}
            </span>
          </span>
        )}
      </div>
      {rows.length === 0 ? (
        <p className={styles.empty}>
          Nessun corso presente in Factorial: il modulo formazione non risulta utilizzato.
        </p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>ID</th>
                <th>Corso</th>
                <th>Anno</th>
                <th>Stato</th>
                <th>Fornitore</th>
                <th>Categorie</th>
                <th className={styles.numCell}>Costo</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((t: FactorialTraining) => (
                <tr key={t.id}>
                  <td className={styles.idCell}>{t.id}</td>
                  <td>
                    <button type="button" className={styles.rowButton} onClick={() => setSelected(t)}>
                      {t.name}
                    </button>
                  </td>
                  <td>{t.year ?? '—'}</td>
                  <td>{t.status || '—'}</td>
                  <td>{t.provider || (t.external ? 'Esterno' : '—')}</td>
                  <td className={styles.mutedCell}>{t.categories?.join(', ') || '—'}</td>
                  <td className={styles.numCell}>{formatEuro(t.cost)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <TrainingDetailDrawer training={selected} onClose={() => setSelected(null)} />
    </QueryStates>
  );
}

function TrainingDetailDrawer({
  training,
  onClose,
}: {
  training: FactorialTraining | null;
  onClose: () => void;
}) {
  const memberships = useFactorialTrainingMemberships(training?.id ?? null);
  const structure = useFactorialTrainingStructure(training?.id ?? null);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const seenTrainingId = useRef<string | null>(null);

  useEffect(() => {
    const id = training?.id ?? null;
    if (id !== seenTrainingId.current) {
      seenTrainingId.current = id;
      setSelectedSessionId(null);
    }
  }, [training?.id]);

  const classes = structure.data?.classes ?? [];
  const sessions = structure.data?.sessions ?? [];
  const classById = new Map(classes.map((c) => [c.id, c]));
  const selectedSession = sessions.find((s) => s.id === selectedSessionId) ?? null;
  const membershipRows = memberships.data ?? [];

  return (
    <Drawer open={training !== null} onClose={onClose} title={training?.name ?? ''} size="lg">
      <div className={styles.drawerBody}>
        <section className={styles.drawerSection}>
          <h2 className={styles.sectionTitle}>Iscrizioni al training</h2>
          <p className={styles.drawerMeta}>Iscrizioni registrate in Factorial per questo corso.</p>
          {memberships.isLoading ? (
            <Skeleton rows={4} />
          ) : memberships.isError ? (
            <p className={styles.errorNotice}>Lettura delle iscrizioni non riuscita.</p>
          ) : membershipRows.length === 0 ? (
            <p className={styles.empty}>Nessuna iscrizione registrata.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>ID persona</th>
                    <th>Persona</th>
                    <th>Stato</th>
                    <th>Scadenza</th>
                    <th>Completato</th>
                  </tr>
                </thead>
                <tbody>
                  {membershipRows.map((m) => (
                    <tr key={`${m.employeeId}`}>
                      <td className={styles.idCell}>{m.employeeId}</td>
                      <td>{m.employeeName || m.employeeId}</td>
                      <td>{m.status || '—'}</td>
                      <td>{m.dueDate ? (formatLocalDate(m.dueDate) ?? '—') : '—'}</td>
                      <td>{m.completedAt ? (formatLocalDate(m.completedAt) ?? '—') : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className={styles.drawerSection}>
          <h2 className={styles.sectionTitle}>Classi</h2>
          {structure.isLoading ? (
            <Skeleton rows={4} />
          ) : structure.isError ? (
            <p className={styles.errorNotice}>Lettura della struttura del corso non riuscita.</p>
          ) : classes.length === 0 ? (
            <p className={styles.empty}>Nessuna classe registrata.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Nome</th>
                    <th>Periodo</th>
                    <th className={styles.numCell}>Costo</th>
                    <th>Pagamento</th>
                    <th className={styles.numCell}>Attendance</th>
                  </tr>
                </thead>
                <tbody>
                  {classes.map((c) => (
                    <tr key={c.id}>
                      <td className={styles.idCell}>{c.id}</td>
                      <td>{c.name || '—'}</td>
                      <td>
                        {c.startDate ? (formatLocalDate(c.startDate) ?? '—') : '—'}
                        {c.endDate ? ` – ${formatLocalDate(c.endDate) ?? ''}` : ''}
                      </td>
                      <td className={styles.numCell}>{formatCost(c.cost, c.currency)}</td>
                      <td>{c.paymentStatus || '—'}</td>
                      <td className={styles.numCell}>
                        {c.totalAttendancesCount != null
                          ? `${c.completedAttendancesCount ?? 0}/${c.totalAttendancesCount}`
                          : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className={styles.drawerSection}>
          <h2 className={styles.sectionTitle}>Sessioni</h2>
          {structure.isLoading ? (
            <Skeleton rows={4} />
          ) : structure.isError ? (
            <p className={styles.errorNotice}>Lettura della struttura del corso non riuscita.</p>
          ) : sessions.length === 0 ? (
            <p className={styles.empty}>Nessuna sessione registrata.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Nome</th>
                    <th>Classe</th>
                    <th>Tipo</th>
                    <th>Periodo / scadenza</th>
                    <th className={styles.numCell}>Durata</th>
                    <th>Modalità</th>
                    <th>Stato</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.map((s) => {
                    const cls = s.trainingClassId ? classById.get(s.trainingClassId) : undefined;
                    return (
                      <tr key={s.id}>
                        <td className={styles.idCell}>{s.id}</td>
                        <td>
                          <button
                            type="button"
                            className={styles.rowButton}
                            aria-pressed={s.id === selectedSessionId}
                            onClick={() =>
                              setSelectedSessionId(s.id === selectedSessionId ? null : s.id)
                            }
                          >
                            {s.name || s.id}
                          </button>
                        </td>
                        <td>
                          {cls?.name || <span className={styles.mutedCell}>Senza classe</span>}
                        </td>
                        <td>{labeled(s.schedule, SCHEDULE_LABELS)}</td>
                        <td>{sessionPeriod(s)}</td>
                        <td className={styles.numCell}>{s.duration || '—'}</td>
                        <td>{labeled(s.modality, MODALITY_LABELS)}</td>
                        <td>{s.status || '—'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>

        {selectedSession && (
          <SessionParticipantsSection session={selectedSession} memberships={membershipRows} />
        )}
      </div>
    </Drawer>
  );
}

function SessionParticipantsSection({
  session,
  memberships,
}: {
  session: FactorialSession;
  memberships: FactorialMembership[];
}) {
  const participants = useFactorialSessionParticipants(session.id);
  const rows = participants.data?.participants ?? [];
  const unmatched = participants.data?.unmatchedAttendances ?? [];

  const statusCounts = new Map<string, number>();
  for (const p of rows) {
    for (const a of p.attendances) {
      const status = a.status || '—';
      statusCounts.set(status, (statusCounts.get(status) ?? 0) + 1);
    }
  }

  const membershipKeys = new Set<string>();
  let membershipsUnkeyed = 0;
  for (const m of memberships) {
    const key = personKey(m.employeeId, m.accessId);
    if (key) membershipKeys.add(key);
    else membershipsUnkeyed += 1;
  }
  const participantKeys = new Set<string>();
  let participantsUnkeyed = 0;
  for (const p of rows) {
    const key = personKey(p.employeeId, p.accessId);
    if (key) participantKeys.add(key);
    else participantsUnkeyed += 1;
  }
  const membershipNotAssigned = [...membershipKeys].filter((k) => !participantKeys.has(k)).length;
  const assignmentsWithoutMembership = [...participantKeys].filter((k) => !membershipKeys.has(k)).length;

  const membershipDups = memberships.length - membershipKeys.size - membershipsUnkeyed;
  const participantDups = rows.length - participantKeys.size - participantsUnkeyed;
  const dupParts: string[] = [];
  if (membershipDups > 0) dupParts.push(`${membershipDups} membership duplicate`);
  if (participantDups > 0) dupParts.push(`${participantDups} assegnazioni duplicate`);

  return (
    <section className={styles.drawerSection}>
      <h2 className={styles.sectionTitle}>Partecipazione alla sessione</h2>
      <div className={styles.facts}>
        <span className={styles.fact}>
          sessione
          <span className={styles.factValue}>{session.name || session.id}</span>
        </span>
        <span className={styles.fact}>
          ID sessione
          <span className={styles.factValue}>{session.id}</span>
        </span>
        <span className={styles.fact}>
          tipo
          <span className={styles.factValue}>{labeled(session.schedule, SCHEDULE_LABELS)}</span>
        </span>
        <span className={styles.fact}>
          stato
          <span className={styles.factValue}>{session.status || '—'}</span>
        </span>
      </div>

      {participants.isLoading ? (
        <Skeleton rows={4} />
      ) : participants.isError ? (
        <p className={styles.errorNotice}>Lettura delle partecipazioni non riuscita.</p>
      ) : (
        <>
          <div className={styles.compareRow} role="group" aria-label="Confronto membership e assegnazioni">
            <span className={styles.compareTitle}>Confronto</span>
            <span className={styles.fact}>
              membership complessive
              <span className={styles.factValue}>{membershipKeys.size}</span>
            </span>
            <span className={styles.fact}>
              assegnati alla sessione
              <span className={styles.factValue}>{participantKeys.size}</span>
            </span>
            <span className={styles.fact}>
              membership non assegnate
              <span className={styles.factValue}>{membershipNotAssigned}</span>
            </span>
            <span className={styles.fact}>
              assegnazioni senza membership
              <span className={styles.factValue}>{assignmentsWithoutMembership}</span>
            </span>
            <span className={styles.fact}>
              non confrontabili
              <span className={styles.factValue}>{membershipsUnkeyed + participantsUnkeyed}</span>
            </span>
          </div>
          {dupParts.length > 0 && (
            <p className={styles.drawerMeta}>Duplicati osservati: {dupParts.join(', ')}.</p>
          )}

          {statusCounts.size === 0 ? (
            <p className={styles.empty}>Nessuna attendance registrata per questa sessione.</p>
          ) : (
            <div className={styles.facts}>
              {[...statusCounts.entries()]
                .sort(
                  (a, b) =>
                    (ATTENDANCE_STATUS_ORDER.indexOf(a[0]) + 1 || 99) -
                    (ATTENDANCE_STATUS_ORDER.indexOf(b[0]) + 1 || 99),
                )
                .map(([status, count]) => (
                  <span key={status} className={styles.fact}>
                    {labeled(status, ATTENDANCE_LABELS)}
                    <span className={styles.factValue}>{count}</span>
                  </span>
                ))}
            </div>
          )}

          {rows.length === 0 ? (
            <p className={styles.empty}>Nessun partecipante assegnato a questa sessione.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Persona</th>
                    <th>Employee / access ID</th>
                    <th>Attendance</th>
                    <th className={styles.numCell}>Durata completata</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((p) => (
                    <tr key={p.sessionAccessMembershipId}>
                      <td>
                        {[p.firstName, p.lastName].filter(Boolean).join(' ') ||
                          p.employeeId ||
                          p.accessId ||
                          '—'}
                      </td>
                      <td className={styles.idCell}>
                        {p.employeeId || '—'}
                        {p.accessId ? ` / ${p.accessId}` : ''}
                      </td>
                      <td>
                        {p.attendances.length > 0
                          ? p.attendances.map((a) => labeled(a.status, ATTENDANCE_LABELS)).join(', ')
                          : '—'}
                      </td>
                      <td className={styles.numCell}>
                        {p.attendances.map((a) => a.completedDuration).filter(Boolean).join(', ') ||
                          '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {unmatched.length > 0 && (
            <div className={styles.unmatchedBlock}>
              <h3 className={styles.sectionSubtitle}>Attendance non collegate</h3>
              <p className={styles.drawerMeta}>
                {unmatched.length}{' '}
                {unmatched.length === 1
                  ? 'attendance non collegata a una membership della sessione.'
                  : 'attendance non collegate a una membership della sessione.'}
              </p>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Employee / access ID</th>
                      <th>Stato</th>
                      <th className={styles.numCell}>Durata</th>
                    </tr>
                  </thead>
                  <tbody>
                    {unmatched.map((a) => (
                      <tr key={a.id}>
                        <td className={styles.idCell}>{a.id}</td>
                        <td className={styles.idCell}>
                          {a.employeeId || '—'}
                          {a.accessId ? ` / ${a.accessId}` : ''}
                        </td>
                        <td>{labeled(a.status, ATTENDANCE_LABELS)}</td>
                        <td className={styles.numCell}>{a.completedDuration || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </section>
  );
}

function SyncView() {
  const { toast } = useToast();
  const runs = useDirectorySyncRuns(true);
  const runSync = useRunDirectorySync();
  const [plan, setPlan] = useState<DirectorySyncRun | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const history = runs.data ?? [];
  const lastRun = history[0];

  async function verify() {
    try {
      const run = await runSync.mutateAsync(true);
      setPlan(run);
      if ((run.stats?.total ?? 0) === 0) toast('Nessuna modifica da applicare');
    } catch {
      toast('Verifica non riuscita', 'error');
    }
  }

  async function apply() {
    setConfirmOpen(false);
    try {
      const run = await runSync.mutateAsync(false);
      setPlan(run);
      toast('Sincronizzazione applicata');
    } catch {
      toast('Sincronizzazione non riuscita', 'error');
    }
  }

  const planTotal = plan?.stats?.total ?? 0;
  const canApply = plan !== null && plan.dryRun && planTotal > 0;

  return (
    <div className={styles.syncLayout}>
      <div className={styles.syncActions}>
        <Button variant="primary" size="md" onClick={verify} loading={runSync.isPending}>
          Verifica modifiche
        </Button>
        {canApply && (
          <Button variant="secondary" size="md" onClick={() => setConfirmOpen(true)}>
            Applica sincronizzazione
          </Button>
        )}
      </div>

      {lastRun && (
        <div className={styles.facts}>
          <span className={styles.fact}>
            ultima esecuzione
            <span className={styles.factValue}>{formatRunInstant(lastRun.startedAt)}</span>
          </span>
          <span className={styles.fact}>
            tipo
            <span className={styles.factValue}>{lastRun.dryRun ? 'verifica' : 'applicata'}</span>
          </span>
          <span className={styles.fact}>
            esito
            <span className={lastRun.status === 'failed' ? styles.factValueWarning : styles.factValue}>
              {runStatusLabel(lastRun.status)}
            </span>
          </span>
          <span className={styles.fact}>
            richiesta da
            <span className={styles.factValue}>{lastRun.actor}</span>
          </span>
        </div>
      )}

      {plan ? <PlanReport run={plan} /> : <p className={styles.notice}>
        La verifica confronta l'anagrafica di Factorial con quella del tool e mostra le modifiche
        prima di applicarle.
      </p>}

      <RunHistory runs={history} isLoading={runs.isLoading} isError={runs.isError} />

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="Applicare la sincronizzazione?"
        size="sm"
      >
        <div className={styles.drawerBody}>
          <p className={styles.drawerMeta}>
            {planTotal} modifiche verranno scritte sull'anagrafica del tool. Le persone e i team
            sincronizzati diventano di sola lettura.
          </p>
          <div className={styles.syncActions}>
            <Button variant="ghost" size="md" onClick={() => setConfirmOpen(false)}>
              Annulla
            </Button>
            <Button variant="primary" size="md" onClick={apply} loading={runSync.isPending}>
              Applica
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function runStatusLabel(status: DirectorySyncRun['status']): string {
  switch (status) {
    case 'ok':
      return 'completata';
    case 'failed':
      return 'non riuscita';
    default:
      return 'in corso';
  }
}

function PlanReport({ run }: { run: DirectorySyncRun }) {
  const stats = run.stats;
  if (!stats) {
    return <p className={styles.errorNotice}>{run.error || 'Esecuzione senza resoconto.'}</p>;
  }
  const groups = ACTION_ORDER.filter((key) => (stats.counts[key] ?? 0) > 0);
  const source = stats.source.stats;

  return (
    <section className={styles.syncPlan}>
      <div className={styles.facts}>
        <Fact label="modifiche" value={stats.total} />
        <Fact label="persone in Factorial" value={source.people} />
        <Fact label="team in Factorial" value={source.teams} />
        <Fact label="senza email di login" value={source.skippedNoLoginEmail} warnWhenPositive />
        <Fact label="record duplicati" value={source.skippedDuplicate} />
        <Fact label="appartenenze di cessati" value={source.skippedMembership} />
        <Fact label="in gestione manuale" value={source.exemptLocal ?? 0} />
      </div>
      {groups.length === 0 ? (
        <p className={styles.empty}>Anagrafica allineata: nessuna modifica.</p>
      ) : (
        <div className={styles.syncGroups}>
          {groups.map((key) => (
            <div key={key} className={styles.syncGroup}>
              <h2 className={styles.syncGroupTitle}>
                {ACTION_LABELS[key]}
                <span className={styles.factValue}>{stats.counts[key]}</span>
              </h2>
              <ul className={styles.personList}>
                {(stats.samples[key] ?? []).map((entry, index) => (
                  <li key={`${key}-${index}`}>{entry}</li>
                ))}
              </ul>
              {(stats.counts[key] ?? 0) > (stats.samples[key]?.length ?? 0) && (
                <p className={styles.drawerMeta}>
                  Elenco parziale: {stats.samples[key]?.length ?? 0} voci su {stats.counts[key]}.
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function RunHistory({
  runs,
  isLoading,
  isError,
}: {
  runs: DirectorySyncRun[];
  isLoading: boolean;
  isError: boolean;
}) {
  return (
    <QueryStates isLoading={isLoading} isError={isError}>
      {runs.length === 0 ? (
        <p className={styles.empty}>Nessuna sincronizzazione registrata.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Avvio</th>
                <th>Tipo</th>
                <th>Esito</th>
                <th className={styles.numCell}>Modifiche</th>
                <th>Richiesta da</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <tr key={run.id}>
                  <td>{formatRunInstant(run.startedAt)}</td>
                  <td>{run.dryRun ? 'verifica' : 'applicata'}</td>
                  <td>
                    <StatusBadge
                      value={run.status}
                      label={runStatusLabel(run.status)}
                      variant={run.status === 'ok' ? 'success' : run.status === 'failed' ? 'danger' : 'neutral'}
                    />
                  </td>
                  <td className={styles.numCell}>{run.stats?.total ?? '—'}</td>
                  <td className={styles.mutedCell}>{run.actor}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </QueryStates>
  );
}

function Fact({
  label,
  value,
  warnWhenPositive = false,
}: {
  label: string;
  value: number;
  warnWhenPositive?: boolean;
}) {
  return (
    <span className={styles.fact}>
      {label}
      <span className={warnWhenPositive && value > 0 ? styles.factValueWarning : styles.factValue}>
        {value.toLocaleString('it-IT')}
      </span>
    </span>
  );
}
