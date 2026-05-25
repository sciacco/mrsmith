import { useDeferredValue, useMemo, useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useAllUsers } from './queries';
import { UserCreateModal } from './UserCreateModal';
import { UserEditModal } from './UserEditModal';
import { UserDisableConfirm } from './UserDisableConfirm';
import type { ArakIntUser } from '../../api/types';
import styles from './UtentiPage.module.css';

type SortKey = 'first_name' | 'last_name' | 'email' | 'role' | 'state' | 'updated';
type SortDirection = 'asc' | 'desc';

const COLLATOR = new Intl.Collator('it', { sensitivity: 'base' });
const DATE_FMT = new Intl.DateTimeFormat('it-IT', {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
});

function normalize(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
}

function initials(u: ArakIntUser): string {
  const f = u.first_name?.[0] ?? '';
  const l = u.last_name?.[0] ?? '';
  return (f + l).toUpperCase() || '?';
}

function isUserActive(u: ArakIntUser): boolean {
  return u.state?.enabled ?? u.enabled ?? false;
}

function compareUsers(a: ArakIntUser, b: ArakIntUser, key: SortKey, dir: SortDirection): number {
  const mul = dir === 'asc' ? 1 : -1;
  let primary = 0;
  switch (key) {
    case 'first_name':
      primary = COLLATOR.compare(a.first_name, b.first_name);
      break;
    case 'last_name':
      primary = COLLATOR.compare(a.last_name, b.last_name);
      break;
    case 'email':
      primary = COLLATOR.compare(a.email, b.email);
      break;
    case 'role':
      primary = COLLATOR.compare(a.role.name, b.role.name);
      break;
    case 'state': {
      const ae = isUserActive(a);
      const be = isUserActive(b);
      primary = ae === be ? 0 : ae ? -1 : 1;
      break;
    }
    case 'updated':
      primary = a.updated.localeCompare(b.updated);
      break;
  }
  if (primary !== 0) return primary * mul;
  // Stable tiebreak: first_name then last_name then id (always ascending)
  const fnTb = COLLATOR.compare(a.first_name, b.first_name);
  if (fnTb !== 0) return fnTb;
  const lnTb = COLLATOR.compare(a.last_name, b.last_name);
  if (lnTb !== 0) return lnTb;
  return a.id - b.id;
}

function ariaSort(active: boolean, dir: SortDirection): 'ascending' | 'descending' | 'none' {
  if (!active) return 'none';
  return dir === 'asc' ? 'ascending' : 'descending';
}

interface SortHeaderProps {
  sortKey: SortKey;
  label: string;
  active: SortKey;
  direction: SortDirection;
  onSort: (key: SortKey) => void;
}

function SortHeader({ sortKey, label, active, direction, onSort }: SortHeaderProps) {
  const isActive = sortKey === active;
  return (
    <button
      type="button"
      className={`${styles.sortButton} ${isActive ? styles.sortButtonActive : ''}`}
      onClick={() => onSort(sortKey)}
    >
      <span>{label}</span>
      <span className={styles.sortGlyph} aria-hidden="true">
        {isActive ? (direction === 'asc' ? '▲' : '▼') : '↕'}
      </span>
    </button>
  );
}

