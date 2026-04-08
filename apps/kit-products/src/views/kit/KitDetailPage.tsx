import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Modal, MultiSelect, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCategories,
  useCustomFieldKeys,
  useCustomerGroups,
  useProducts,
  useVocabulary,
} from '../../api/queries';
import {
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
  const deleteKitProduct = useDeleteKitProduct(isValidId ? kitId : null);
  const createCustomValue = useCreateKitCustomValue(isValidId ? kitId : null);
  const updateCustomValue = useUpdateKitCustomValue(isValidId ? kitId : null);
  const deleteCustomValue = useDeleteKitCustomValue(isValidId ? kitId : null);

  const [kitDraft, setKitDraft] = useState<KitFormState>(emptyKitFormState());
  const [translationDrafts, setTranslationDrafts] = useState<TranslationDraftMap>(emptyTranslationDrafts());
  const [productDrafts, setProductDrafts] = useState<Record<number, KitProductInlineDraft>>({});
  const [helpUrlDraft, setHelpUrlDraft] = useState('');
  const [productModalOpen, setProductModalOpen] = useState(false);
  const [productDeleteId, setProductDeleteId] = useState<number | null>(null);
  const [productModalMode, setProductModalMode] = useState<'create' | 'edit'>('create');
  const [productModalDraft, setProductModalDraft] = useState<KitProductFormState>(emptyProductFormState());
  const [productEditingId, setProductEditingId] = useState<number | null>(null);
  const [customDeleteId, setCustomDeleteId] = useState<number | null>(null);
  const [customNewDraft, setCustomNewDraft] = useState<KitCustomValueFormState>(emptyCustomValueFormState());
  const [customModalOpen, setCustomModalOpen] = useState(false);
  const [customModalMode, setCustomModalMode] = useState<'create' | 'edit'>('create');
  const [customEditingId, setCustomEditingId] = useState<number | null>(null);

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
    setCustomNewDraft(emptyCustomValueFormState());
    setCustomEditingId(null);
    setProductModalOpen(false);
    setProductDeleteId(null);
    setProductEditingId(null);
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

  const anyDirty = metadataDirty || helpDirty || translationDirty;
  const anySaving = updateKit.isPending || updateHelp.isPending || updateTranslations.isPending;

  async function handleSaveAll() {
    if (metadataDirty) await handleSaveKit();
    if (helpDirty) await handleSaveHelp();
    if (translationDirty) await handleSaveTranslations();
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
      setProductEditingId((current) => (current === productDeleteId ? null : current));
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


  async function handleSaveCustomModal() {
    if (!customNewDraft.key_name.trim()) {
      toast('Seleziona una chiave custom', 'error');
      return;
    }
    try {
      const payload = { key_name: customNewDraft.key_name, value: parseJsonValue(customNewDraft.valueText) };
      if (customModalMode === 'create') {
        await createCustomValue.mutateAsync(payload);
      } else if (customEditingId != null) {
        await updateCustomValue.mutateAsync({ valueId: customEditingId, ...payload });
      }
      setCustomModalOpen(false);
      setCustomNewDraft(emptyCustomValueFormState());
      setCustomEditingId(null);
      await refetchCustomValues();
      toast(customModalMode === 'create' ? 'Valore custom creato' : 'Valore custom aggiornato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile salvare il valore custom'), 'error');
    }
  }

  async function handleDeleteCustomValue() {
    if (customDeleteId == null) {
      return;
    }

    try {
      await deleteCustomValue.mutateAsync(customDeleteId);
      setCustomDeleteId(null);
      setCustomEditingId((c) => (c === customDeleteId ? null : c));
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
          <div className={styles.emptyIcon}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
          </div>
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
          <div className={styles.emptyIcon}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
          </div>
          <p className={styles.emptyTitle}>Impossibile caricare il kit</p>
          <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
        </div>
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <div className={styles.breadcrumbRow}>
            <Link to="/kit" className={styles.backLink}>← Kit</Link>
            <span className={styles.crumbSeparator}>/</span>
            <span>#{kit.id}</span>
          </div>
          <h1>{kit.internal_name}</h1>
          <div className={styles.metaRow}>
            <span className={`${styles.statusDot} ${kit.is_active ? styles.statusActive : ''}`} />
            <span>{kit.is_active ? 'Attivo' : 'Disattivo'}</span>
            <span className={styles.metaSep}>·</span>
            <span>{selectedCategory?.name ?? kit.category_name}</span>
            {kit.main_product_code ? (
              <>
                <span className={styles.metaSep}>·</span>
                <code className={styles.mono}>{kit.main_product_code}</code>
              </>
            ) : null}
          </div>
        </div>
      </header>

      <nav className={styles.anchorRail} aria-label="Sezioni">
        <div className={styles.anchorLinks}>
          <a href="#identity" className={styles.anchorLink}>Dettagli</a>
          <a href="#products" className={styles.anchorLink}>Prodotti ({productRows.length})</a>
          <a href="#content" className={styles.anchorLink}>Note e traduzioni</a>
          <a href="#custom-values" className={styles.anchorLink}>Custom ({customValueRows.length})</a>
        </div>
        <div className={styles.anchorActions}>
          {anyDirty ? <span className={styles.dirtyPill}>Modifiche non salvate</span> : null}
          <button
            type="button"
            className={styles.primaryButton}
            disabled={!anyDirty || anySaving}
            onClick={() => void handleSaveAll()}
          >
            Salva tutto
          </button>
        </div>
      </nav>

      {/* ── Identity & Pricing ── */}
      <section id="identity" className={styles.card}>
        <h3>Dettagli e pricing</h3>
        <div className={styles.formGrid}>
          <label className={styles.field}>
            <span>Internal name</span>
            <input value={kitDraft.internal_name} onChange={(e) => setKitDraft((c) => ({ ...c, internal_name: e.target.value }))} />
          </label>
          <label className={styles.field}>
            <span>Main product</span>
            <select value={kitDraft.main_product_code ?? ''} onChange={(e) => setKitDraft((c) => ({ ...c, main_product_code: e.target.value || null }))}>
              <option value="">Seleziona</option>
              {products?.map((p) => <option key={p.code} value={p.code}>{p.code} - {p.internal_name}</option>)}
            </select>
          </label>
          <label className={styles.field}>
            <span>Categoria</span>
            <select value={kitDraft.category_id} onChange={(e) => setKitDraft((c) => ({ ...c, category_id: Number(e.target.value) }))}>
              <option value={0}>Seleziona</option>
              {categories?.map((cat) => <option key={cat.id} value={cat.id}>{cat.name}</option>)}
            </select>
          </label>
          <label className={styles.field}>
            <span>Bundle prefix</span>
            <input value={kitDraft.bundle_prefix ?? ''} disabled />
          </label>
        </div>
        <div className={styles.formGrid4}>
          <label className={styles.field}>
            <span>NRC</span>
            <input type="number" step="0.01" value={kitDraft.nrc} onChange={(e) => setKitDraft((c) => ({ ...c, nrc: Number(e.target.value) }))} />
          </label>
          <label className={styles.field}>
            <span>MRC</span>
            <input type="number" step="0.01" value={kitDraft.mrc} onChange={(e) => setKitDraft((c) => ({ ...c, mrc: Number(e.target.value) }))} />
          </label>
          <label className={styles.field}>
            <span>Sconto massimo</span>
            <input type="number" step="0.01" value={kitDraft.sconto_massimo} onChange={(e) => setKitDraft((c) => ({ ...c, sconto_massimo: Number(e.target.value) }))} />
          </label>
          <label className={styles.field}>
            <span>SLA ore</span>
            <input type="number" value={kitDraft.sla_resolution_hours} onChange={(e) => setKitDraft((c) => ({ ...c, sla_resolution_hours: Number(e.target.value) }))} />
          </label>
        </div>
        <div className={styles.toggleGrid}>
          {kitToggleDefs.map((item) => (
            <label key={item.key} className={styles.toggle}>
              <input type="checkbox" checked={kitDraft[item.key]} onChange={(e) => setKitDraft((c) => ({ ...c, [item.key]: e.target.checked } as KitFormState))} />
              <span>{item.label}</span>
            </label>
          ))}
        </div>
        <div className={styles.formGrid4}>
          <label className={styles.field}>
            <span>Billing period</span>
            <select value={kitDraft.billing_period} onChange={(e) => setKitDraft((c) => ({ ...c, billing_period: Number(e.target.value) }))}>
              {billingPeriodOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <label className={styles.field}>
            <span>Activation days</span>
            <input type="number" value={kitDraft.activation_time_days} onChange={(e) => setKitDraft((c) => ({ ...c, activation_time_days: Number(e.target.value) }))} />
          </label>
          <label className={styles.field}>
            <span>Initial months</span>
            <input type="number" value={kitDraft.initial_subscription_months} onChange={(e) => setKitDraft((c) => ({ ...c, initial_subscription_months: Number(e.target.value) }))} />
          </label>
          <label className={styles.field}>
            <span>Next months</span>
            <input type="number" value={kitDraft.next_subscription_months} onChange={(e) => setKitDraft((c) => ({ ...c, next_subscription_months: Number(e.target.value) }))} />
          </label>
        </div>
        <label className={styles.field}>
          <span>Sellable groups</span>
          <MultiSelect<number>
            options={(customerGroups ?? []).map((g) => ({ value: g.id, label: g.name }))}
            selected={kitDraft.sellable_group_ids}
            onChange={(values) => setKitDraft((c) => ({ ...c, sellable_group_ids: values }))}
            placeholder="Seleziona gruppi"
          />
        </label>
      </section>

      {/* ── Products ── */}
      <section id="products" className={styles.card}>
        <div className={styles.panelHeader}>
          <h3>Prodotti ({productRows.length})</h3>
          <div className={styles.panelActions}>
            <button type="button" className={styles.primaryButton} onClick={() => { setProductModalMode('create'); setProductEditingId(null); setProductModalDraft(emptyProductFormState()); setProductModalOpen(true); }}>Aggiungi</button>
            <button type="button" className={styles.secondaryButton} disabled={productEditingId == null} onClick={() => { const row = selectedProductRow; if (!row) return; setProductModalMode('edit'); setProductModalDraft(toProductFormState(row, productDrafts[row.id])); setProductModalOpen(true); }}>Modifica</button>
            <button type="button" className={styles.dangerButton} disabled={productEditingId == null} onClick={() => { if (productEditingId != null) setProductDeleteId(productEditingId); }}>Rimuovi</button>
          </div>
        </div>

          {isProductsLoading ? <Skeleton rows={6} /> : null}

          {productsError ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
                </svg>
              </div>
              <p className={styles.emptyTitle}>Impossibile caricare i prodotti del kit</p>
              <p className={styles.emptyText}>{getErrorMessage(productsError, 'Riprova tra poco.')}</p>
            </div>
          ) : productRows.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0-3-3m3 3 3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
                </svg>
              </div>
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
                    <th>Req</th>
                    <th>NRC</th>
                    <th>MRC</th>
                    <th>Pos</th>
                  </tr>
                </thead>
                <tbody>
                  {productRows.map((row, index) => {
                    const selected = productEditingId === row.id;
                    return (
                      <tr
                        key={row.id}
                        className={selected ? styles.rowSelected : ''}
                        style={{ animationDelay: `${index * 0.03}s` }}
                        onClick={() => setProductEditingId(row.id)}
                        onDoubleClick={() => {
                          setProductEditingId(row.id);
                          setProductModalMode('edit');
                          setProductModalDraft(toProductFormState(row, productDrafts[row.id]));
                          setProductModalOpen(true);
                        }}
                      >
                        <td>
                          <span className={styles.codeCell}>
                            {row.product_code}
                            <small>{resolveProductLabel(products, row)}</small>
                          </span>
                        </td>
                        <td>{row.group_name ?? '—'}</td>
                        <td>{row.minimum}</td>
                        <td>{row.maximum}</td>
                        <td>{row.required ? 'Si' : 'No'}</td>
                        <td className={styles.mono}>{formatMoney(row.nrc)}</td>
                        <td className={styles.mono}>{formatMoney(row.mrc)}</td>
                        <td>{row.position}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
      </section>

      {/* ── Content: Notes, Help URL, Translations ── */}
      <section id="content" className={styles.card}>
        <h3>Note e traduzioni</h3>
        <label className={styles.field}>
          <span>Notes</span>
          <textarea value={kitDraft.notes ?? ''} onChange={(e) => setKitDraft((c) => ({ ...c, notes: e.target.value }))} rows={3} />
        </label>
        <label className={styles.field}>
          <span>Help URL</span>
          <input value={helpUrlDraft} onChange={(e) => setHelpUrlDraft(e.target.value)} placeholder="https://" />
        </label>
        <div className={styles.translationGrid}>
          {(['it', 'en'] as const).map((language) => (
            <article key={language} className={styles.translationCard}>
              <div className={styles.translationHeader}>
                <strong>{language.toUpperCase()}</strong>
              </div>
              <label className={styles.field}>
                <span>Short</span>
                <input value={translationDrafts[language].short} onChange={(e) => setTranslationDrafts((c) => ({ ...c, [language]: { ...c[language], short: e.target.value } }))} />
              </label>
              <label className={styles.field}>
                <span>Long</span>
                <textarea value={translationDrafts[language].long} onChange={(e) => setTranslationDrafts((c) => ({ ...c, [language]: { ...c[language], long: e.target.value } }))} rows={5} />
              </label>
            </article>
          ))}
        </div>
      </section>

      {/* ── Custom Values ── */}
      <section id="custom-values" className={styles.card}>
        <div className={styles.panelHeader}>
          <h3>Valori custom ({customValueRows.length})</h3>
          <div className={styles.panelActions}>
            <button type="button" className={styles.primaryButton} onClick={() => { setCustomModalMode('create'); setCustomNewDraft(emptyCustomValueFormState()); setCustomEditingId(null); setCustomModalOpen(true); }}>Aggiungi</button>
            <button type="button" className={styles.secondaryButton} disabled={customEditingId == null} onClick={() => { const row = customValueRows.find((r) => r.id === customEditingId); if (!row) return; setCustomModalMode('edit'); setCustomNewDraft(toCustomValueInlineDraft(row)); setCustomModalOpen(true); }}>Modifica</button>
            <button type="button" className={styles.dangerButton} disabled={customEditingId == null} onClick={() => { if (customEditingId != null) setCustomDeleteId(customEditingId); }}>Rimuovi</button>
          </div>
        </div>

        {isCustomValuesLoading ? <Skeleton rows={5} /> : null}

        {customValuesError ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Impossibile caricare i valori custom</p>
            <p className={styles.emptyText}>{getErrorMessage(customValuesError, 'Riprova tra poco.')}</p>
          </div>
        ) : customValueRows.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.431.992a6.759 6.759 0 0 1 0 .255c-.007.378.138.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.248a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 0 1 0-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281Z" />
                <path d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessun valore custom</p>
            <p className={styles.emptyText}>Aggiungi il primo valore con il pulsante Aggiungi.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Key</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                {customValueRows.map((row, index) => (
                  <tr
                    key={row.id}
                    className={customEditingId === row.id ? styles.rowSelected : ''}
                    style={{ animationDelay: `${index * 0.03}s` }}
                    onClick={() => setCustomEditingId(row.id)}
                    onDoubleClick={() => { setCustomEditingId(row.id); setCustomModalMode('edit'); setCustomNewDraft(toCustomValueInlineDraft(row)); setCustomModalOpen(true); }}
                  >
                    <td><code className={styles.mono}>{row.key_name}</code></td>
                    <td><code className={styles.mono}>{stringifyJsonValue(row.value)}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Modal
        open={productModalOpen}
        onClose={() => setProductModalOpen(false)}
        title={productModalMode === 'create' ? 'Aggiungi prodotto al kit' : 'Modifica prodotto del kit'}
        wide
      >
        <div className={styles.modalBody}>
          <div className={styles.formGrid}>
            <div className={styles.field}>
              <span>Product</span>
              <SingleSelect<string>
                options={(products ?? []).map((p) => ({ value: p.code, label: `${p.code} - ${p.internal_name}` }))}
                selected={productModalDraft.product_code || null}
                onChange={(v) => setProductModalDraft((c) => ({ ...c, product_code: v ?? '' }))}
                placeholder="Cerca prodotto..."
              />
            </div>
            <label className={styles.field}>
              <span>Group</span>
              <select value={productModalDraft.group_name ?? ''} onChange={(e) => setProductModalDraft((c) => ({ ...c, group_name: e.target.value || null }))}>
                <option value="">Nessuno</option>
                {productGroupOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </label>
          </div>
          <div className={styles.formGrid3}>
            <label className={styles.field}>
              <span>Minimum</span>
              <input type="number" value={productModalDraft.minimum} onChange={(e) => setProductModalDraft((c) => ({ ...c, minimum: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>Maximum</span>
              <input type="number" value={productModalDraft.maximum} onChange={(e) => setProductModalDraft((c) => ({ ...c, maximum: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>Position</span>
              <input type="number" value={productModalDraft.position} onChange={(e) => setProductModalDraft((c) => ({ ...c, position: Number(e.target.value) }))} />
            </label>
          </div>
          <div className={styles.formGrid3}>
            <label className={styles.field}>
              <span>NRC</span>
              <input type="number" step="0.01" value={productModalDraft.nrc} onChange={(e) => setProductModalDraft((c) => ({ ...c, nrc: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input type="number" step="0.01" value={productModalDraft.mrc} onChange={(e) => setProductModalDraft((c) => ({ ...c, mrc: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>Required</span>
              <label className={styles.checkboxInline}>
                <input type="checkbox" checked={productModalDraft.required} onChange={(e) => setProductModalDraft((c) => ({ ...c, required: e.target.checked }))} />
                <span>{productModalDraft.required ? 'Si' : 'No'}</span>
              </label>
            </label>
          </div>
          <label className={styles.field}>
            <span>Notes</span>
            <textarea rows={3} value={productModalDraft.notes} onChange={(e) => setProductModalDraft((c) => ({ ...c, notes: e.target.value }))} />
          </label>
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
        open={customModalOpen}
        onClose={() => setCustomModalOpen(false)}
        title={customModalMode === 'create' ? 'Nuovo valore custom' : 'Modifica valore custom'}
      >
        <div className={styles.modalBody}>
          <label className={styles.field}>
            <span>Key name</span>
            <select value={customNewDraft.key_name} onChange={(e) => setCustomNewDraft((c) => ({ ...c, key_name: e.target.value }))}>
              <option value="">Seleziona</option>
              {customFieldKeyOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <label className={styles.field}>
            <span>Value (JSON)</span>
            <textarea value={customNewDraft.valueText} onChange={(e) => setCustomNewDraft((c) => ({ ...c, valueText: e.target.value }))} rows={6} placeholder='{"example": true}' />
          </label>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCustomModalOpen(false)}>Annulla</button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleSaveCustomModal()}>Salva</button>
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

function toProductFormState(row: KitProductItem, draft?: KitProductInlineDraft): KitProductFormState {
  const source = draft ?? toProductInlineDraft(row);
  return {
    ...source,
    notes: source.notes ?? '',
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



function toCustomValueInlineDraft(row: KitCustomValueItem): KitCustomValueInlineDraft {
  return {
    key_name: row.key_name,
    valueText: stringifyJsonValue(row.value),
  };
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

function formatMoney(value: number) {
  return new Intl.NumberFormat('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
}

function resolveProductLabel(
  products: Array<{ code: string; internal_name: string }> | undefined,
  row: KitProductItem,
) {
  const match = products?.find((product) => product.code === row.product_code);
  return match ? match.internal_name : row.product_internal_name ?? row.product_name ?? row.name ?? 'n/d';
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
