import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Modal, MultiSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCategories,
  useCustomFieldKeys,
  useCustomerGroups,
  useProducts,
  useVocabulary,
} from '../../api/queries';
import {
  useBatchUpdateKitProducts,
  useCreateKitCustomValue,
  useCreateKitProduct,
  useDeleteKitCustomValue,
  useDeleteKitProduct,
  useKit,
  useKitCustomValues,
  useKitProducts,
  useUpdateKit,
  useUpdateKitCustomValue,
  useUpdateKitHelp,
  useUpdateKitProduct,
  useUpdateKitTranslations,
} from './kitQueries';
import type {
  KitCustomValueItem,
  KitDetail,
  KitProductItem,
  KitProductWriteRequest,
  KitWriteRequest,
} from './kitTypes';
import styles from './KitDetailPage.module.css';

type TabKey = 'details' | 'products' | 'custom-values';

interface KitFormState extends KitWriteRequest {
  help_url: string;
}

interface TranslationDraft {
  short: string;
  long: string;
}

type TranslationDraftMap = Record<'it' | 'en', TranslationDraft>;

interface KitProductFormState extends KitProductWriteRequest {
  notes: string;
}

interface KitProductInlineDraft extends KitProductWriteRequest {
  notes: string | null;
}

interface KitCustomValueFormState {
  key_name: string;
  valueText: string;
}

interface KitCustomValueInlineDraft {
  key_name: string;
  valueText: string;
}

type KitToggleKey =
  | 'ecommerce'
  | 'is_active'
  | 'is_main_prd_sellable'
  | 'variable_billing'
  | 'h24_assurance';

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: 'details', label: 'Dettagli' },
  { key: 'products', label: 'Prodotti' },
  { key: 'custom-values', label: 'Valori custom' },
];

const billingPeriodOptions = [
  { value: 1, label: 'Mensile' },
  { value: 2, label: 'Bimestrale' },
  { value: 3, label: 'Trimestrale' },
  { value: 4, label: 'Quadrimestrale' },
  { value: 6, label: 'Semestrale' },
  { value: 12, label: 'Annuale' },
  { value: 24, label: 'Biennale' },
];

const kitToggleDefs: Array<{ key: KitToggleKey; label: string }> = [
  { key: 'ecommerce', label: 'Ecommerce' },
  { key: 'is_active', label: 'Active' },
  { key: 'is_main_prd_sellable', label: 'Main product sellable' },
  { key: 'variable_billing', label: 'Variable billing' },
  { key: 'h24_assurance', label: 'H24 assurance' },
];