export function UtentiPage() {
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [sortKey, setSortKey] = useState<SortKey>('state');
  const [sortDir, setSortDir] = useState<SortDirection>('asc');
  const [showCreate, setShowCreate] = useState(false);
  const [editTarget, setEditTarget] = useState<ArakIntUser | null>(null);
  const [disableTarget, setDisableTarget] = useState<ArakIntUser | null>(null);

  const { data: users, isLoading, error } = useAllUsers();

  const filtered = useMemo(() => {
    if (!users) return [];
    const needle = normalize(deferredSearch.trim());
    if (!needle) return users;
    return users.filter((u) => {
      const haystack = normalize(
        `${u.first_name} ${u.last_name} ${u.email} ${u.role?.name ?? ''}`,
      );
      return haystack.includes(needle);
    });
  }, [users, deferredSearch]);

  const sorted = useMemo(
    () => [...filtered].sort((a, b) => compareUsers(a, b, sortKey, sortDir)),
    [filtered, sortKey, sortDir],
  );

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  }

  const showSearch = !isLoading && !isUpstreamAuthFailed(error) && (users?.length ?? 0) > 0;

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div>
          <h1 className={styles.pageTitle}>Utenti</h1>
          <p className={styles.pageSubtitle}>Gestisci gli utenti interni</p>
        </div>
        <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
          Nuovo utente
        </button>
      </div>

      {showSearch && (
        <div className={styles.searchBar}>
          <span className={styles.searchIcon} aria-hidden="true">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <circle cx="7" cy="7" r="4.5" stroke="currentColor" strokeWidth="1.5" />
              <path d="M11 11l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </span>
          <input
            type="search"
            className={styles.searchInput}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cerca per nome, cognome, email, ruolo…"
            aria-label="Cerca utenti"
          />
        </div>
      )}

      <div className={styles.tableCard}>
        {isLoading ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={6} />
          </div>
        ) : isUpstreamAuthFailed(error) ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
            <p className={styles.emptyText}>L&apos;elenco degli utenti non puo essere caricato ora.</p>
          </div>
        ) : error ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Impossibile caricare gli utenti</p>
            <p className={styles.emptyText}>Riprova fra qualche istante.</p>
          </div>
        ) : !users || users.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                <circle cx="24" cy="18" r="8" stroke="currentColor" strokeWidth="2" />
                <path d="M8 40c0-7 7-12 16-12s16 5 16 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessun utente trovato</p>
            <p className={styles.emptyText}>Crea il primo utente per iniziare</p>
          </div>
        ) : (
          <div className={styles.tableScroll}>
            <table className={styles.usersTable}>
              <thead>
                <tr>
                  <th className={styles.colAvatar} aria-label="Avatar" />
                  <th className={styles.colName} aria-sort={ariaSort(sortKey === 'first_name', sortDir)}>
                    <SortHeader sortKey="first_name" label="Nome" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colSurname} aria-sort={ariaSort(sortKey === 'last_name', sortDir)}>
                    <SortHeader sortKey="last_name" label="Cognome" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colEmail} aria-sort={ariaSort(sortKey === 'email', sortDir)}>
                    <SortHeader sortKey="email" label="Email" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colRole} aria-sort={ariaSort(sortKey === 'role', sortDir)}>
                    <SortHeader sortKey="role" label="Ruolo" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colStatus} aria-sort={ariaSort(sortKey === 'state', sortDir)}>
                    <SortHeader sortKey="state" label="Stato" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colUpdated} aria-sort={ariaSort(sortKey === 'updated', sortDir)}>
                    <SortHeader sortKey="updated" label="Aggiornato" active={sortKey} direction={sortDir} onSort={handleSort} />
                  </th>
                  <th className={styles.colActions} aria-label="Azioni" />
                </tr>
              </thead>
              <tbody>
                {sorted.length === 0 ? (
                  <tr>
                    <td colSpan={8}>
                      <div className={styles.searchEmpty}>
                        Nessun utente corrisponde alla ricerca
                        <button
                          type="button"
                          className={styles.linkButton}
                          onClick={() => setSearch('')}
                        >
                          Pulisci ricerca
                        </button>
                      </div>
                    </td>
                  </tr>
                ) : (
                  sorted.map((u, i) => {
                    const active = isUserActive(u);
                    return (
                    <tr
                      key={u.id}
                      className={!active ? styles.rowDisabled : ''}
                      onDoubleClick={() => setEditTarget(u)}
                      style={{ animationDelay: `${Math.min(i * 14, 240)}ms` }}
                    >
                      <td className={styles.avatarCell}>
                        <div className={styles.avatar}>{initials(u)}</div>
                      </td>
                      <td className={styles.nameCell}>{u.first_name}</td>
                      <td className={styles.nameCell}>{u.last_name}</td>
                      <td className={styles.emailCell} title={u.email}>{u.email}</td>
                      <td>{u.role?.name ?? '—'}</td>
                      <td>
                        <span className={`${styles.statusBadge} ${active ? styles.statusEnabled : styles.statusDisabled}`}>
                          <span className={styles.statusDot} />
                          {active ? 'Attivo' : 'Disabilitato'}
                        </span>
                      </td>
                      <td className={styles.updatedCell}>
                        {u.updated ? DATE_FMT.format(new Date(u.updated)) : '—'}
                      </td>
                      <td className={styles.actionsCell}>
                        <button
                          type="button"
                          className={styles.btnSecondary}
                          onClick={() => setEditTarget(u)}
                          aria-label={`Modifica ${u.first_name} ${u.last_name}`}
                        >
                          Modifica
                        </button>
                        <button
                          type="button"
                          className={styles.btnDanger}
                          disabled={!active}
                          onClick={() => setDisableTarget(u)}
                          aria-label={`Disattiva ${u.first_name} ${u.last_name}`}
                        >
                          Disattiva
                        </button>
                      </td>
                    </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <UserCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
      {editTarget && (
        <UserEditModal
          open={!!editTarget}
          onClose={() => setEditTarget(null)}
          user={editTarget}
        />
      )}
      {disableTarget && (
        <UserDisableConfirm
          open={!!disableTarget}
          onClose={() => setDisableTarget(null)}
          user={disableTarget}
        />
      )}
    </div>
  );
}
