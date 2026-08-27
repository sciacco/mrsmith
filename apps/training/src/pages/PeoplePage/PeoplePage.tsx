// Directory persone (#158, §Persone 1): filtri client per team, gruppo,
// stato e testo. La modifica di team/gruppi gestiti dalla sync resta
// bloccata dal backend (person_managed_by_directory) — nessuna previsione
// locale di quali persone siano agganciate, il form si limita a mostrare
// l'errore.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Icon, SearchInput, Skeleton, SingleSelect, StatusBadge, TableToolbar } from '@mrsmith/ui';
import { useTrainingGroups, useTrainingPeople, useTrainingTeams } from '../../api/queries';
import type { PersonListRow } from '../../api/types';
import { GroupsSection } from '../../components/people/GroupsSection';
import { PersonEditorModal } from '../../components/people/PersonEditorModal';
import { PERSON_STATUS_LABELS } from '../../lib/labels';
import styles from '../RequestsPage/listPage.module.css';

function statusVariant(status: string): 'success' | 'warning' | 'neutral' {
  if (status === 'active') return 'success';
  if (status === 'on_leave') return 'warning';
  return 'neutral';
}

function teamsCell(row: PersonListRow): string {
  if (row.teams.length === 0) return '—';
  return row.teams.map((t) => (t.role === 'lead' ? `${t.name} (lead)` : t.name)).join(', ');
}

export function PeoplePage() {
  const navigate = useNavigate();
  const people = useTrainingPeople();
  const teams = useTrainingTeams();
  const groups = useTrainingGroups();

  const [q, setQ] = useState('');
  const [teamId, setTeamId] = useState<string | null>(null);
  const [groupId, setGroupId] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const filtered = useMemo(() => {
    const query = q.trim().toLowerCase();
    return (people.data ?? []).filter((p) => {
      if (teamId && !p.teams.some((t) => t.id === teamId)) return false;
      if (groupId && !p.groups.some((g) => g.id === groupId)) return false;
      if (status && p.status !== status) return false;
      if (!query) return true;
      return `${p.firstName} ${p.lastName} ${p.email}`.toLowerCase().includes(query);
    });
  }, [people.data, q, teamId, groupId, status]);

  const activeFilterCount = (teamId ? 1 : 0) + (groupId ? 1 : 0) + (status ? 1 : 0);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Persone</h1>
          <p className={styles.subtitle}>Directory formativa: appartenenze, gruppi locali e gestione manuale.</p>
        </div>
        <Button
          variant="primary"
          size="md"
          leftIcon={<Icon name="plus" size={16} />}
          onClick={() => setShowCreate(true)}
        >
          Nuova persona
        </Button>
      </header>

      <TableToolbar
        activeFilterCount={activeFilterCount}
        filters={
          <>
            <SingleSelect
              options={(teams.data ?? []).map((t) => ({ value: t.id, label: t.name }))}
              selected={teamId}
              onChange={setTeamId}
              placeholder="Team"
              allowClear
              ariaLabel="Filtra per team"
            />
            <SingleSelect
              options={(groups.data ?? []).map((g) => ({ value: g.id, label: g.name }))}
              selected={groupId}
              onChange={setGroupId}
              placeholder="Gruppo"
              allowClear
              ariaLabel="Filtra per gruppo"
            />
            <SingleSelect
              options={Object.entries(PERSON_STATUS_LABELS).map(([value, label]) => ({ value, label }))}
              selected={status}
              onChange={setStatus}
              placeholder="Stato"
              allowClear
              ariaLabel="Filtra per stato"
            />
          </>
        }
      >
        <SearchInput value={q} onChange={setQ} placeholder="Cerca per nome o email..." />
      </TableToolbar>

      {people.isLoading ? (
        <Skeleton rows={6} />
      ) : people.isError ? (
        <p className={styles.errorNotice}>Lettura della directory non riuscita. Riprovare più tardi.</p>
      ) : (people.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>
            <Icon name="user" size={32} />
          </div>
          <p className={styles.emptyTitle}>Nessuna persona</p>
          <p className={styles.emptyDescription}>Crea la prima persona o esegui il sync anagrafico da Factorial.</p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Nuova persona
          </Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyTitle}>Nessuna persona corrisponde ai filtri</p>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Email</th>
                <th>Stato</th>
                <th>Team</th>
                <th>Gruppi</th>
                <th>Gestione</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => (
                <tr key={row.id} className={styles.row} onClick={() => navigate(`/persone/${row.id}`)}>
                  <td>
                    <button
                      type="button"
                      className={styles.rowLink}
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`/persone/${row.id}`);
                      }}
                    >
                      {row.lastName} {row.firstName}
                    </button>
                  </td>
                  <td>{row.email}</td>
                  <td>
                    <StatusBadge
                      value={row.status}
                      label={PERSON_STATUS_LABELS[row.status] ?? row.status}
                      variant={statusVariant(row.status)}
                    />
                  </td>
                  <td>{teamsCell(row)}</td>
                  <td>{row.groups.length === 0 ? '—' : row.groups.map((g) => g.name).join(', ')}</td>
                  <td>
                    {row.directoryExempt ? (
                      <StatusBadge value="exempt" label="Manuale" variant="accent" />
                    ) : (
                      <StatusBadge value="synced" label="Da directory" variant="neutral" />
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <GroupsSection />

      {showCreate && (
        <PersonEditorModal
          mode="create"
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onSaved={(id) => {
            setShowCreate(false);
            navigate(`/persone/${id}`);
          }}
        />
      )}
    </main>
  );
}
