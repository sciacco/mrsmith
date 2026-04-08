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

  const sortedKits = useMemo(() => {
    const items = kits ?? [];
    return [...items].sort((a, b) => {
      if (a.is_active !== b.is_active) {
        return Number(b.is_active) - Number(a.is_active);
      }
      return a.internal_name.localeCompare(b.internal_name, 'it');
    });
  }, [kits]);

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
  const selectedCategory = selectedKit
    ? categories?.find((category) => category.id === selectedKit.category_id)
    : null;

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={9} />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.hero}>
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow}>Phase 4</p>
          <h1>Kit e bundle</h1>
          <p className={styles.lead}>
            Catalogo operativo per creare, clonare e disattivare kit, con colonna attiva in testa e
            categorie colorate come nel contratto Mistra.
          </p>
        </div>
        <div className={styles.heroStats}>
          <div className={styles.statCard}>
            <span>Kit attivi</span>
            <strong>{activeCount}</strong>
            <p>su {kitCount} totali</p>
          </div>
          <div className={styles.statCardAccent}>
            <span>Kit selezionato</span>
            <strong>{selectedKit ? `#${selectedKit.id}` : 'Nessuno'}</strong>
            <p>{selectedCategory?.name ?? 'Seleziona una riga per operare'}</p>
          </div>
        </div>
      </header>

      <section className={styles.toolbar}>
        <div className={styles.toolbarGroup}>
          <button type="button" className={styles.primaryButton} onClick={() => setCreateOpen(true)}>
            New Kit
          </button>
          <button
            type="button"
            className={styles.secondaryButton}
            disabled={!selectedKit}
            onClick={() => {
              if (!selectedKit) return;
              navigate(`/kit/${selectedKit.id}`);
            }}
          >
            Edit Kit
          </button>
        </div>
        <div className={styles.toolbarGroup}>
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
            Clone
          </button>
          <button type="button" className={styles.secondaryButton} onClick={() => void refetch()}>
            Refresh
          </button>
          <button
            type="button"
            className={styles.dangerButton}
            disabled={!selectedKit}
            onClick={() => setDeleteOpen(true)}
          >
            Soft Delete
          </button>
        </div>
        <div className={styles.toolbarGroup}>
          <button
            type="button"
            className={styles.secondaryButton}
            onClick={() => setColumnPickerOpen((current) => !current)}
          >
            Columns
          </button>
        </div>
      </section>

      {columnPickerOpen ? (
        <section className={styles.columnPicker}>
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
        </section>
      ) : null}

      <section className={styles.card}>
        {error ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Impossibile caricare i kit</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : sortedKits.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nessun kit disponibile</p>
            <p className={styles.emptyText}>Crea il primo kit per iniziare il catalogo bundle.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  {visibleColumns.map((column) => (
                    <th key={column.key}>{column.label}</th>
                  ))}
                  <th className={styles.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {sortedKits.map((kit) => {
                  const category = categories?.find((item) => item.id === kit.category_id);
                  return (
                    <tr
                      key={kit.id}
                      className={selectedId === kit.id ? styles.rowSelected : ''}
                      onClick={() => setSelectedId(kit.id)}
                    >
                      {visibleColumns.map((column) => (
                        <td key={column.key}>{renderKitColumn(kit, column.key, category?.name ?? kit.category_name, category?.color ?? kit.category_color)}</td>
                      ))}
                      <td className={styles.actionsCell}>
                        <button
                          type="button"
                          className={styles.linkButton}
                          onClick={(event) => {
                            event.stopPropagation();
                            navigate(`/kit/${kit.id}`);
                          }}
                        >
                          Edit
                        </button>
                      </td>
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
          <strong>{kit.internal_name}</strong>
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
