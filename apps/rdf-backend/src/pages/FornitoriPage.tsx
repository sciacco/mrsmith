import { ApiError } from '@mrsmith/api-client';
import { startTransition, useDeferredValue, useEffect, useMemo, useState, type FormEvent } from 'react';
import { Button, Icon, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import type { ErrorResponse, Supplier } from '../api/types';
import {
  useCreateSupplier,
  useDeleteSupplier,
  useSuppliers,
  useUpdateSupplier,
  type SortKey,
  type SortOrder,
} from './queries';
import styles from './FornitoriPage.module.css';

const pageSize = 20;

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    const body = error.body as ErrorResponse | undefined;
    switch (body?.error) {
      case 'nome_required':
        return 'Inserisci il nome del fornitore.';
      case 'not_found':
        return 'Il fornitore selezionato non e piu disponibile.';
      case 'anisetta_database_not_configured':
        return 'Il servizio fornitori non e disponibile.';
      default:
        if (error.status === 401) return 'La sessione non e valida. Ricarica la pagina.';
        if (error.status === 403) return 'Non hai i permessi per consultare i fornitori.';
      }
  }

  return fallback;
}

function SortButton({
  active,
  direction,
  label,
  onClick,
}: {
  active: boolean;
  direction: SortOrder;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`${styles.sortButton} ${active ? styles.sortButtonActive : ''}`}
      onClick={onClick}
    >
      <span>{label}</span>
      <Icon
        name={active && direction === 'desc' ? 'chevron-down' : 'chevron-up'}
        size={14}
      />
    </button>
  );
}

