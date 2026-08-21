import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatLocalDate } from '@mrsmith/format';
import { Drawer, Skeleton, StatusBadge } from '@mrsmith/ui';
import {
  useFactorialEmployees,
  useFactorialStatus,
  useFactorialTeams,
  useFactorialTrainingMemberships,
  useFactorialTrainings,
  type FactorialEmployee,
  type FactorialTeam,
  type FactorialTraining,
} from '../api/factorialDiag';
import styles from './FactorialPage.module.css';

interface FactorialPageProps {
  isPeopleAdmin: boolean;
}

type View = 'persone' | 'team' | 'formazione';

const VIEWS: { key: View; label: string }[] = [
  { key: 'persone', label: 'Persone' },
  { key: 'team', label: 'Team' },
  { key: 'formazione', label: 'Formazione' },
];

function formatEuro(decimal: string | undefined): string {
  if (!decimal) return '—';
  const value = Number(decimal);
  if (Number.isNaN(value)) return decimal;
  return value.toLocaleString('it-IT', { style: 'currency', currency: 'EUR' });
}

export function FactorialPage({ isPeopleAdmin }: FactorialPageProps) {
  const [params, setParams] = useSearchParams();
  const rawView = params.get('vista');
  const view: View = rawView === 'team' || rawView === 'formazione' ? rawView : 'persone';

  const status = useFactorialStatus(isPeopleAdmin);
  const configured = status.data?.configured === true;

  if (!isPeopleAdmin) {
    return (
      <main className={styles.page}>
        <p className={styles.accessDenied}>Accesso riservato al team People.</p>
      </main>
    );
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Factorial</h1>
          <p className={styles.subtitle}>
            Dati presenti in Factorial: anagrafica, team e formazione. Sola lettura.
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
                <th>Nome</th>
                <th>Email di login</th>
                <th>Responsabile</th>
                <th>Stato</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((e: FactorialEmployee) => (
                <tr key={e.id}>
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
                <th>Team</th>
                <th>Lead</th>
                <th>Membri</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((t: FactorialTeam) => (
                <tr key={t.id}>
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
      <MembershipsDrawer training={selected} onClose={() => setSelected(null)} />
    </QueryStates>
  );
}

function MembershipsDrawer({
  training,
  onClose,
}: {
  training: FactorialTraining | null;
  onClose: () => void;
}) {
  const memberships = useFactorialTrainingMemberships(training?.id ?? null);
  const rows = memberships.data ?? [];
  return (
    <Drawer open={training !== null} onClose={onClose} title={training?.name ?? ''}>
      <div className={styles.drawerBody}>
        <p className={styles.drawerMeta}>Iscrizioni registrate in Factorial per questo corso.</p>
        {memberships.isLoading ? (
          <Skeleton rows={5} />
        ) : memberships.isError ? (
          <p className={styles.errorNotice}>Lettura delle iscrizioni non riuscita.</p>
        ) : rows.length === 0 ? (
          <p className={styles.empty}>Nessuna iscrizione registrata.</p>
        ) : (
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Persona</th>
                <th>Stato</th>
                <th>Scadenza</th>
                <th>Completato</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((m) => (
                <tr key={`${m.employeeId}`}>
                  <td>{m.employeeName || m.employeeId}</td>
                  <td>{m.status || '—'}</td>
                  <td>{m.dueDate ? (formatLocalDate(m.dueDate) ?? '—') : '—'}</td>
                  <td>{m.completedAt ? (formatLocalDate(m.completedAt) ?? '—') : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Drawer>
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
