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
        return 'Il nome del fornitore e obbligatorio.';
      case 'not_found':
        return 'Il fornitore selezionato non e piu disponibile.';
      case 'anisetta_database_not_configured':
        return 'La connessione ad Anisetta non e configurata.';
      default:
        if (error.status === 401) return 'La sessione non e valida. Ricarica la pagina.';
        if (error.status === 403) return 'Non hai i permessi per usare RDF Backend.';
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
      <section className={styles.hero}>
        <div className={styles.heroContent}>
          <div>
            <p className={styles.eyebrow}>Provisioning</p>
            <h1>RDF Backend</h1>
            <p className={styles.heroLead}>
              Registro fornitori su Anisetta con ricerca, ordinamento e paginazione lato server.
            </p>
          </div>
          <div className={styles.heroStats}>
            <div className={styles.statCard}>
              <span className={styles.statLabel}>Totale fornitori</span>
              <strong className={styles.statValue}>{total}</strong>
            </div>
            <div className={styles.statCard}>
              <span className={styles.statLabel}>Pagina corrente</span>
              <strong className={styles.statValue}>
                {pageStart === 0 ? '0' : `${pageStart}-${pageEnd}`}
              </strong>
            </div>
            <div className={styles.statCard}>
              <span className={styles.statLabel}>Ordinamento</span>
              <strong className={styles.statValueMono}>
                {sortKey}.{sortOrder}
              </strong>
            </div>
          </div>
        </div>
        <div className={styles.heroGlow} />
      </section>

      <div className={styles.workspace}>
        <section className={styles.registryPanel}>
          <header className={styles.panelHeader}>
            <div>
              <h2>Fornitori</h2>
              <p>Seleziona una riga per l&apos;aggiornamento inline oppure crea un nuovo record.</p>
            </div>
            <div className={styles.liveBadge}>
              <span className={`${styles.liveDot} ${suppliersQuery.isFetching ? styles.liveDotBusy : ''}`} />
              {suppliersQuery.isFetching ? 'Sincronizzazione' : 'Allineato'}
            </div>
          </header>

          <div className={styles.toolbar}>
            <SearchInput
              value={searchInput}
              onChange={handleSearchChange}
              placeholder="Cerca fornitore..."
              className={styles.search}
            />
            <div className={styles.toolbarActions}>
              <Button
                variant="ghost"
                onClick={() => suppliersQuery.refetch()}
                leftIcon={<Icon name="arrow-right" size={16} />}
                loading={suppliersQuery.isFetching}
              >
                Refresh
              </Button>
              <Button
                onClick={() => setCreateOpen(true)}
                leftIcon={<Icon name="plus" size={16} />}
              >
                Nuovo fornitore
              </Button>
            </div>
          </div>

          <div className={styles.tableShell}>
            <div className={styles.tableHeader}>
              <SortButton
                active={sortKey === 'id'}
                direction={sortOrder}
                label="ID"
                onClick={() => handleSort('id')}
              />
              <SortButton
                active={sortKey === 'nome'}
                direction={sortOrder}
                label="Nome"
                onClick={() => handleSort('nome')}
              />
              <span className={styles.headerAction}>Azione</span>
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
                <h3>Elenco non disponibile</h3>
                <p>{getErrorMessage(suppliersQuery.error, 'Impossibile caricare i fornitori.')}</p>
              </div>
            ) : items.length === 0 ? (
              <div className={styles.emptyState}>
                <div className={styles.emptyIcon}>
                  <Icon name="server" size={20} />
                </div>
                <h3>Nessun fornitore trovato</h3>
                <p>Affina la ricerca oppure crea un nuovo record nel registro RDF.</p>
              </div>
            ) : (
              <div className={styles.tableBody}>
                <table className={styles.table}>
                  <tbody>
                    {items.map((supplier, index) => (
                      <tr
                        key={supplier.id}
                        className={selectedId === supplier.id ? styles.rowSelected : ''}
                        onClick={() => setSelectedId(supplier.id)}
                        style={{ animationDelay: `${index * 45}ms` }}
                      >
                        <td>
                          <span className={styles.idPill}>#{supplier.id}</span>
                        </td>
                        <td>
                          <div className={styles.nameCell}>
                            <span className={styles.name}>{supplier.nome}</span>
                            {selectedId === supplier.id && (
                              <span className={styles.selectedTag}>selected</span>
                            )}
                          </div>
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
                            Delete
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
                  Pagina {page} / {totalPages}
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
            <div className={styles.detailEmpty}>
              <div className={styles.detailIcon}>
                <Icon name="pencil" size={22} />
              </div>
              <h3>Seleziona una riga</h3>
              <p>Il pannello laterale riproduce l&apos;update inline dell&apos;app originale senza aprire modali.</p>
            </div>
          ) : (
            <form className={styles.detailCard} onSubmit={handleUpdateSubmit}>
              <div className={styles.detailHeader}>
                <div>
                  <p className={styles.detailLabel}>Record attivo</p>
                  <h3>{selectedSupplier.nome}</h3>
                </div>
                <span className={styles.detailId}>#{selectedSupplier.id}</span>
              </div>

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

              <div className={styles.changeRow}>
                <span className={updateDirty ? styles.dirtyTag : styles.cleanTag}>
                  {updateDirty ? 'Modifiche pronte' : 'Nessuna modifica'}
                </span>
              </div>

              <div className={styles.detailActions}>
                <Button
                  type="submit"
                  loading={updateSupplier.isPending}
                  disabled={!updateDirty || draftName.trim() === ''}
                  fullWidth
                >
                  Aggiorna fornitore
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => setSelectedId(null)}
                  fullWidth
                >
                  Chiudi selezione
                </Button>
              </div>
            </form>
          )}
        </aside>
      </div>

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
          <p>
            Are you sure you want to delete this item?
          </p>
          {deleteTarget && (
            <div className={styles.deleteCard}>
              <span className={styles.idPill}>#{deleteTarget.id}</span>
              <strong>{deleteTarget.nome}</strong>
            </div>
          )}
          <div className={styles.modalActions}>
            <Button
              variant="secondary"
              onClick={() => setDeleteTarget(null)}
              disabled={deleteSupplier.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={handleDeleteConfirm}
              loading={deleteSupplier.isPending}
            >
              Confirm
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
