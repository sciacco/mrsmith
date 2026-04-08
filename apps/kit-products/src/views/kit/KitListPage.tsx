import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCategories, useProducts } from '../../api/queries';
import { useCloneKit, useCreateKit, useDeleteKit, useKits } from './kitQueries';
import type { KitCreateRequest, KitCreateResponse, KitSummary } from './kitTypes';
import styles from './KitListPage.module.css';

const billingPeriodOptions = [
  { value: 1, label: 'Mensile' },
  { value: 2, label: 'Bimestrale' },
  { value: 3, label: 'Trimestrale' },
  { value: 4, label: 'Quadrimestrale' },
  { value: 6, label: 'Semestrale' },
  { value: 12, label: 'Annuale' },
  { value: 24, label: 'Biennale' },
];

const kitColumnDefs = [
  { key: 'internal_name', label: 'Kit' },
  { key: 'bundle_prefix', label: 'Prefix' },
  { key: 'nrc', label: 'NRC' },
  { key: 'mrc', label: 'MRC' },
  { key: 'category', label: 'Categoria' },
  { key: 'is_active', label: 'Attivo' },
  { key: 'billing_period', label: 'Periodo' },
  { key: 'sconto_massimo', label: 'Sconto max' },
  { key: 'main_product_code', label: 'Main product' },
  { key: 'ecommerce', label: 'Ecommerce' },
  { key: 'is_main_prd_sellable', label: 'Sellable' },
  { key: 'variable_billing', label: 'Variable billing' },
  { key: 'h24_assurance', label: 'H24' },
  { key: 'sla_resolution_hours', label: 'SLA ore' },
  { key: 'activation_time_days', label: 'Activation days' },
  { key: 'next_subscription_months', label: 'Next sub.' },
  { key: 'initial_subscription_months', label: 'Initial sub.' },
  { key: 'notes', label: 'Notes' },
] as const;

type KitColumnKey = (typeof kitColumnDefs)[number]['key'];

const defaultVisibleColumns: KitColumnKey[] = [
  'internal_name',
  'bundle_prefix',
  'nrc',
  'mrc',
  'category',
  'is_active',
  'billing_period',
  'sconto_massimo',
];

const kitToggleDefs = [
  { key: 'ecommerce', label: 'Ecommerce' },
] as const;

