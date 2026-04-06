import { useState } from 'react';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useGroups, useGroupDetails } from './queries';
import { Skeleton } from '@mrsmith/ui';
import { GroupCreateModal } from './GroupCreateModal';
import { GroupEditModal } from './GroupEditModal';
import { GroupDeleteConfirm } from './GroupDeleteConfirm';
import styles from './GruppiPage.module.css';

export function GruppiPage() {
  const [selectedGroupName, setSelectedGroupName] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  const { data: groups, isLoading: groupsLoading, error: groupsError } = useGroups();
  const { data: details, isLoading: detailsLoading, error: detailsError } = useGroupDetails(selectedGroupName);

  return (
    <div className={styles.page}>
      {/* Master: group list */}
      <div className={styles.master}>
        <div className={styles.toolbar}>
          <div>
            <h1 className={styles.pageTitle}>Gruppi</h1>
            <p className={styles.pageSubtitle}>Gestisci i gruppi e i loro membri</p>
          </div>
          <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
            Nuovo gruppo
          </button>
        </div>

        <div className={styles.tableCard}>
          {groupsLoading ? (
            <div className={styles.tableBody}>
              <Skeleton rows={5} />
            </div>
          ) : isUpstreamAuthFailed(groupsError) ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
              <p className={styles.emptyText}>L&apos;elenco gruppi non puo essere caricato in questo momento.</p>
            </div>
          ) : !groups || groups.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                  <rect x="4" y="8" width="40" height="32" rx="4" stroke="currentColor" strokeWidth="2" />
                  <path d="M4 16h40" stroke="currentColor" strokeWidth="2" />
                  <circle cx="24" cy="30" r="4" stroke="currentColor" strokeWidth="2" />
                  <path d="M20 34c0-2.2 1.8-4 4-4s4 1.8 4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Nessun gruppo trovato</p>
              <p className={styles.emptyText}>Crea il tuo primo gruppo per iniziare</p>
            </div>
          ) : (
            <>
              <div className={styles.tableHeader}>
                <span className={styles.colName}>Nome</span>
                <span className={styles.colCount}>Utenti</span>
              </div>
              <div className={styles.tableBody}>
                {groups.map((g, i) => (
                  <div
                    key={g.name}
                    className={`${styles.row} ${selectedGroupName === g.name ? styles.rowSelected : ''}`}
                    onClick={() => setSelectedGroupName(g.name)}
                    style={{ animationDelay: `${i * 50}ms` }}
                  >
                    <div className={styles.rowAccent} />
                    <div className={styles.rowIcon}>
                      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                        <circle cx="7" cy="6" r="2.5" stroke="currentColor" strokeWidth="1.5" />
                        <path d="M2.5 14.5c0-2.5 2-4 4.5-4s4.5 1.5 4.5 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                        <circle cx="13" cy="7" r="2" stroke="currentColor" strokeWidth="1.5" />
                        <path d="M15.5 14.5c0-2 1.5-3.2-1-3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                      </svg>
                    </div>
                    <span className={styles.rowName}>{g.name}</span>
                    <span className={styles.rowCount}>
                      <span className={styles.badge}>{g.user_count}</span>
                    </span>
                    <svg className={styles.rowChevron} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                      <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Detail: side panel */}
      <div className={`${styles.detail} ${selectedGroupName ? styles.detailOpen : ''}`}>
        {!selectedGroupName ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg width="40" height="40" viewBox="0 0 40 40" fill="none" aria-hidden="true">
                <path d="M8 12l12 8-12 8V12z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
                <path d="M24 16h8M24 20h12M24 24h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Seleziona un gruppo</p>
            <p className={styles.emptyText}>Scegli un gruppo dalla lista per vederne i dettagli</p>
          </div>
        ) : detailsLoading ? (
          <Skeleton rows={4} />
        ) : isUpstreamAuthFailed(detailsError) ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Dettaglio non disponibile</p>
            <p className={styles.emptyText}>I dettagli del gruppo non sono al momento raggiungibili.</p>
          </div>
        ) : details ? (
          <div className={styles.detailContent}>
            <div className={styles.detailHeader}>
              <div className={styles.detailIconLg}>
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <circle cx="9" cy="8" r="3" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M3 19c0-3.3 2.7-5.5 6-5.5s6 2.2 6 5.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  <circle cx="17.5" cy="9.5" r="2.5" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M21 19c0-2.5-1.5-4-4-4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                </svg>
              </div>
              <div>
                <h2 className={styles.detailTitle}>{details.name}</h2>
                <p className={styles.detailMeta}>
                  {details.users.length} {details.users.length === 1 ? 'membro' : 'membri'}
                </p>
              </div>
            </div>

            <div className={styles.divider} />

            <div className={styles.membersSection}>
              <h3 className={styles.sectionLabel}>Membri</h3>
              {details.users.length === 0 ? (
                <p className={styles.muted}>Nessun membro assegnato</p>
              ) : (
                <div className={styles.memberList}>
                  {details.users.map((u) => (
                    <div key={u.id} className={styles.memberRow}>
                      <div className={styles.memberAvatar}>
                        {u.first_name[0]}{u.last_name[0]}
                      </div>
                      <div className={styles.memberInfo}>
                        <span className={styles.memberName}>
                          {u.first_name} {u.last_name}
                        </span>
                        <span className={styles.memberEmail}>{u.email}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className={styles.divider} />

            <div className={styles.detailActions}>
              <button className={styles.btnSecondary} onClick={() => setShowEdit(true)}>
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                  <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                </svg>
                Modifica
              </button>
              <button className={styles.btnDanger} onClick={() => setShowDelete(true)}>
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                  <path d="M2 4h10M5 4V2.5h4V4M3 4l.7 8.1c.1.8.7 1.4 1.5 1.4h3.6c.8 0 1.4-.6 1.5-1.4L11 4" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                Elimina
              </button>
            </div>
          </div>
        ) : null}
      </div>

      {/* Modals */}
      <GroupCreateModal open={showCreate} onClose={() => setShowCreate(false)} />

      {selectedGroupName && (
        <>
          <GroupEditModal
            open={showEdit}
            onClose={() => setShowEdit(false)}
            groupName={selectedGroupName}
            onRenamed={(newName) => setSelectedGroupName(newName)}
          />
          <GroupDeleteConfirm
            open={showDelete}
            onClose={() => setShowDelete(false)}
            groupName={selectedGroupName}
            onDeleted={() => setSelectedGroupName(null)}
          />
        </>
      )}
    </div>
  );
}
