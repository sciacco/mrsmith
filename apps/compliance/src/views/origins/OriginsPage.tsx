import { useState } from 'react';
import { Skeleton, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useOrigins, useEnableOrigin } from '../../api/queries';
import type { Origin } from '../../api/types';
import { OriginCreateModal } from './OriginCreateModal';
import { OriginEditModal } from './OriginEditModal';
import { DeactivateOriginConfirm } from '../../components/DeactivateOriginConfirm';
import styles from './OriginsPage.module.css';

export function OriginsPage() {
  const [showCreate, setShowCreate] = useState(false);
  const [editingOrigin, setEditingOrigin] = useState<Origin | null>(null);
  const [deactivatingOrigin, setDeactivatingOrigin] = useState<Origin | null>(null);
  const { toast } = useToast();

  const { data: origins, isLoading, error } = useOrigins(true);
  const enableOrigin = useEnableOrigin();

  function handleEnable(methodId: string) {
    enableOrigin.mutate(methodId, {
      onSuccess: () => toast('Provenienza abilitata'),
      onError: (err) => {
        if (err instanceof ApiError) {
          toast((err.body as { message?: string })?.message ?? err.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    });
  }

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div>
          <h1 className={styles.pageTitle}>Provenienze</h1>
          <p className={styles.pageSubtitle}>Gestisci le fonti delle richieste di blocco</p>
        </div>
        <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
          Nuova provenienza
        </button>
      </div>

      <div className={styles.tableCard}>
        {isLoading ? (
          <div className={styles.tableBody}><Skeleton rows={4} /></div>
        ) : isUpstreamAuthFailed(error) ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
          </div>
        ) : !origins || origins.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                <path d="M24 8l14 7v10c0 8-6 13-14 17-8-4-14-9-14-17V15l14-7z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessuna provenienza</p>
            <p className={styles.emptyText}>Crea la tua prima provenienza per iniziare</p>
          </div>
        ) : (
          <>
            <div className={styles.tableHeader}>
              <span>Codice</span>
              <span>Descrizione</span>
              <span>Stato</span>
              <span />
            </div>
            <div className={styles.tableBody}>
              {origins.map((origin, i) => (
                <div
                  key={origin.method_id}
                  className={`${styles.row} ${!origin.is_active ? styles.rowDisabled : ''}`}
                  style={{ animationDelay: `${i * 50}ms` }}
                >
                  <span className={styles.cellCode}>{origin.method_id}</span>
                  <span className={styles.cellDesc}>{origin.description}</span>
                  <span>
                    <span className={`${styles.statusBadge} ${origin.is_active ? styles.statusEnabled : styles.statusDisabled}`}>
                      <span className={styles.statusDot} />
                      {origin.is_active ? 'Attivo' : 'Disabilitato'}
                    </span>
                  </span>
                  <div className={styles.rowActions}>
                    <button
                      className={styles.actionBtn}
                      onClick={() => setEditingOrigin(origin)}
                      aria-label="Modifica"
                    >
                      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                        <path d="M10.5 1.5l2 2-8 8H2.5v-2l8-8z" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="round" />
                      </svg>
                    </button>
                    {origin.is_active ? (
                      <button
                        className={`${styles.actionBtn} ${styles.actionBtnDanger}`}
                        onClick={() => setDeactivatingOrigin(origin)}
                        aria-label="Disabilita"
                      >
                        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                          <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeWidth="1.25" />
                          <path d="M4 7h6" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" />
                        </svg>
                      </button>
                    ) : (
                      <button
                        className={`${styles.actionBtn} ${styles.actionBtnSuccess}`}
                        onClick={() => handleEnable(origin.method_id)}
                        disabled={enableOrigin.isPending}
                        aria-label="Abilita"
                      >
                        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                          <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeWidth="1.25" />
                          <path d="M5 7l1.5 1.5L9 5.5" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
                        </svg>
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      <OriginCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
      <OriginEditModal
        open={!!editingOrigin}
        onClose={() => setEditingOrigin(null)}
        origin={editingOrigin}
      />
      <DeactivateOriginConfirm
        open={!!deactivatingOrigin}
        onClose={() => setDeactivatingOrigin(null)}
        origin={deactivatingOrigin}
      />
    </div>
  );
}
