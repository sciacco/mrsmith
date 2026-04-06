import { useState } from 'react';
import {
  useCostCenters,
  useCostCenterDetails,
  useEnableCostCenter,
} from './queries';
import { useToast, Skeleton } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { CostCenterCreateModal } from './CostCenterCreateModal';
import { CostCenterEditModal } from './CostCenterEditModal';
import { CostCenterDisableConfirm } from './CostCenterDisableConfirm';
import styles from './CentriDiCostoPage.module.css';

export function CentriDiCostoPage() {
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [showDisable, setShowDisable] = useState(false);

  const { data: costCenters, isLoading: listLoading } = useCostCenters();
  const { data: details, isLoading: detailsLoading } = useCostCenterDetails(selectedName);
  const enableCC = useEnableCostCenter();
  const { toast } = useToast();

  function handleEnable() {
    if (!selectedName) return;
    enableCC.mutate(selectedName, {
      onSuccess: (res) => toast(res.message),
      onError: (error) => {
        if (error instanceof ApiError) {
          toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    });
  }

  const detailUsers = details?.users ?? [];

  return (
    <div className={styles.page}>
      {/* Master: cost center list */}
      <div className={styles.master}>
        <div className={styles.toolbar}>
          <div>
            <h1 className={styles.pageTitle}>Centri di costo</h1>
            <p className={styles.pageSubtitle}>Gestisci i centri di costo e i loro membri</p>
          </div>
          <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
            Nuovo centro di costo
          </button>
        </div>

        <div className={styles.tableCard}>
          {listLoading ? (
            <div className={styles.tableBody}>
              <Skeleton rows={5} />
            </div>
          ) : !costCenters || costCenters.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                  <rect x="6" y="10" width="36" height="28" rx="4" stroke="currentColor" strokeWidth="2" />
                  <path d="M6 18h36" stroke="currentColor" strokeWidth="2" />
                  <path d="M18 10V6M30 10V6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                  <circle cx="24" cy="30" r="4" stroke="currentColor" strokeWidth="2" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Nessun centro di costo trovato</p>
              <p className={styles.emptyText}>Crea il tuo primo centro di costo per iniziare</p>
            </div>
          ) : (
            <>
              <div className={styles.tableHeader}>
                <span>Nome</span>
                <span className={styles.colStatus}>Stato</span>
                <span className={styles.colManager}>Manager</span>
                <span className={styles.colCount}>Membri</span>
                <span className={styles.colCount}>Gruppi</span>
                <span />
              </div>
              <div className={styles.tableBody}>
                {costCenters.map((cc, i) => (
                  <div
                    key={cc.name}
                    className={`${styles.row} ${selectedName === cc.name ? styles.rowSelected : ''} ${!cc.enabled ? styles.rowDisabled : ''}`}
                    onClick={() => setSelectedName(cc.name)}
                    style={{ animationDelay: `${i * 50}ms` }}
                  >
                    <div className={styles.rowAccent} />
                    <div className={styles.rowIcon}>
                      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                        <rect x="2" y="3" width="14" height="12" rx="2" stroke="currentColor" strokeWidth="1.5" />
                        <path d="M2 7h14" stroke="currentColor" strokeWidth="1.5" />
                        <path d="M6 3V1M12 3V1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                      </svg>
                    </div>
                    <span className={styles.rowName}>{cc.name}</span>
                    <span className={styles.rowCount}>
                      <span className={`${styles.statusBadge} ${cc.enabled ? styles.statusEnabled : styles.statusDisabled}`}>
                        <span className={styles.statusDot} />
                      </span>
                    </span>
                    <span className={styles.rowManager}>{cc.manager_email}</span>
                    <span className={styles.rowCount}>
                      <span className={styles.badge}>{cc.user_count}</span>
                    </span>
                    <span className={styles.rowCount}>
                      <span className={styles.badge}>{cc.group_count} <span className={styles.badgeSub}>({cc.group_user_count})</span></span>
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

      {/* Detail: side column */}
      <div className={styles.sidebar}>
        <div className={`${styles.detail} ${selectedName ? styles.detailOpen : ''}`}>
          {!selectedName ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" aria-hidden="true">
                  <rect x="6" y="8" width="28" height="24" rx="3" stroke="currentColor" strokeWidth="2" />
                  <path d="M6 14h28" stroke="currentColor" strokeWidth="2" />
                  <path d="M14 8V4M26 8V4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Seleziona un centro di costo</p>
              <p className={styles.emptyText}>Scegli un centro di costo dalla lista per vederne i dettagli</p>
            </div>
          ) : detailsLoading ? (
            <Skeleton rows={4} />
          ) : details ? (
            <div className={styles.detailContent}>
              <div className={styles.detailHeader}>
                <div className={styles.detailIconLg}>
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" />
                    <path d="M3 9h18" stroke="currentColor" strokeWidth="1.5" />
                    <path d="M8 5V3M16 5V3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </div>
                <div>
                  <h2 className={styles.detailTitle}>{details.name}</h2>
                  <div className={styles.detailMeta}>
                    <span className={`${styles.statusBadge} ${details.enabled ? styles.statusEnabled : styles.statusDisabled}`}>
                      <span className={styles.statusDot} />
                      {details.enabled ? 'Attivo' : 'Disabilitato'}
                    </span>
                    <span>{detailUsers.length} {detailUsers.length === 1 ? 'membro' : 'membri'}</span>
                  </div>
                </div>
              </div>

              <div className={styles.divider} />

              {/* Manager info */}
              <div className={styles.detailInfo}>
                <div className={styles.infoRow}>
                  <div className={styles.infoIcon}>
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                      <circle cx="8" cy="5" r="3" stroke="currentColor" strokeWidth="1.25" />
                      <path d="M2 14c0-3 2.5-5 6-5s6 2 6 5" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                    </svg>
                  </div>
                  <div className={styles.infoContent}>
                    <span className={styles.infoLabel}>Manager</span>
                    <span className={styles.infoValue}>
                      {details.manager.first_name} {details.manager.last_name}
                    </span>
                  </div>
                </div>
              </div>

              <div className={styles.divider} />

              {/* Members */}
              <div className={styles.membersSection}>
                <h3 className={styles.sectionLabel}>Membri</h3>
                {detailUsers.length === 0 ? (
                  <p className={styles.muted}>Nessun membro assegnato</p>
                ) : (
                  <div className={styles.memberList}>
                    {detailUsers.map((u) => (
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

              {/* Groups */}
              {details.groups && details.groups.length > 0 && (
                <>
                  <div className={styles.divider} />
                  <div className={styles.membersSection}>
                    <h3 className={styles.sectionLabel}>Gruppi</h3>
                    <div className={styles.groupChips}>
                      {details.groups.map((g) => (
                        <span key={g.name} className={styles.groupChip}>
                          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                            <circle cx="5" cy="4" r="1.5" stroke="currentColor" strokeWidth="1" />
                            <path d="M2 9c0-1.5 1.2-2.5 3-2.5s3 1 3 2.5" stroke="currentColor" strokeWidth="1" strokeLinecap="round" />
                            <circle cx="9" cy="5" r="1.2" stroke="currentColor" strokeWidth="1" />
                            <path d="M10.5 9c0-1 -.8-1.8-1.5-2" stroke="currentColor" strokeWidth="1" strokeLinecap="round" />
                          </svg>
                          {g.name}
                        </span>
                      ))}
                    </div>
                  </div>
                </>
              )}

              <div className={styles.divider} />

              <div className={styles.detailActions}>
                <button className={styles.btnSecondary} onClick={() => setShowEdit(true)}>
                  <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                    <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                  </svg>
                  Modifica
                </button>
                {details.enabled ? (
                  <button className={styles.btnDanger} onClick={() => setShowDisable(true)}>
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                      <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeWidth="1.25" />
                      <path d="M4 7h6" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                    </svg>
                    Disabilita
                  </button>
                ) : (
                  <button
                    className={styles.btnSuccess}
                    onClick={handleEnable}
                    disabled={enableCC.isPending}
                  >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                      <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeWidth="1.25" />
                      <path d="M5 7l1.5 1.5L9 5.5" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                    {enableCC.isPending ? 'Abilitazione...' : 'Abilita'}
                  </button>
                )}
              </div>
            </div>
          ) : null}
        </div>

        {/* Group members list — below the detail card */}
        {details && details.groups && details.groups.length > 0 && (
          <div className={styles.groupMembersCard}>
            <h3 className={styles.groupMembersTitle}>Utenti per gruppo</h3>
            {details.groups.map((g) => (
              <div key={g.name} className={styles.groupBlock}>
                <div className={styles.groupBlockHeader}>
                  <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                    <circle cx="5" cy="5" r="2" stroke="currentColor" strokeWidth="1.25" />
                    <path d="M1.5 12c0-2 1.5-3.5 3.5-3.5s3.5 1.5 3.5 3.5" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                    <circle cx="10.5" cy="6" r="1.5" stroke="currentColor" strokeWidth="1.25" />
                    <path d="M12.5 12c0-1.5-1-2.5-2-3" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                  </svg>
                  <span>{g.name}</span>
                  <span className={styles.groupBlockCount}>{g.users.length}</span>
                </div>
                <div className={styles.memberList}>
                  {g.users.map((u) => (
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
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Modals */}
      <CostCenterCreateModal open={showCreate} onClose={() => setShowCreate(false)} />

      {selectedName && (
        <>
          <CostCenterEditModal
            open={showEdit}
            onClose={() => setShowEdit(false)}
            costCenterName={selectedName}
            onRenamed={(newName) => setSelectedName(newName)}
          />
          <CostCenterDisableConfirm
            open={showDisable}
            onClose={() => setShowDisable(false)}
            costCenterName={selectedName}
          />
        </>
      )}
    </div>
  );
}