export function KitListPage() {
  const { toast } = useToast();
  const navigate = useNavigate();
  const { data: kits, isLoading, error, refetch } = useKits();
  const { data: categories } = useCategories();
  const { data: products } = useProducts();
  const createKit = useCreateKit();
  const cloneKit = useCloneKit();
  const deleteKit = useDeleteKit();

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [cloneOpen, setCloneOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [columnPickerOpen, setColumnPickerOpen] = useState(false);
  const [cloneSource, setCloneSource] = useState<KitSummary | null>(null);
  const [selectedColumns, setSelectedColumns] = useState<KitColumnKey[]>(defaultVisibleColumns);
  const [newKit, setNewKit] = useState<KitCreateRequest>({
    internal_name: '',
    main_product_code: null,
    category_id: 0,
    bundle_prefix: '',
    initial_subscription_months: 12,
    next_subscription_months: 12,
    activation_time_days: 30,
    nrc: 0,
    mrc: 0,
    ecommerce: true,
  });
  const [cloneName, setCloneName] = useState('');
  const [search, setSearch] = useState('');
  const [showInactive, setShowInactive] = useState(false);
  const [showEcommerce, setShowEcommerce] = useState(false);

  const sortedKits = useMemo(() => {
    const items = kits ?? [];
    return [...items].sort((a, b) => {
      if (a.is_active !== b.is_active) {
        return Number(b.is_active) - Number(a.is_active);
      }
      return a.internal_name.localeCompare(b.internal_name, 'it');
    });
  }, [kits]);

  const filteredKits = useMemo(() => {
    let items = sortedKits;
    if (!showInactive) {
      items = items.filter((kit) => kit.is_active);
    }
    if (!showEcommerce) {
      items = items.filter((kit) => !kit.ecommerce);
    }
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((kit) => {
      const categoryName = categories?.find((c) => c.id === kit.category_id)?.name ?? kit.category_name ?? '';
      return (
        kit.internal_name.toLowerCase().includes(q) ||
        (kit.bundle_prefix ?? '').toLowerCase().includes(q) ||
        (kit.main_product_code ?? '').toLowerCase().includes(q) ||
        categoryName.toLowerCase().includes(q) ||
        String(kit.id).includes(q)
      );
    });
  }, [sortedKits, search, categories, showInactive, showEcommerce]);

  const selectedKit = useMemo(
    () => sortedKits.find((kit) => kit.id === selectedId) ?? null,
    [selectedId, sortedKits],
  );

  const visibleColumns = useMemo(
    () => kitColumnDefs.filter((column) => selectedColumns.includes(column.key)),
    [selectedColumns],
  );

  async function handleCreateKit() {
    if (
      !newKit.internal_name.trim() ||
      !newKit.main_product_code ||
      newKit.category_id <= 0 ||
      !newKit.bundle_prefix.trim()
    ) {
      toast('Compila nome, categoria, bundle prefix e main product prima di creare il kit', 'error');
      return;
    }

    try {
      const result = await createKit.mutateAsync({
        ...newKit,
        internal_name: newKit.internal_name.trim(),
        main_product_code: newKit.main_product_code,
        bundle_prefix: newKit.bundle_prefix.trim(),
      });
      const createdId = extractKitId(result);
      setCreateOpen(false);
      setNewKit(resetKitDraft());
      if (createdId != null) {
        navigate(`/kit/${createdId}`);
      }
      toast('Kit creato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile creare il kit'), 'error');
    }
  }

  async function handleCloneKit() {
    if (!cloneSource || !cloneName.trim()) {
      toast('Inserisci un nome per il clone', 'error');
      return;
    }

    try {
      const result = await cloneKit.mutateAsync({ id: cloneSource.id, body: { name: cloneName.trim() } });
      const createdId = extractKitId(result);
      setCloneOpen(false);
      setCloneSource(null);
      setCloneName('');
      await refetch();
      if (createdId != null) {
        navigate(`/kit/${createdId}`);
      }
      toast('Kit clonato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile clonare il kit'), 'error');
    }
  }

  async function handleDeleteKit() {
    if (!selectedKit) {
      return;
    }

    try {
      await deleteKit.mutateAsync(selectedKit.id);
      setSelectedId(null);
      setDeleteOpen(false);
      toast(`Kit ${selectedKit.internal_name} disattivato`, 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile disattivare il kit'), 'error');
    }
  }

  const kitCount = sortedKits.length;
  const activeCount = sortedKits.filter((kit) => kit.is_active).length;

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={9} />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1>Kit e bundle</h1>
          <p className={styles.subtitle}>
            {activeCount} attivi su {kitCount} totali
          </p>
        </div>
        <button type="button" className={styles.primaryButton} onClick={() => setCreateOpen(true)}>
          Nuovo kit
        </button>
      </header>

      <section className={styles.card}>
        <div className={styles.cardToolbar}>
          <div className={styles.toolbarGroup}>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={!selectedKit}
              onClick={() => {
                if (!selectedKit) return;
                navigate(`/kit/${selectedKit.id}`);
              }}
            >
              Modifica
            </button>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={!selectedKit}
              onClick={() => {
                if (!selectedKit) return;
                setCloneSource(selectedKit);
                setCloneName(`${selectedKit.internal_name}-Copy`);
                setCloneOpen(true);
              }}
            >
              Clona
            </button>
            <button
              type="button"
              className={styles.dangerButton}
              disabled={!selectedKit}
              onClick={() => setDeleteOpen(true)}
            >
              Disattiva
            </button>
          </div>
          <div className={styles.searchWrap}>
            <svg className={styles.searchIcon} viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
              <path fillRule="evenodd" d="M9 3.5a5.5 5.5 0 1 0 0 11 5.5 5.5 0 0 0 0-11ZM2 9a7 7 0 1 1 12.452 4.391l3.328 3.329a.75.75 0 1 1-1.06 1.06l-3.329-3.328A7 7 0 0 1 2 9Z" clipRule="evenodd" />
            </svg>
            <input
              type="text"
              className={styles.searchInput}
              placeholder="Cerca kit..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className={styles.toolbarGroup}>
            <button
              type="button"
              className={`${styles.iconButton} ${showInactive ? styles.iconButtonActive : ''}`}
              onClick={() => setShowInactive((v) => !v)}
              aria-label={showInactive ? 'Nascondi inattivi' : 'Mostra inattivi'}
              title={showInactive ? 'Nascondi inattivi' : 'Mostra inattivi'}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="16" height="16">
                {showInactive ? (
                  <>
                    <path d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
                    <path d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                  </>
                ) : (
                  <path d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12c1.292 4.338 5.31 7.5 10.066 7.5.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
                )}
              </svg>
            </button>
            <button
              type="button"
              className={`${styles.iconButton} ${showEcommerce ? styles.iconButtonActive : ''}`}
              onClick={() => setShowEcommerce((v) => !v)}
              aria-label={showEcommerce ? 'Nascondi ecommerce' : 'Mostra ecommerce'}
              title={showEcommerce ? 'Nascondi ecommerce' : 'Mostra ecommerce'}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="16" height="16">
                <path d="M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0 0 0-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114 60.114 0 0 0-16.536-1.84M7.5 14.25 5.106 5.272M6 20.25a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm12.75 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
              </svg>
            </button>
            <button
              type="button"
              className={styles.iconButton}
              onClick={() => setColumnPickerOpen((current) => !current)}
              aria-label="Colonne"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
                <path d="M2 4.75A.75.75 0 0 1 2.75 4h14.5a.75.75 0 0 1 0 1.5H2.75A.75.75 0 0 1 2 4.75Zm0 10.5a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5a.75.75 0 0 1-.75-.75ZM2 10a.75.75 0 0 1 .75-.75h14.5a.75.75 0 0 1 0 1.5H2.75A.75.75 0 0 1 2 10Z" />
              </svg>
            </button>
          </div>
        </div>

        {columnPickerOpen ? (
          <div className={styles.columnPicker}>
            {kitColumnDefs.map((column) => {
              const checked = selectedColumns.includes(column.key);
              return (
                <label key={column.key} className={styles.checkboxPill}>
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={(event) => {
                      setSelectedColumns((current) => {
                        if (event.target.checked) {
                          return current.includes(column.key) ? current : [...current, column.key];
                        }
                        return current.filter((key) => key !== column.key);
                      });
                    }}
                  />
                  <span>{column.label}</span>
                </label>
              );
            })}
          </div>
        ) : null}
        {error ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIconDanger}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Impossibile caricare i kit</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : filteredKits.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0-3-3m3 3 3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>{search ? 'Nessun risultato' : 'Nessun kit disponibile'}</p>
            <p className={styles.emptyText}>{search ? `Nessun kit corrisponde a "${search}".` : 'Crea il primo kit per iniziare il catalogo bundle.'}</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.accentCell} />
                  {visibleColumns.map((column) => (
                    <th key={column.key}>{column.label}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filteredKits.map((kit, index) => {
                  const category = categories?.find((item) => item.id === kit.category_id);
                  return (
                    <tr
                      key={kit.id}
                      className={`${selectedId === kit.id ? styles.rowSelected : ''} ${!kit.is_active ? styles.rowInactive : ''}`}
                      style={{ animationDelay: `${index * 0.03}s` }}
                      onClick={() => setSelectedId(kit.id)}
                      onDoubleClick={() => navigate(`/kit/${kit.id}`)}
                    >
                      <td className={styles.accentCell}><div className={styles.accentBar} /></td>
                      {visibleColumns.map((column) => (
                        <td key={column.key}>{renderKitColumn(kit, column.key, category?.name ?? kit.category_name, category?.color ?? kit.category_color)}</td>
                      ))}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Nuovo Kit"
      >
        <div className={styles.modalBody}>
          <p className={styles.modalLead}>Compila i campi supportati dalla procedura di creazione del kit.</p>
          <div className={styles.formGrid}>
            <label className={styles.field}>
              <span>Internal name</span>
              <input
                value={newKit.internal_name}
                onChange={(event) => setNewKit((current) => ({ ...current, internal_name: event.target.value }))}
                placeholder="Kit Enterprise"
              />
            </label>
            <label className={styles.field}>
              <span>Main product</span>
              <select
                value={newKit.main_product_code ?? ''}
                onChange={(event) => setNewKit((current) => ({ ...current, main_product_code: event.target.value || null }))}
              >
                <option value="">Seleziona</option>
                {products?.map((product) => (
                  <option key={product.code} value={product.code}>
                    {product.code} - {product.internal_name}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span>Category</span>
              <select
                value={newKit.category_id}
                onChange={(event) => setNewKit((current) => ({ ...current, category_id: Number(event.target.value) }))}
              >
                <option value={0}>Seleziona</option>
                {categories?.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span>Bundle prefix</span>
              <input
                value={newKit.bundle_prefix ?? ''}
                onChange={(event) => setNewKit((current) => ({ ...current, bundle_prefix: event.target.value }))}
                placeholder="KIT-ENT"
              />
            </label>
            <label className={styles.field}>
              <span>Initial months</span>
              <input
                type="number"
                min={0}
                value={newKit.initial_subscription_months}
                onChange={(event) =>
                  setNewKit((current) => ({ ...current, initial_subscription_months: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>Next months</span>
              <input
                type="number"
                min={0}
                value={newKit.next_subscription_months}
                onChange={(event) =>
                  setNewKit((current) => ({ ...current, next_subscription_months: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>Activation days</span>
              <input
                type="number"
                min={0}
                value={newKit.activation_time_days}
                onChange={(event) =>
                  setNewKit((current) => ({ ...current, activation_time_days: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>NRC</span>
              <input
                type="number"
                step="0.01"
                value={newKit.nrc}
                onChange={(event) => setNewKit((current) => ({ ...current, nrc: Number(event.target.value) }))}
              />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input
                type="number"
                step="0.01"
                value={newKit.mrc}
                onChange={(event) => setNewKit((current) => ({ ...current, mrc: Number(event.target.value) }))}
              />
            </label>
          </div>
          <div className={styles.toggleGrid}>
            {kitToggleDefs.map((item) => (
              <label key={item.key} className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={newKit[item.key]}
                  onChange={(event) =>
                    setNewKit((current) => ({ ...current, [item.key]: event.target.checked }))
                  }
                />
                <span>{item.label}</span>
              </label>
            ))}
          </div>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCreateOpen(false)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.primaryButton}
              disabled={createKit.isPending}
              onClick={() => void handleCreateKit()}
            >
              Crea kit
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={cloneOpen}
        onClose={() => setCloneOpen(false)}
        title="Clona Kit"
      >
        <div className={styles.modalBody}>
          <p className={styles.modalLead}>
            Il clone preserva prodotti e custom value, ma non i gruppi cliente associati.
          </p>
          <label className={styles.field}>
            <span>Nuovo nome</span>
            <input
              value={cloneName}
              onChange={(event) => setCloneName(event.target.value)}
              placeholder={cloneSource ? `${cloneSource.internal_name}-Copy` : 'Kit-Copy'}
            />
          </label>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCloneOpen(false)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.primaryButton}
              disabled={cloneKit.isPending}
              onClick={() => void handleCloneKit()}
            >
              Clona
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="Disattiva kit"
      >
        <div className={styles.modalBody}>
          <p className={styles.modalLead}>
            Questo setta `is_active = false` per {selectedKit ? `KIT #${selectedKit.id}` : 'il kit selezionato'}.
          </p>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setDeleteOpen(false)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.dangerButton}
              disabled={deleteKit.isPending}
              onClick={() => void handleDeleteKit()}
            >
              Conferma
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function renderKitColumn(kit: KitSummary, key: KitColumnKey, categoryName: string, categoryColor: string) {
  switch (key) {
    case 'internal_name':
      return (
        <div className={styles.nameCell}>
          <div className={styles.nameLine}>
            <strong>{kit.internal_name}</strong>
            {kit.ecommerce ? (
              <svg className={styles.ecommerceIcon} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-label="Ecommerce" role="img">
                <title>Ecommerce</title>
                <path d="M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0 0 0-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114 60.114 0 0 0-16.536-1.84M7.5 14.25 5.106 5.272M6 20.25a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm12.75 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
              </svg>
            ) : null}
          </div>
          {kit.main_product_code ? <span>({kit.main_product_code})</span> : null}
        </div>
      );
    case 'bundle_prefix':
      return <code className={styles.mono}>{kit.bundle_prefix ?? 'n/d'}</code>;
    case 'nrc':
      return formatMoney(kit.nrc);
    case 'mrc':
      return formatMoney(kit.mrc);
    case 'category':
      return (
        <span className={styles.categoryChip} style={{ backgroundColor: categoryColor }}>
          {categoryName}
        </span>
      );
    case 'is_active':
      return <span className={kit.is_active ? styles.goodBadge : styles.badBadge}>{kit.is_active ? 'Si' : 'No'}</span>;
    case 'billing_period':
      return getBillingLabel(kit.billing_period);
    case 'sconto_massimo':
      return `${Number(kit.sconto_massimo).toFixed(2)}%`;
    case 'main_product_code':
      return kit.main_product_code ?? 'n/d';
    case 'ecommerce':
      return kit.ecommerce ? 'Si' : 'No';
    case 'is_main_prd_sellable':
      return kit.is_main_prd_sellable == null ? 'n/d' : kit.is_main_prd_sellable ? 'Si' : 'No';
    case 'variable_billing':
      return kit.variable_billing ? 'Si' : 'No';
    case 'h24_assurance':
      return kit.h24_assurance ? 'Si' : 'No';
    case 'sla_resolution_hours':
      return String(kit.sla_resolution_hours);
    case 'activation_time_days':
      return String(kit.activation_time_days);
    case 'next_subscription_months':
      return String(kit.next_subscription_months);
    case 'initial_subscription_months':
      return String(kit.initial_subscription_months);
    case 'notes':
      return kit.notes ?? 'n/d';
    default:
      return null;
  }
}

function getBillingLabel(value: number) {
  return billingPeriodOptions.find((option) => option.value === value)?.label ?? String(value);
}

function formatMoney(value: number) {
  return new Intl.NumberFormat('it-IT', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function extractKitId(value: KitCreateResponse | number | null | undefined) {
  if (typeof value === 'number') {
    return value;
  }
  if (value && typeof value === 'object') {
    if (typeof value.id === 'number') {
      return value.id;
    }
    if (typeof value.kit_id === 'number') {
      return value.kit_id;
    }
  }
  return null;
}

function resetKitDraft(): KitCreateRequest {
  return {
    internal_name: '',
    main_product_code: null,
    category_id: 0,
    bundle_prefix: '',
    initial_subscription_months: 12,
    next_subscription_months: 12,
    activation_time_days: 30,
    nrc: 0,
    mrc: 0,
    ecommerce: true,
  };
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') {
      return message;
    }
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}
