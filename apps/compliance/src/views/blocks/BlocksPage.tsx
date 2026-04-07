import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useBlocks, useBlockDomains, useAddBlockDomains, useUpdateBlockDomain } from '../../api/queries';
import { BlocksTable } from './BlocksTable';
import { BlockDetail } from './BlockDetail';
import { BlockCreateModal } from './BlockCreateModal';
import { BlockEditModal } from './BlockEditModal';
import { AddDomainsModal } from '../../components/AddDomainsModal';
import { DomainEditModal } from '../../components/DomainEditModal';
import styles from './BlocksPage.module.css';

export function BlocksPage() {
  const [selectedBlockId, setSelectedBlockId] = useState<number | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [showAddDomains, setShowAddDomains] = useState(false);
  const [editingDomain, setEditingDomain] = useState<{ id: number; domain: string } | null>(null);
  const { toast } = useToast();

  const { data: blocks, isLoading: blocksLoading, error: blocksError } = useBlocks();
  const { data: domains, isLoading: domainsLoading } = useBlockDomains(selectedBlockId);
  const addBlockDomains = useAddBlockDomains();
  const updateBlockDomain = useUpdateBlockDomain();

  const selectedBlock = blocks?.find((b) => b.id === selectedBlockId);

  return (
    <div className={styles.page}>
      {/* Master */}
      <div className={styles.master}>
        <div className={styles.toolbar}>
          <div>
            <h1 className={styles.pageTitle}>Blocchi</h1>
            <p className={styles.pageSubtitle}>Gestisci le richieste di blocco domini</p>
          </div>
          <button className={styles.btnPrimary} onClick={() => setShowCreate(true)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
            Nuova richiesta
          </button>
        </div>

        <div className={styles.tableCard}>
          {blocksLoading ? (
            <div className={styles.tableBody}><Skeleton rows={5} /></div>
          ) : isUpstreamAuthFailed(blocksError) ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
              <p className={styles.emptyText}>L&apos;elenco delle richieste di blocco non puo essere caricato in questo momento.</p>
            </div>
          ) : !blocks || blocks.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none" aria-hidden="true">
                  <rect x="6" y="6" width="36" height="36" rx="4" stroke="currentColor" strokeWidth="2" />
                  <path d="M16 16l16 16M32 16L16 32" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Nessuna richiesta di blocco</p>
              <p className={styles.emptyText}>Crea la tua prima richiesta di blocco per iniziare</p>
            </div>
          ) : (
            <BlocksTable blocks={blocks} selectedId={selectedBlockId} onSelect={setSelectedBlockId} />
          )}
        </div>
      </div>

      {/* Detail */}
      <div className={`${styles.detail} ${selectedBlockId ? styles.detailOpen : ''}`}>
        {!selectedBlockId ? (
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
        ) : !selectedBlock ? (
          <Skeleton rows={4} />
        ) : (
          <BlockDetail
            block={selectedBlock}
            domains={domains ?? []}
            domainsLoading={domainsLoading}
            onEdit={() => setShowEdit(true)}
            onAddDomains={() => setShowAddDomains(true)}
            onEditDomain={setEditingDomain}
          />
        )}
      </div>

      {/* Modals */}
      <BlockCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
      {selectedBlockId && (
        <>
          <BlockEditModal open={showEdit} onClose={() => setShowEdit(false)} blockId={selectedBlockId} />
          <AddDomainsModal
            open={showAddDomains}
            onClose={() => setShowAddDomains(false)}
            title="Aggiungi domini al blocco"
            onSubmit={(doms) =>
              addBlockDomains.mutate(
                { blockId: selectedBlockId, domains: doms },
                {
                  onSuccess: () => { toast('Domini aggiunti'); setShowAddDomains(false); },
                  onError: (error) => {
                    if (error instanceof ApiError) {
                      const body = error.body as { invalid?: string[] } | undefined;
                      if (body?.invalid) toast(`Alcuni domini non sono validi: ${body.invalid.join(', ')}`, 'error');
                      else toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
                    } else {
                      toast('Errore di connessione', 'error');
                    }
                  },
                },
              )
            }
            isPending={addBlockDomains.isPending}
          />
          <DomainEditModal
            open={!!editingDomain}
            onClose={() => setEditingDomain(null)}
            domain={editingDomain}
            onSave={(id, newDomain) =>
              updateBlockDomain.mutate(
                { blockId: selectedBlockId, domainId: id, domain: newDomain },
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
            isPending={updateBlockDomain.isPending}
          />
        </>
      )}
    </div>
  );
}
