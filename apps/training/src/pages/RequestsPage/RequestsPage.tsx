// Lista delle richieste formative (#157, §Richieste 1): filtro server
// state=open|closed|all (default aperte) + ricerca client su persona, corso
// o titolo libero e team. Il rimando dalla coda di lavoro apre il dettaglio
// via ?id=, letto anche qui per il deep link.

import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Icon, SearchInput, Skeleton, SingleSelect, StatusBadge, TableToolbar } from '@mrsmith/ui';
import { useTrainingRequests } from '../../api/queries';
import type { RequestListRow } from '../../api/types';
import { RequestCreateModal } from '../../components/requests/RequestCreateModal';
import { RequestDetailDrawer } from '../../components/requests/RequestDetailDrawer';
import { formatInstantDate } from '../../components/events/eventFormat';
import { ReminderBadge } from '../../components/reminders/ReminderBadge';
import { outcomeVariant, tlOpinionVariant } from '../../components/requests/requestVariants';
import { REQUEST_OUTCOME_LABELS, REQUEST_STATE_LABELS, TL_OPINION_LABELS } from '../../lib/labels';
import styles from './listPage.module.css';

const STATE_OPTIONS = (['open', 'suspended', 'closed', 'all'] as const).map((value) => ({
  value,
  label: REQUEST_STATE_LABELS[value],
}));

export function RequestsPage() {
  const [params, setParams] = useSearchParams();
  const state = (['open', 'suspended', 'closed', 'all'] as const).includes(params.get('state') as never)
    ? (params.get('state') as 'open' | 'suspended' | 'closed' | 'all')
    : 'open';
  const selectedId = params.get('id');
  const [q, setQ] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  const requests = useTrainingRequests(state);

  function setState(next: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('state', next);
    setParams(nextParams, { replace: true });
  }

  function openDetail(id: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('id', id);
    setParams(nextParams, { replace: true });
  }

  function closeDetail() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('id');
    setParams(nextParams, { replace: true });
  }

  const filtered = useMemo(() => {
    const query = q.trim().toLowerCase();
    const rows = requests.data ?? [];
    if (!query) return rows;
    return rows.filter((r) =>
      [r.employeeName, r.courseTitle, r.selectedTeamName]
        .filter((v): v is string => Boolean(v))
        .some((v) => v.toLowerCase().includes(query)),
    );
  }, [requests.data, q]);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Richieste</h1>
          <p className={styles.subtitle}>
            Richieste formative espresse dalle persone, con parere del lead e decisione People.
          </p>
        </div>
        <Button
          variant="primary"
          size="md"
          leftIcon={<Icon name="plus" size={16} />}
          onClick={() => setShowCreate(true)}
        >
          Registra richiesta
        </Button>
      </header>

      <TableToolbar
        activeFilterCount={state !== 'open' ? 1 : 0}
        filters={
          <SingleSelect
            options={STATE_OPTIONS}
            selected={state}
            onChange={(v) => setState(v ?? 'open')}
            placeholder="Stato"
            ariaLabel="Filtra per stato"
          />
        }
      >
        <SearchInput value={q} onChange={setQ} placeholder="Cerca per persona, corso o team..." />
      </TableToolbar>

      {requests.isLoading ? (
        <Skeleton rows={6} />
      ) : requests.isError ? (
        <p className={styles.errorNotice}>Lettura delle richieste non riuscita. Riprovare più tardi.</p>
      ) : (requests.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>
            <Icon name="file-text" size={32} />
          </div>
          <p className={styles.emptyTitle}>
            {state === 'open' ? 'Nessuna richiesta aperta' : state === 'suspended' ? 'Nessuna richiesta sospesa' : state === 'closed' ? 'Nessuna richiesta chiusa' : 'Nessuna richiesta'}
          </p>
          <p className={styles.emptyDescription}>
            {state === 'open' || state === 'all'
              ? 'Registra una richiesta per conto di una persona.'
              : 'Cambia il filtro dello stato per vedere le altre richieste.'}
          </p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Registra richiesta
          </Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyTitle}>Nessuna richiesta corrisponde alla ricerca</p>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Persona</th>
                <th>Corso o titolo</th>
                <th>Team</th>
                <th>Priorità</th>
                <th>Promemoria</th>
                <th>Parere TL</th>
                <th>Decisione</th>
                <th>Esito</th>
                <th>Creata il</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row: RequestListRow) => (
                <tr key={row.id} className={styles.row} onClick={() => openDetail(row.id)}>
                  <td>
                    <button
                      type="button"
                      className={styles.rowLink}
                      onClick={(e) => {
                        e.stopPropagation();
                        openDetail(row.id);
                      }}
                    >
                      {row.employeeName}
                    </button>
                  </td>
                  <td className={styles.wrapCell}>
                    <span className={styles.inlineBadges}>
                      {row.courseTitle || '—'}
                      {row.suspendedAt && <StatusBadge value="suspended" label="Sospesa" variant="warning" />}
                    </span>
                  </td>
                  <td>{row.selectedTeamName}</td>
                  <td>
                    {row.priority != null ? (
                      <StatusBadge value="priority" label={`P${row.priority}`} variant="neutral" />
                    ) : (
                      '—'
                    )}
                  </td>
                  <td className={styles.wrapCell}>
                    <ReminderBadge text={row.reminderText} date={row.reminderAt} fallback="—" neutral={Boolean(row.suspendedAt)} />
                  </td>
                  <td>
                    {row.tlOpinion ? (
                      <StatusBadge
                        value={row.tlOpinion}
                        label={TL_OPINION_LABELS[row.tlOpinion]}
                        variant={tlOpinionVariant(row.tlOpinion)}
                      />
                    ) : (
                      '—'
                    )}
                  </td>
                  <td>
                    {row.peopleDecision ? (
                      <StatusBadge
                        value={row.peopleDecision}
                        label={REQUEST_OUTCOME_LABELS[row.peopleDecision] ?? row.peopleDecision}
                        variant={outcomeVariant(row.peopleDecision)}
                      />
                    ) : (
                      '—'
                    )}
                  </td>
                  <td>
                    {row.outcome ? (
                      <StatusBadge
                        value={row.outcome}
                        label={REQUEST_OUTCOME_LABELS[row.outcome] ?? row.outcome}
                        variant={outcomeVariant(row.outcome)}
                      />
                    ) : (
                      '—'
                    )}
                  </td>
                  <td>{formatInstantDate(row.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <RequestCreateModal
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            openDetail(id);
          }}
        />
      )}

      {selectedId && <RequestDetailDrawer id={selectedId} onClose={closeDetail} />}
    </main>
  );
}