export function KitDetailPage() {
  const { toast } = useToast();
  const params = useParams<{ id: string }>();
  const kitId = Number(params.id);
  const isValidId = Number.isFinite(kitId);

  const { data: kit, isLoading, error } = useKit(isValidId ? kitId : null);
  const { data: categories } = useCategories();
  const { data: customerGroups } = useCustomerGroups();
  const { data: products } = useProducts();
  const { data: vocabulary } = useVocabulary('kit_product_group');
  const { data: customFieldKeys } = useCustomFieldKeys();
  const {
    data: kitProducts,
    isLoading: isProductsLoading,
    error: productsError,
    refetch: refetchProducts,
  } = useKitProducts(isValidId ? kitId : null);
  const {
    data: customValues,
    isLoading: isCustomValuesLoading,
    error: customValuesError,
    refetch: refetchCustomValues,
  } = useKitCustomValues(isValidId ? kitId : null);
  const updateKit = useUpdateKit(isValidId ? kitId : null);
  const updateHelp = useUpdateKitHelp(isValidId ? kitId : null);
  const updateTranslations = useUpdateKitTranslations(isValidId ? kitId : null);
  const createKitProduct = useCreateKitProduct(isValidId ? kitId : null);
  const updateKitProduct = useUpdateKitProduct(isValidId ? kitId : null);
  const batchUpdateProducts = useBatchUpdateKitProducts(isValidId ? kitId : null);
  const deleteKitProduct = useDeleteKitProduct(isValidId ? kitId : null);
  const createCustomValue = useCreateKitCustomValue(isValidId ? kitId : null);
  const updateCustomValue = useUpdateKitCustomValue(isValidId ? kitId : null);
  const deleteCustomValue = useDeleteKitCustomValue(isValidId ? kitId : null);

  const [activeTab, setActiveTab] = useState<TabKey>('details');
  const [kitDraft, setKitDraft] = useState<KitFormState>(emptyKitFormState());
  const [translationDrafts, setTranslationDrafts] = useState<TranslationDraftMap>(emptyTranslationDrafts());
  const [productDrafts, setProductDrafts] = useState<Record<number, KitProductInlineDraft>>({});
  const [customValueDrafts, setCustomValueDrafts] = useState<Record<number, KitCustomValueInlineDraft>>({});
  const [helpUrlDraft, setHelpUrlDraft] = useState('');
  const [productModalOpen, setProductModalOpen] = useState(false);
  const [productDeleteId, setProductDeleteId] = useState<number | null>(null);
  const [productModalMode, setProductModalMode] = useState<'create' | 'edit'>('create');
  const [productModalDraft, setProductModalDraft] = useState<KitProductFormState>(emptyProductFormState());
  const [productEditingId, setProductEditingId] = useState<number | null>(null);
  const [customDeleteId, setCustomDeleteId] = useState<number | null>(null);
  const [customNewDraft, setCustomNewDraft] = useState<KitCustomValueFormState>(emptyCustomValueFormState());

  const initializedKitId = useRef<number | null>(null);

  useEffect(() => {
    if (!kit || initializedKitId.current === kit.id) {
      return;
    }

    initializedKitId.current = kit.id;
    setKitDraft(toKitFormState(kit));
    setTranslationDrafts(toTranslationDraftMap(kit));
    setHelpUrlDraft(kit.help_url ?? '');
    setProductDrafts({});
    setCustomValueDrafts({});
    setCustomNewDraft(emptyCustomValueFormState());
    setProductModalOpen(false);
    setProductDeleteId(null);
    setCustomDeleteId(null);
  }, [kit]);

  const productRows = kitProducts ?? [];
  const customValueRows = customValues ?? [];

  const selectedCategory = useMemo(
    () => categories?.find((category) => category.id === kitDraft.category_id) ?? null,
    [categories, kitDraft.category_id],
  );

  const metadataDirty = kit ? isKitFormDirty(kit, kitDraft) : false;
  const helpDirty = kit ? normalizeUrl(kit.help_url) !== normalizeUrl(helpUrlDraft) : false;
  const translationDirty = kit ? isTranslationDirty(kit, translationDrafts) : false;
  const changedProducts = useMemo(
    () =>
      productRows
        .filter((row) => {
          const draft = productDrafts[row.id];
          return draft != null && isKitProductDirty(row, draft);
        })
      .map((row) => ({
          id: row.id,
          product_code: productDrafts[row.id]!.product_code,
          group_name: productDrafts[row.id]!.group_name,
          minimum: productDrafts[row.id]!.minimum,
          maximum: productDrafts[row.id]!.maximum,
          required: productDrafts[row.id]!.required,
          nrc: productDrafts[row.id]!.nrc,
          mrc: productDrafts[row.id]!.mrc,
          position: productDrafts[row.id]!.position,
          notes: row.notes ?? null,
        })),
    [productDrafts, productRows],
  );

  const selectedProductRow = useMemo(
    () => productRows.find((row) => row.id === productEditingId) ?? null,
    [productEditingId, productRows],
  );

  const productGroupOptions = useMemo(
    () =>
      (vocabulary ?? []).map((item) => ({
        value: item.value,
        label: item.label,
      })),
    [vocabulary],
  );

  const customFieldKeyOptions = useMemo(
    () =>
      (customFieldKeys ?? []).map((item) => ({
        value: item.key_name,
        label: `${item.key_name} - ${item.key_description}`,
      })),
    [customFieldKeys],
  );

  async function handleSaveKit() {
    if (!kit) {
      return;
    }

    if (!kitDraft.internal_name.trim() || !kitDraft.main_product_code || kitDraft.category_id <= 0) {
      toast('Compila i campi obbligatori del kit prima di salvare', 'error');
      return;
    }

    try {
      const result = await updateKit.mutateAsync({
        ...kitDraft,
        internal_name: kitDraft.internal_name.trim(),
        bundle_prefix: kitDraft.bundle_prefix || null,
        notes: emptyToNull(kitDraft.notes),
      });
      setKitDraft(toKitFormState(result));
      toast('Kit aggiornato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare il kit'), 'error');
    }
  }

  async function handleSaveHelp() {
    if (!kit) {
      return;
    }

    try {
      await updateHelp.mutateAsync({ help_url: emptyToNull(helpUrlDraft) });
      setHelpUrlDraft(normalizeUrl(helpUrlDraft));
      toast('Help URL aggiornato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, "Impossibile aggiornare l'help URL"), 'error');
    }
  }

  async function handleSaveTranslations() {
    if (!kit) {
      return;
    }

    const translations = translationDraftsToArray(translationDrafts);

    try {
      await updateTranslations.mutateAsync({ translations });
      toast('Traduzioni aggiornate', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare le traduzioni'), 'error');
    }
  }

  async function handleBatchSaveProducts() {
    if (changedProducts.length === 0) {
      return;
    }

    try {
      await batchUpdateProducts.mutateAsync({ items: changedProducts });
      setProductDrafts((current) => {
        const next = { ...current };
        changedProducts.forEach((item) => {
          delete next[item.id];
        });
        return next;
      });
      await refetchProducts();
      toast('Prodotti aggiornati', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile salvare i prodotti'), 'error');
    }
  }

  async function handleSaveSingleProduct(row: KitProductItem) {
    const draft = productDrafts[row.id] ?? toProductInlineDraft(row);

    try {
        await updateKitProduct.mutateAsync({
          productId: row.id,
          product_code: draft.product_code,
          group_name: draft.group_name,
          minimum: draft.minimum,
          maximum: draft.maximum,
          required: draft.required,
          nrc: draft.nrc,
          mrc: draft.mrc,
          position: draft.position,
          notes: draft.notes ?? row.notes ?? null,
        });
      setProductDrafts((current) => {
        const next = { ...current };
        delete next[row.id];
        return next;
      });
      await refetchProducts();
      toast('Prodotto salvato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile salvare il prodotto del kit'), 'error');
    }
  }

  async function handleSaveProduct() {
    const editingId = productEditingId;
    try {
      if (productModalMode === 'create') {
        await createKitProduct.mutateAsync({
          ...productModalDraft,
          notes: emptyToNull(productModalDraft.notes),
        });
      } else if (editingId != null) {
        await updateKitProduct.mutateAsync({
          productId: editingId,
          ...productModalDraft,
          notes: emptyToNull(productModalDraft.notes),
        });
      }

      setProductModalOpen(false);
      setProductEditingId(null);
      if (editingId != null) {
        setProductDrafts((current) => {
          const next = { ...current };
          delete next[editingId];
          return next;
        });
      }
      await refetchProducts();
      toast(productModalMode === 'create' ? 'Prodotto aggiunto al kit' : 'Prodotto del kit aggiornato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile salvare il prodotto del kit'), 'error');
    }
  }

  async function handleDeleteProduct() {
    if (productDeleteId == null) {
      return;
    }

    try {
      await deleteKitProduct.mutateAsync(productDeleteId);
      setProductDeleteId(null);
      setProductDrafts((current) => {
        const next = { ...current };
        delete next[productDeleteId];
        return next;
      });
      await refetchProducts();
      toast('Prodotto rimosso dal kit', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile eliminare il prodotto del kit'), 'error');
    }
  }

  async function handleSaveCustomValue(id: number) {
    const draft = customValueDrafts[id];
    if (!draft) {
      return;
    }

    try {
      const payload = {
        key_name: draft.key_name,
        value: parseJsonValue(draft.valueText),
      };
      await updateCustomValue.mutateAsync({ valueId: id, ...payload });
      setCustomValueDrafts((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      await refetchCustomValues();
      toast('Valore custom aggiornato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare il valore custom'), 'error');
    }
  }

  async function handleCreateCustomValue() {
    if (!customNewDraft.key_name.trim()) {
      toast('Seleziona una chiave custom', 'error');
      return;
    }

    try {
      await createCustomValue.mutateAsync({
        key_name: customNewDraft.key_name,
        value: parseJsonValue(customNewDraft.valueText),
      });
      setCustomNewDraft(emptyCustomValueFormState());
      await refetchCustomValues();
      toast('Valore custom creato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile creare il valore custom'), 'error');
    }
  }

  async function handleDeleteCustomValue() {
    if (customDeleteId == null) {
      return;
    }

    try {
      await deleteCustomValue.mutateAsync(customDeleteId);
      setCustomDeleteId(null);
      setCustomValueDrafts((current) => {
        const next = { ...current };
        delete next[customDeleteId];
        return next;
      });
      await refetchCustomValues();
      toast('Valore custom eliminato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile eliminare il valore custom'), 'error');
    }
  }

  if (!isValidId) {
    return (
      <section className={styles.page}>
        <div className={styles.emptyState}>
          <p className={styles.emptyTitle}>Kit non valido</p>
          <p className={styles.emptyText}>L&apos;identificativo nella route non e valido.</p>
        </div>
      </section>
    );
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={10} />
      </section>
    );
  }

  if (error || !kit) {
    return (
      <section className={styles.page}>
        <div className={styles.emptyState}>
          <p className={styles.emptyTitle}>Impossibile caricare il kit</p>
          <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
        </div>
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.hero}>
        <div className={styles.breadcrumbRow}>
          <Link to="/kit" className={styles.backLink}>
            <span aria-hidden="true">←</span> Tutti i Kit
          </Link>
          <span className={styles.crumbSeparator}>/</span>
          <span className={styles.crumbCurrent}>KIT #{kit.id}</span>
        </div>
        <div className={styles.heroTop}>
          <div>
            <p className={styles.eyebrow}>Phase 5</p>
            <h1>
              KIT #{kit.id} - {kit.internal_name}
            </h1>
            <p className={styles.lead}>
              Editor completo per metadati, traduzioni, prodotti associati e valori custom.
            </p>
          </div>
          <div className={styles.summaryCard}>
            <span>{selectedCategory?.name ?? kit.category_name}</span>
            <strong>{kit.is_active ? 'Attivo' : 'Disattivo'}</strong>
            <p>
              {kit.main_product_code ? `Main product: ${kit.main_product_code}` : 'Main product non impostato'}
            </p>
          </div>
        </div>
      </header>

      <nav className={styles.tabRail} aria-label="Sezioni kit">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            className={`${styles.tabButton} ${activeTab === tab.key ? styles.tabButtonActive : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </nav>

      {activeTab === 'details' ? (
        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>Dettagli</h2>
              <p>Metadati, gruppi vendibili, help URL e traduzioni del kit.</p>
            </div>
            <div className={styles.panelActions}>
              <button
                type="button"
                className={styles.secondaryButton}
                disabled={!helpDirty || updateHelp.isPending}
                onClick={() => void handleSaveHelp()}
              >
                Save help URL
              </button>
              <button
                type="button"
                className={styles.secondaryButton}
                disabled={!translationDirty || updateTranslations.isPending}
                onClick={() => void handleSaveTranslations()}
              >
                Save translations
              </button>
              <button
                type="button"
                className={styles.primaryButton}
                disabled={!metadataDirty || updateKit.isPending}
                onClick={() => void handleSaveKit()}
              >
                Save kit
              </button>
            </div>
          </div>

          <div className={styles.detailGrid}>
            <section className={styles.card}>
              <div className={styles.sectionTitle}>
                <h3>Metadati</h3>
                <p>Le modifiche salvano il kit e la relazione con i gruppi cliente.</p>
              </div>
              <div className={styles.formGrid}>
                <label className={styles.field}>
                  <span>Internal name</span>
                  <input
                    value={kitDraft.internal_name}
                    onChange={(event) => setKitDraft((current) => ({ ...current, internal_name: event.target.value }))}
                  />
                </label>
                <label className={styles.field}>
                  <span>Main product</span>
                  <select
                    value={kitDraft.main_product_code ?? ''}
                    onChange={(event) =>
                      setKitDraft((current) => ({ ...current, main_product_code: event.target.value || null }))
                    }
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
                  <span>Categoria</span>
                  <select
                    value={kitDraft.category_id}
                    onChange={(event) =>
                      setKitDraft((current) => ({ ...current, category_id: Number(event.target.value) }))
                    }
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
                  <input value={kitDraft.bundle_prefix ?? ''} disabled />
                </label>
                <label className={styles.field}>
                  <span>Billing period</span>
                  <select
                    value={kitDraft.billing_period}
                    onChange={(event) =>
                      setKitDraft((current) => ({ ...current, billing_period: Number(event.target.value) }))
                    }
                  >
                    {billingPeriodOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className={styles.field}>
                  <span>Sellable groups</span>
                  <MultiSelect<number>
                    options={(customerGroups ?? []).map((group) => ({ value: group.id, label: group.name }))}
                    selected={kitDraft.sellable_group_ids}
                    onChange={(values) => setKitDraft((current) => ({ ...current, sellable_group_ids: values }))}
                    placeholder="Seleziona gruppi"
                  />
                </label>
                <label className={styles.field}>
                  <span>Initial months</span>
                  <input
                    type="number"
                    value={kitDraft.initial_subscription_months}
                    onChange={(event) =>
                      setKitDraft((current) => ({
                        ...current,
                        initial_subscription_months: Number(event.target.value),
                      }))
                    }
                  />
                </label>
                <label className={styles.field}>
                  <span>Next months</span>
                  <input
                    type="number"
                    value={kitDraft.next_subscription_months}
                    onChange={(event) =>
                      setKitDraft((current) => ({
                        ...current,
                        next_subscription_months: Number(event.target.value),
                      }))
                    }
                  />
                </label>
                <label className={styles.field}>
                  <span>Activation days</span>
                  <input
                    type="number"
                    value={kitDraft.activation_time_days}
                    onChange={(event) =>
                      setKitDraft((current) => ({
                        ...current,
                        activation_time_days: Number(event.target.value),
                      }))
                    }
                  />
                </label>
                <label className={styles.field}>
                  <span>NRC</span>
                  <input
                    type="number"
                    step="0.01"
                    value={kitDraft.nrc}
                    onChange={(event) => setKitDraft((current) => ({ ...current, nrc: Number(event.target.value) }))}
                  />
                </label>
                <label className={styles.field}>
                  <span>MRC</span>
                  <input
                    type="number"
                    step="0.01"
                    value={kitDraft.mrc}
                    onChange={(event) => setKitDraft((current) => ({ ...current, mrc: Number(event.target.value) }))}
                  />
                </label>
                <label className={styles.field}>
                  <span>Sconto massimo</span>
                  <input
                    type="number"
                    step="0.01"
                    value={kitDraft.sconto_massimo}
                    onChange={(event) =>
                      setKitDraft((current) => ({ ...current, sconto_massimo: Number(event.target.value) }))
                    }
                  />
                </label>
                <label className={styles.field}>
                  <span>SLA resolution hours</span>
                  <input
                    type="number"
                    value={kitDraft.sla_resolution_hours}
                    onChange={(event) =>
                      setKitDraft((current) => ({ ...current, sla_resolution_hours: Number(event.target.value) }))
                    }
                  />
                </label>
                <label className={styles.field}>
                  <span>Notes</span>
                  <textarea
                    value={kitDraft.notes ?? ''}
                    onChange={(event) => setKitDraft((current) => ({ ...current, notes: event.target.value }))}
                    rows={4}
                  />
                </label>
              </div>
              <div className={styles.toggleGrid}>
                {kitToggleDefs.map((item) => (
                  <label key={item.key} className={styles.toggle}>
                    <input
                      type="checkbox"
                      checked={kitDraft[item.key]}
                      onChange={(event) =>
                        setKitDraft((current) => ({ ...current, [item.key]: event.target.checked } as KitFormState))
                      }
                    />
                    <span>{item.label}</span>
                  </label>
                ))}
              </div>
            </section>

            <section className={styles.card}>
              <div className={styles.sectionTitle}>
                <h3>Traduzioni</h3>
                <p>Salvataggio indipendente dal blocco metadati.</p>
              </div>
              <div className={styles.translationGrid}>
                {(['it', 'en'] as const).map((language) => (
                  <article key={language} className={styles.translationCard}>
                    <div className={styles.translationHeader}>
                      <strong>{language.toUpperCase()}</strong>
                      <span>Kit description</span>
                    </div>
                    <label className={styles.field}>
                      <span>Short</span>
                      <input
                        value={translationDrafts[language].short}
                        onChange={(event) =>
                          setTranslationDrafts((current) => ({
                            ...current,
                            [language]: { ...current[language], short: event.target.value },
                          }))
                        }
                      />
                    </label>
                    <label className={styles.field}>
                      <span>Long</span>
                      <textarea
                        value={translationDrafts[language].long}
                        onChange={(event) =>
                          setTranslationDrafts((current) => ({
                            ...current,
                            [language]: { ...current[language], long: event.target.value },
                          }))
                        }
                        rows={6}
                      />
                    </label>
                  </article>
                ))}
              </div>
            </section>
          </div>

          <section className={styles.card}>
            <div className={styles.sectionTitle}>
              <h3>Context</h3>
              <p>Informazioni utili per riconoscere la riga e verificare l&apos;allineamento con il contratto.</p>
            </div>
            <div className={styles.metaGrid}>
              <div>
                <span>ID</span>
                <strong>#{kit.id}</strong>
              </div>
              <div>
                <span>Translation UUID</span>
                <strong className={styles.mono}>{kit.translation_uuid}</strong>
              </div>
              <div>
                <span>Categoria</span>
                <strong>{selectedCategory?.name ?? kit.category_name}</strong>
              </div>
              <div>
                <span>Help URL</span>
                <strong>{normalizeUrl(helpUrlDraft) || 'n/d'}</strong>
              </div>
            </div>
          </section>

          <section className={styles.card}>
            <div className={styles.sectionTitle}>
              <h3>Help URL</h3>
              <p>Salvataggio tramite endpoint dedicato.</p>
            </div>
            <div className={styles.helpRow}>
              <input
                value={helpUrlDraft}
                onChange={(event) => setHelpUrlDraft(event.target.value)}
                placeholder="https://"
              />
              <button
                type="button"
                className={styles.secondaryButton}
                disabled={!helpDirty || updateHelp.isPending}
                onClick={() => void handleSaveHelp()}
              >
                Save help URL
              </button>
            </div>
          </section>
        </section>
      ) : null}

      {activeTab === 'products' ? (
        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>Prodotti</h2>
              <p>Modifica i componenti del kit con salvataggio batch e dialog dedicato per create/edit.</p>
            </div>
            <div className={styles.panelActions}>
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => {
                  setProductModalMode('create');
                  setProductEditingId(null);
                  setProductModalDraft(emptyProductFormState());
                  setProductModalOpen(true);
                }}
              >
                Add
              </button>
              <button
                type="button"
                className={styles.secondaryButton}
                disabled={productEditingId == null}
                onClick={() => {
                  const row = selectedProductRow;
                  if (!row) {
                    return;
                  }
                  setProductModalMode('edit');
                  setProductModalDraft(toProductFormState(row));
                  setProductModalOpen(true);
                }}
              >
                Edit
              </button>
              <button
                type="button"
                className={styles.secondaryButton}
                disabled={changedProducts.length === 0 || batchUpdateProducts.isPending}
                onClick={() => void handleBatchSaveProducts()}
              >
                Save batch
              </button>
              <button type="button" className={styles.secondaryButton} onClick={() => void refetchProducts()}>
                Refresh
              </button>
              <button
                type="button"
                className={styles.dangerButton}
                disabled={productEditingId == null}
                onClick={() => {
                  if (productEditingId != null) {
                    setProductDeleteId(productEditingId);
                  }
                }}
              >
                Delete
              </button>
            </div>
          </div>

          {isProductsLoading ? <Skeleton rows={6} /> : null}

          {productsError ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Impossibile caricare i prodotti del kit</p>
              <p className={styles.emptyText}>{getErrorMessage(productsError, 'Riprova tra poco.')}</p>
            </div>
          ) : productRows.length === 0 ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Nessun prodotto associato</p>
              <p className={styles.emptyText}>Aggiungi il primo prodotto con il pulsante Add.</p>
            </div>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Product</th>
                    <th>Group</th>
                    <th>Min</th>
                    <th>Max</th>
                    <th>Required</th>
                    <th>NRC</th>
                    <th>MRC</th>
                    <th>Position</th>
                    <th>Notes</th>
                    <th className={styles.actionsCell}>Azioni</th>
                  </tr>
                </thead>
                <tbody>
                  {productRows.map((row) => {
                    const draft = productDrafts[row.id] ?? toProductInlineDraft(row);
                    const dirty = isKitProductDirty(row, draft);
                    const selected = productEditingId === row.id;
                    return (
                      <tr
                        key={row.id}
                        className={selected ? styles.rowSelected : ''}
                        onClick={() => setProductEditingId(row.id)}
                      >
                        <td>
                          <span className={styles.codeCell}>
                            {row.product_code}
                            <small>{resolveProductLabel(products, row)}</small>
                          </span>
                        </td>
                        <td>
                          <select
                            value={draft.group_name ?? ''}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, {
                                group_name: event.target.value || null,
                              })
                            }
                          >
                            <option value="">Nessuno</option>
                            {productGroupOptions.map((option) => (
                              <option key={option.value} value={option.value}>
                                {option.label}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td>
                          <input
                            type="number"
                            value={draft.minimum}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, { minimum: Number(event.target.value) })
                            }
                          />
                        </td>
                        <td>
                          <input
                            type="number"
                            value={draft.maximum}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, { maximum: Number(event.target.value) })
                            }
                          />
                        </td>
                        <td>
                          <label className={styles.checkboxInline}>
                            <input
                              type="checkbox"
                              checked={draft.required}
                              onChange={(event) =>
                                updateProductDraft(row.id, setProductDrafts, { required: event.target.checked })
                              }
                            />
                            <span>{draft.required ? 'Si' : 'No'}</span>
                          </label>
                        </td>
                        <td>
                          <input
                            type="number"
                            step="0.01"
                            value={draft.nrc}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, { nrc: Number(event.target.value) })
                            }
                          />
                        </td>
                        <td>
                          <input
                            type="number"
                            step="0.01"
                            value={draft.mrc}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, { mrc: Number(event.target.value) })
                            }
                          />
                        </td>
                        <td>
                          <input
                            type="number"
                            value={draft.position}
                            onChange={(event) =>
                              updateProductDraft(row.id, setProductDrafts, { position: Number(event.target.value) })
                            }
                          />
                        </td>
                        <td className={styles.notesCell}>{row.notes ?? 'n/d'}</td>
                        <td className={styles.actionsCell}>
                          <button
                            type="button"
                            className={styles.linkButton}
                            disabled={!dirty}
                          onClick={(event) => {
                            event.stopPropagation();
                            void handleSaveSingleProduct(row);
                          }}
                          >
                            Save
                          </button>
                          <button
                            type="button"
                            className={styles.linkButton}
                            onClick={(event) => {
                              event.stopPropagation();
                              setProductEditingId(row.id);
                              setProductModalMode('edit');
                              setProductModalDraft(toProductFormState(row));
                              setProductModalOpen(true);
                            }}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className={styles.linkButtonDanger}
                            onClick={(event) => {
                              event.stopPropagation();
                              setProductDeleteId(row.id);
                            }}
                          >
                            Delete
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
      ) : null}

      {activeTab === 'custom-values' ? (
        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>Valori custom</h2>
              <p>Chiavi registry e JSON value con salvataggio per riga.</p>
            </div>
            <div className={styles.panelActions}>
              <button type="button" className={styles.primaryButton} onClick={() => void handleCreateCustomValue()}>
                Add
              </button>
              <button type="button" className={styles.secondaryButton} onClick={() => void refetchCustomValues()}>
                Refresh
              </button>
            </div>
          </div>

          <section className={styles.inlineCreateCard}>
            <div className={styles.inlineCreateGrid}>
              <label className={styles.field}>
                <span>Key name</span>
                <select
                  value={customNewDraft.key_name}
                  onChange={(event) => setCustomNewDraft((current) => ({ ...current, key_name: event.target.value }))}
                >
                  <option value="">Seleziona</option>
                  {customFieldKeyOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className={styles.field}>
                <span>Value JSON</span>
                <textarea
                  value={customNewDraft.valueText}
                  onChange={(event) =>
                    setCustomNewDraft((current) => ({ ...current, valueText: event.target.value }))
                  }
                  rows={4}
                  placeholder='{"example": true}'
                />
              </label>
            </div>
            <div className={styles.inlineCreateActions}>
              <button type="button" className={styles.secondaryButton} onClick={() => setCustomNewDraft(emptyCustomValueFormState())}>
                Reset
              </button>
              <button type="button" className={styles.primaryButton} onClick={() => void handleCreateCustomValue()}>
                Create
              </button>
            </div>
          </section>

          {isCustomValuesLoading ? <Skeleton rows={5} /> : null}

          {customValuesError ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Impossibile caricare i valori custom</p>
              <p className={styles.emptyText}>{getErrorMessage(customValuesError, 'Riprova tra poco.')}</p>
            </div>
          ) : customValueRows.length === 0 ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyTitle}>Nessun valore custom presente</p>
              <p className={styles.emptyText}>Usa il form sopra per aggiungere il primo elemento.</p>
            </div>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Key name</th>
                    <th>Value</th>
                    <th className={styles.actionsCell}>Azioni</th>
                  </tr>
                </thead>
                <tbody>
                  {customValueRows.map((row) => {
                    const draft = customValueDrafts[row.id] ?? toCustomValueInlineDraft(row);
                    const dirty = isCustomValueDirty(row, draft);
                    return (
                      <tr key={row.id}>
                        <td>
                          <select
                            value={draft.key_name}
                            onChange={(event) =>
                              updateCustomValueDraft(row.id, setCustomValueDrafts, { key_name: event.target.value })
                            }
                          >
                            <option value="">Seleziona</option>
                            {customFieldKeyOptions.map((option) => (
                              <option key={option.value} value={option.value}>
                                {option.label}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td>
                          <textarea
                            rows={4}
                            value={draft.valueText}
                            onChange={(event) =>
                              updateCustomValueDraft(row.id, setCustomValueDrafts, { valueText: event.target.value })
                            }
                          />
                        </td>
                        <td className={styles.actionsCell}>
                          <button
                            type="button"
                            className={styles.linkButton}
                            disabled={!dirty}
                            onClick={() => void handleSaveCustomValue(row.id)}
                          >
                            Save
                          </button>
                          <button
                            type="button"
                            className={styles.linkButtonDanger}
                            onClick={() => setCustomDeleteId(row.id)}
                          >
                            Delete
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
      ) : null}

      <Modal
        open={productModalOpen}
        onClose={() => setProductModalOpen(false)}
        title={productModalMode === 'create' ? 'Aggiungi prodotto al kit' : 'Modifica prodotto del kit'}
      >
        <div className={styles.modalBody}>
          <div className={styles.formGrid}>
            <label className={styles.field}>
              <span>Product</span>
              <select
                value={productModalDraft.product_code}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, product_code: event.target.value }))
                }
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
              <span>Group name</span>
              <select
                value={productModalDraft.group_name ?? ''}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, group_name: event.target.value || null }))
                }
              >
                <option value="">Nessuno</option>
                {productGroupOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span>Minimum</span>
              <input
                type="number"
                value={productModalDraft.minimum}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, minimum: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>Maximum</span>
              <input
                type="number"
                value={productModalDraft.maximum}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, maximum: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>Position</span>
              <input
                type="number"
                value={productModalDraft.position}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, position: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>NRC</span>
              <input
                type="number"
                step="0.01"
                value={productModalDraft.nrc}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, nrc: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input
                type="number"
                step="0.01"
                value={productModalDraft.mrc}
                onChange={(event) =>
                  setProductModalDraft((current) => ({ ...current, mrc: Number(event.target.value) }))
                }
              />
            </label>
            <label className={styles.field}>
              <span>Required</span>
              <label className={styles.checkboxInline}>
                <input
                  type="checkbox"
                  checked={productModalDraft.required}
                  onChange={(event) =>
                    setProductModalDraft((current) => ({ ...current, required: event.target.checked }))
                  }
                />
                <span>{productModalDraft.required ? 'Si' : 'No'}</span>
              </label>
            </label>
            <label className={styles.field}>
              <span>Notes</span>
              <textarea
                rows={4}
                value={productModalDraft.notes}
                onChange={(event) => setProductModalDraft((current) => ({ ...current, notes: event.target.value }))}
              />
            </label>
          </div>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setProductModalOpen(false)}>
              Annulla
            </button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleSaveProduct()}>
              Salva
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={productDeleteId != null}
        onClose={() => setProductDeleteId(null)}
        title="Rimuovi prodotto"
      >
        <div className={styles.modalBody}>
          <p className={styles.modalLead}>Confermi la rimozione del prodotto dal kit?</p>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setProductDeleteId(null)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.dangerButton}
              onClick={() => void handleDeleteProduct()}
            >
              Elimina
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={customDeleteId != null}
        onClose={() => setCustomDeleteId(null)}
        title="Rimuovi valore custom"
      >
        <div className={styles.modalBody}>
          <p className={styles.modalLead}>Confermi la rimozione del valore custom?</p>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCustomDeleteId(null)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.dangerButton}
              onClick={() => void handleDeleteCustomValue()}
            >
              Elimina
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function updateProductDraft(
  id: number,
  setDrafts: Dispatch<SetStateAction<Record<number, KitProductInlineDraft>>>,
  patch: Partial<KitProductInlineDraft>,
) {
  setDrafts((current) => ({
    ...current,
    [id]: {
      ...toProductInlineDraftFromCurrent(current[id]),
      ...patch,
    },
  }));
}

function updateCustomValueDraft(
  id: number,
  setDrafts: Dispatch<SetStateAction<Record<number, KitCustomValueInlineDraft>>>,
  patch: Partial<KitCustomValueInlineDraft>,
) {
  setDrafts((current) => ({
    ...current,
    [id]: {
      ...toCustomValueInlineDraftFromCurrent(current[id]),
      ...patch,
    },
  }));
}

function toProductInlineDraft(row: KitProductItem): KitProductInlineDraft {
  return {
    product_code: row.product_code,
    group_name: row.group_name,
    minimum: row.minimum,
    maximum: row.maximum,
    required: row.required,
    nrc: row.nrc,
    mrc: row.mrc,
    position: row.position,
    notes: row.notes ?? null,
  };
}

function toProductInlineDraftFromCurrent(current: KitProductInlineDraft | undefined): KitProductInlineDraft {
  return current ?? {
    product_code: '',
    group_name: null,
    minimum: 0,
    maximum: -1,
    required: false,
    nrc: 0,
    mrc: 0,
    position: 0,
    notes: null,
  };
}

function toProductFormState(row: KitProductItem): KitProductFormState {
  return {
    ...toProductInlineDraft(row),
    notes: row.notes ?? '',
  };
}

function emptyProductFormState(): KitProductFormState {
  return {
    product_code: '',
    group_name: null,
    minimum: 0,
    maximum: -1,
    required: false,
    nrc: 0,
    mrc: 0,
    position: 0,
    notes: '',
  };
}

function emptyCustomValueFormState(): KitCustomValueFormState {
  return {
    key_name: '',
    valueText: '',
  };
}

function isKitProductDirty(row: KitProductItem, draft: KitProductInlineDraft) {
  return (
    row.product_code !== draft.product_code ||
    normalizeNullable(row.group_name) !== normalizeNullable(draft.group_name) ||
    row.minimum !== draft.minimum ||
    row.maximum !== draft.maximum ||
    row.required !== draft.required ||
    Number(row.nrc) !== Number(draft.nrc) ||
    Number(row.mrc) !== Number(draft.mrc) ||
    row.position !== draft.position
  );
}

function toCustomValueInlineDraft(row: KitCustomValueItem): KitCustomValueInlineDraft {
  return {
    key_name: row.key_name,
    valueText: stringifyJsonValue(row.value),
  };
}

function toCustomValueInlineDraftFromCurrent(
  current: KitCustomValueInlineDraft | undefined,
): KitCustomValueInlineDraft {
  return current ?? { key_name: '', valueText: '' };
}

function isCustomValueDirty(row: KitCustomValueItem, draft: KitCustomValueInlineDraft) {
  return row.key_name !== draft.key_name || stringifyJsonValue(row.value) !== draft.valueText.trim();
}

function toKitFormState(kit: KitDetail): KitFormState {
  return {
    internal_name: kit.internal_name,
    main_product_code: kit.main_product_code,
    category_id: kit.category_id,
    bundle_prefix: kit.bundle_prefix ?? '',
    initial_subscription_months: kit.initial_subscription_months,
    next_subscription_months: kit.next_subscription_months,
    activation_time_days: kit.activation_time_days,
    nrc: kit.nrc,
    mrc: kit.mrc,
    ecommerce: kit.ecommerce,
    is_active: kit.is_active,
    is_main_prd_sellable: kit.is_main_prd_sellable ?? true,
    billing_period: kit.billing_period,
    sconto_massimo: kit.sconto_massimo,
    variable_billing: kit.variable_billing,
    h24_assurance: kit.h24_assurance,
    sla_resolution_hours: kit.sla_resolution_hours,
    notes: kit.notes ?? '',
    sellable_group_ids: kit.sellable_group_ids ?? [],
    help_url: kit.help_url ?? '',
  };
}

function emptyKitFormState(): KitFormState {
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
    is_active: true,
    is_main_prd_sellable: true,
    billing_period: 3,
    sconto_massimo: 0,
    variable_billing: false,
    h24_assurance: false,
    sla_resolution_hours: 0,
    notes: '',
    sellable_group_ids: [],
    help_url: '',
  };
}

function emptyTranslationDrafts(): TranslationDraftMap {
  return {
    it: { short: '', long: '' },
    en: { short: '', long: '' },
  };
}

function toTranslationDraftMap(kit: KitDetail): TranslationDraftMap {
  const base = emptyTranslationDrafts();
  kit.translations.forEach((translation) => {
    base[translation.language] = {
      short: translation.short ?? '',
      long: translation.long ?? '',
    };
  });
  return base;
}

function translationDraftsToArray(drafts: TranslationDraftMap) {
  return (['it', 'en'] as const).map((language) => ({
    language,
    short: drafts[language].short,
    long: drafts[language].long,
  }));
}

function isTranslationDirty(kit: KitDetail, drafts: TranslationDraftMap) {
  const current = toTranslationDraftMap(kit);
  return (
    current.it.short !== drafts.it.short ||
    current.it.long !== drafts.it.long ||
    current.en.short !== drafts.en.short ||
    current.en.long !== drafts.en.long
  );
}

function isKitFormDirty(kit: KitDetail, draft: KitFormState) {
  return (
    kit.internal_name !== draft.internal_name ||
    normalizeNullable(kit.main_product_code) !== normalizeNullable(draft.main_product_code) ||
    kit.category_id !== draft.category_id ||
    normalizeNullable(kit.bundle_prefix) !== normalizeNullable(draft.bundle_prefix) ||
    kit.initial_subscription_months !== draft.initial_subscription_months ||
    kit.next_subscription_months !== draft.next_subscription_months ||
    kit.activation_time_days !== draft.activation_time_days ||
    Number(kit.nrc) !== Number(draft.nrc) ||
    Number(kit.mrc) !== Number(draft.mrc) ||
    kit.ecommerce !== draft.ecommerce ||
    kit.is_active !== draft.is_active ||
    Boolean(kit.is_main_prd_sellable) !== draft.is_main_prd_sellable ||
    kit.billing_period !== draft.billing_period ||
    Number(kit.sconto_massimo) !== Number(draft.sconto_massimo) ||
    kit.variable_billing !== draft.variable_billing ||
    kit.h24_assurance !== draft.h24_assurance ||
    kit.sla_resolution_hours !== draft.sla_resolution_hours ||
    normalizeNullable(kit.notes) !== normalizeNullable(draft.notes) ||
    !arraysEqual(kit.sellable_group_ids ?? [], draft.sellable_group_ids)
  );
}

function arraysEqual(left: number[], right: number[]) {
  if (left.length !== right.length) {
    return false;
  }
  return left.every((value, index) => value === right[index]);
}

function normalizeNullable(value: string | number | null | undefined) {
  if (value == null) {
    return '';
  }
  return String(value).trim();
}

function normalizeUrl(value: string | null | undefined) {
  const next = value?.trim();
  return next ? next : '';
}

function parseJsonValue(valueText: string) {
  const text = valueText.trim();
  if (text.length === 0) {
    return null;
  }

  try {
    return JSON.parse(text);
  } catch {
    throw new Error('Il valore custom deve essere JSON valido');
  }
}

function stringifyJsonValue(value: unknown) {
  if (typeof value === 'string') {
    return value;
  }

  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function resolveProductLabel(
  products: Array<{ code: string; internal_name: string }> | undefined,
  row: KitProductItem,
) {
  const match = products?.find((product) => product.code === row.product_code);
  return match ? match.internal_name : row.product_internal_name ?? row.product_name ?? 'n/d';
}

function emptyToNull(value: string | null | undefined) {
  const next = value?.trim();
  return next ? next : null;
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