export function FornitoriPage() {
  const { toast } = useToast();

  const [searchInput, setSearchInput] = useState('');
  const deferredSearch = useDeferredValue(searchInput);
  const [sortKey, setSortKey] = useState<SortKey>('id');
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
  const [page, setPage] = useState(1);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [draftName, setDraftName] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Supplier | null>(null);

  const suppliersQuery = useSuppliers({
    search: deferredSearch.trim(),
    sort: sortKey,
    order: sortOrder,
    page,
    pageSize,
  });

  const createSupplier = useCreateSupplier();
  const updateSupplier = useUpdateSupplier();
  const deleteSupplier = useDeleteSupplier();

  const items = suppliersQuery.data?.items ?? [];
  const total = suppliersQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const selectedSupplier = useMemo(
    () => items.find((supplier) => supplier.id === selectedId) ?? null,
    [items, selectedId],
  );
  const updateDirty = selectedSupplier ? draftName.trim() !== selectedSupplier.nome.trim() : false;
  const pageStart = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const pageEnd = total === 0 ? 0 : pageStart + items.length - 1;

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  useEffect(() => {
    if (!selectedId) {
      setDraftName('');
      return;
    }
    if (!selectedSupplier) {
      setSelectedId(null);
      setDraftName('');
      return;
    }
    setDraftName(selectedSupplier.nome);
  }, [selectedId, selectedSupplier]);

  function handleSearchChange(value: string) {
    startTransition(() => {
      setSearchInput(value);
      setPage(1);
    });
  }

  function handleSort(column: SortKey) {
    startTransition(() => {
      setPage(1);
      if (sortKey === column) {
        setSortOrder((current) => (current === 'asc' ? 'desc' : 'asc'));
        return;
      }
      setSortKey(column);
      setSortOrder('asc');
    });
  }

  async function handleCreateSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      await createSupplier.mutateAsync({ nome: createName.trim() });
      setCreateName('');
      setCreateOpen(false);
    } catch (error) {
      toast(getErrorMessage(error, 'Impossibile creare il fornitore.'), 'error');
    }
  }

  async function handleUpdateSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedSupplier) return;
    try {
      await updateSupplier.mutateAsync({
        id: selectedSupplier.id,
        body: { nome: draftName.trim() },
      });
    } catch (error) {
      toast(getErrorMessage(error, 'Impossibile aggiornare il fornitore.'), 'error');
    }
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget) return;
    try {
      await deleteSupplier.mutateAsync(deleteTarget.id);
      if (selectedId === deleteTarget.id) {
        setSelectedId(null);
      }
      setDeleteTarget(null);
    } catch (error) {
      toast(getErrorMessage(error, 'Impossibile eliminare il fornitore.'), 'error');
    }
  }

  return (
    <div className={styles.page}>
      <section className={styles.master}>
        <div className={styles.toolbar}>
          <div>
            <h1 className={styles.pageTitle}>Fornitori</h1>
            <p className={styles.pageSubtitle}>Gestisci l&apos;elenco dei fornitori disponibili.</p>
          </div>
          <div className={styles.toolbarActions}>
            <Button
              variant="secondary"
              onClick={() => suppliersQuery.refetch()}
              leftIcon={<Icon name="arrow-right" size={16} />}
              loading={suppliersQuery.isFetching}
            >
              Aggiorna elenco
            </Button>
            <Button
              onClick={() => setCreateOpen(true)}
              leftIcon={<Icon name="plus" size={16} />}
            >
              Nuovo fornitore
            </Button>
          </div>
        </div>

        <div className={styles.tableCard}>
          <div className={styles.tableTools}>
            <SearchInput
              value={searchInput}
              onChange={handleSearchChange}
              placeholder="Cerca per nome"
              className={styles.search}
            />
          </div>

          {suppliersQuery.isLoading ? (
            <div className={styles.tableLoading}>
              <Skeleton rows={6} />
            </div>
          ) : suppliersQuery.error ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <Icon name="triangle-alert" size={20} />
              </div>
              <h2 className={styles.emptyTitle}>Elenco non disponibile</h2>
              <p className={styles.emptyText}>
                {getErrorMessage(suppliersQuery.error, 'Impossibile caricare i fornitori.')}
              </p>
            </div>
          ) : items.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <Icon name="server" size={20} />
              </div>
              <h2 className={styles.emptyTitle}>Nessun fornitore trovato</h2>
              <p className={styles.emptyText}>
                Prova a cambiare ricerca oppure aggiungi un nuovo fornitore.
              </p>
            </div>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>
                      <SortButton
                        active={sortKey === 'id'}
                        direction={sortOrder}
                        label="ID"
                        onClick={() => handleSort('id')}
                      />
                    </th>
                    <th>
                      <SortButton
                        active={sortKey === 'nome'}
                        direction={sortOrder}
                        label="Nome"
                        onClick={() => handleSort('nome')}
                      />
                    </th>
                    <th className={styles.actionsHeader}>Azioni</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((supplier, index) => (
                    <tr
                      key={supplier.id}
                      className={selectedId === supplier.id ? styles.rowSelected : ''}
                      onClick={() => setSelectedId(supplier.id)}
                      style={{ animationDelay: `${index * 45}ms` }}
                    >
                      <td className={styles.idCell}>
                        <span className={styles.rowId}>#{supplier.id}</span>
                      </td>
                      <td>
                        <span className={styles.rowName}>{supplier.nome}</span>
                      </td>
                      <td className={styles.rowActions}>
                        <button
                          type="button"
                          className={styles.deleteButton}
                          onClick={(event) => {
                            event.stopPropagation();
                            setDeleteTarget(supplier);
                          }}
                        >
                          <Icon name="trash" size={14} />
                          Elimina
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <footer className={styles.pagination}>
            <div className={styles.paginationMeta}>
              {pageStart === 0 ? '0 risultati' : `${pageStart}-${pageEnd} di ${total} fornitori`}
            </div>
            <div className={styles.paginationControls}>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setPage((current) => Math.max(1, current - 1))}
                disabled={page <= 1}
              >
                Precedente
              </Button>
              <span className={styles.pageBadge}>
                Pagina {page} di {totalPages}
              </span>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                disabled={page >= totalPages}
              >
                Successiva
              </Button>
            </div>
          </footer>
        </div>
      </section>

      <aside className={styles.detailPanel}>
        {!selectedSupplier ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <Icon name="pencil" size={22} />
            </div>
            <h2 className={styles.emptyTitle}>Seleziona un fornitore</h2>
            <p className={styles.emptyText}>
              Scegli un fornitore dall&apos;elenco per modificarne il nome.
            </p>
          </div>
        ) : (
          <form className={styles.detailCard} onSubmit={handleUpdateSubmit}>
            <div className={styles.detailHeader}>
              <div>
                <p className={styles.detailLabel}>Fornitore selezionato</p>
                <h2>{selectedSupplier.nome}</h2>
                <p className={styles.detailMeta}>ID #{selectedSupplier.id}</p>
              </div>
            </div>

            <p className={styles.detailDescription}>
              Aggiorna il nome del fornitore e salva le modifiche.
            </p>

            <div className={styles.field}>
              <label htmlFor="supplier-name">Nome</label>
              <input
                id="supplier-name"
                type="text"
                value={draftName}
                onChange={(event) => setDraftName(event.target.value)}
                placeholder="Nome fornitore"
              />
            </div>

            <div className={styles.detailActions}>
              <Button
                type="submit"
                loading={updateSupplier.isPending}
                disabled={!updateDirty || draftName.trim() === ''}
                fullWidth
              >
                Salva modifiche
              </Button>
              <Button
                variant="secondary"
                onClick={() => setSelectedId(null)}
                fullWidth
              >
                Deseleziona
              </Button>
            </div>
          </form>
        )}
      </aside>

      <Modal
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
          setCreateName('');
        }}
        title="Nuovo fornitore"
        dismissible={!createSupplier.isPending}
      >
        <form className={styles.modalForm} onSubmit={handleCreateSubmit}>
          <div className={styles.field}>
            <label htmlFor="new-supplier-name">Nome</label>
            <input
              id="new-supplier-name"
              type="text"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder="Inserisci il nome del fornitore"
              autoFocus
            />
          </div>
          <div className={styles.modalActions}>
            <Button
              variant="secondary"
              onClick={() => {
                setCreateOpen(false);
                setCreateName('');
              }}
              disabled={createSupplier.isPending}
            >
              Annulla
            </Button>
            <Button type="submit" loading={createSupplier.isPending} disabled={createName.trim() === ''}>
              Crea
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={deleteTarget !== null}
        onClose={() => {
          setDeleteTarget(null);
        }}
        title="Elimina fornitore"
        dismissible={!deleteSupplier.isPending}
      >
        <div className={styles.deletePrompt}>
          <p>Vuoi eliminare questo fornitore? L&apos;operazione non puo essere annullata.</p>
          {deleteTarget && (
            <div className={styles.deleteCard}>
              <strong>{deleteTarget.nome}</strong>
              <span className={styles.deleteMeta}>ID #{deleteTarget.id}</span>
            </div>
          )}
          <div className={styles.modalActions}>
            <Button
              variant="secondary"
              onClick={() => setDeleteTarget(null)}
              disabled={deleteSupplier.isPending}
            >
              Annulla
            </Button>
            <Button
              variant="danger"
              onClick={handleDeleteConfirm}
              loading={deleteSupplier.isPending}
            >
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
