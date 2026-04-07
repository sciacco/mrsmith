import { useState } from 'react';
import { Skeleton, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useReleases, useReleaseDomains, useAddReleaseDomains, useUpdateReleaseDomain } from '../../api/queries';
import { ReleasesTable } from './ReleasesTable';
import { ReleaseDetail } from './ReleaseDetail';
import { ReleaseCreateModal } from './ReleaseCreateModal';
import { ReleaseEditModal } from './ReleaseEditModal';
import { AddDomainsModal } from '../../components/AddDomainsModal';
import { DomainEditModal } from '../../components/DomainEditModal';
import styles from './ReleasesPage.module.css';

export function ReleasesPage() {
  const [selectedReleaseId, setSelectedReleaseId] = useState<number | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [showAddDomains, setShowAddDomains] = useState(false);
  const [editingDomain, setEditingDomain] = useState<{ id: number; domain: string } | null>(null);
  const { toast } = useToast();

  const { data: releases, isLoading: releasesLoading, error: releasesError } = useReleases();
  const { data: domains, isLoading: domainsLoading } = useReleaseDomains(selectedReleaseId);
  const addReleaseDomains = useAddReleaseDomains();
  const updateReleaseDomain = useUpdateReleaseDomain();

  const selectedRelease = releases?.find((r) => r.id === selectedReleaseId);

  return (
    <div className={styles.page}>
      <div className={styles.master}>
        <div className={styles.toolbar}>
          <div>
            <h1 className={styles.pageTitle}>Rilasci</h1>
            <p className={styles.pageSubtitle}>Gestisci le richieste di rilascio domini</p>
          </div>
          <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
            Nuova richiesta
          </button>
        </div>

        <div className={styles.tableCard}>
          {releasesLoading ? (
            <div className={styles.tableBody}><Skeleton rows={5} /></div>
          ) : isUpstreamAuthFailed(releasesError) ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
              <p className={styles.emptyText}>L&apos;elenco delle richieste di rilascio non puo essere caricato in questo momento.</p>
            </div>
          ) : !releases || releases.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                  <rect x="6" y="6" width="36" height="36" rx="4" stroke="currentColor" strokeWidth="2" />
                  <path d="M16 24l6 6 10-10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Nessuna richiesta di rilascio</p>
              <p className={styles.emptyText}>Crea la tua prima richiesta di rilascio per iniziare</p>
            </div>
          ) : (
            <ReleasesTable releases={releases} selectedId={selectedReleaseId} onSelect={setSelectedReleaseId} />
          )}
        </div>
      </div>

      <div className={`${styles.detail} ${selectedReleaseId ? styles.detailOpen : ''}`}>
        {!selectedReleaseId ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg width="40" height="40" viewBox="0 0 40 40" fill="none" aria-hidden="true">
                <path d="M8 12l12 8-12 8V12z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
                <path d="M24 16h8M24 20h12M24 24h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Seleziona una richiesta</p>
            <p className={styles.emptyText}>Scegli una richiesta dalla lista per vederne i dettagli</p>
          </div>
        ) : !selectedRelease ? (
          <Skeleton rows={4} />
        ) : (
          <ReleaseDetail
            release={selectedRelease}
            domains={domains ?? []}
            domainsLoading={domainsLoading}
            onEdit={() => setShowEdit(true)}
            onAddDomains={() => setShowAddDomains(true)}
            onEditDomain={setEditingDomain}
          />
        )}
      </div>

      <ReleaseCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
      {selectedReleaseId && (
        <>
          <ReleaseEditModal open={showEdit} onClose={() => setShowEdit(false)} releaseId={selectedReleaseId} />
          <AddDomainsModal
            open={showAddDomains}
            onClose={() => setShowAddDomains(false)}
            title="Aggiungi domini al rilascio"
            onSubmit={(doms) =>
              addReleaseDomains.mutate(
                { releaseId: selectedReleaseId, domains: doms },
                {
                  onSuccess: () => { toast('Domini aggiunti'); setShowAddDomains(false); },
                  onError: (error) => {
                    if (error instanceof ApiError) {
                      toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
                    } else {
                      toast('Errore di connessione', 'error');
                    }
                  },
                },
              )
            }
            isPending={addReleaseDomains.isPending}
          />
          <DomainEditModal
            open={!!editingDomain}
            onClose={() => setEditingDomain(null)}
            domain={editingDomain}
            onSave={(id, newDomain) =>
              updateReleaseDomain.mutate(
                { releaseId: selectedReleaseId, domainId: id, domain: newDomain },
                {
                  onSuccess: () => { toast('Dominio aggiornato'); setEditingDomain(null); },
                  onError: (error) => {
                    if (error instanceof ApiError) {
                      toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
                    } else {
                      toast('Errore di connessione', 'error');
                    }
                  },
                },
              )
            }
            isPending={updateReleaseDomain.isPending}
          />
        </>
      )}
    </div>
  );
}
