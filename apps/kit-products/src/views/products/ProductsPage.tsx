import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import {
  useAssetFlows,
  useCategories,
  useCreateProduct,
  useProducts,
  useUpdateProduct,
  useUpdateProductTranslations,
} from '../../api/queries';
import type {
  Product,
  ProductCreateRequest,
  ProductUpdateRequest,
  Translation,
} from '../../api/types';
import styles from './ProductsPage.module.css';

const emptyTranslations: Translation[] = [
  { language: 'it', short: '', long: '' },
  { language: 'en', short: '', long: '' },
];

export function ProductsPage() {
  const { toast } = useToast();
  const [selectedCode, setSelectedCode] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [newProduct, setNewProduct] = useState<ProductCreateRequest>({
    code: '',
    internal_name: '',
    category_id: 0,
    nrc: 0,
    mrc: 0,
    img_url: null,
    erp_sync: true,
    asset_flow: null,
    translations: emptyTranslations,
  });
  const [editDraft, setEditDraft] = useState<ProductUpdateRequest & { translations: Translation[] }>({
    internal_name: '',
    category_id: 0,
    nrc: 0,
    mrc: 0,
    img_url: null,
    erp_sync: true,
    asset_flow: null,
    translations: emptyTranslations,
  });
  const [originalTranslations, setOriginalTranslations] = useState<Translation[]>(emptyTranslations);

  const { data: products, isLoading, error } = useProducts();
  const { data: categories } = useCategories();
  const { data: assetFlows } = useAssetFlows();
  const createProduct = useCreateProduct();
  const updateProduct = useUpdateProduct();
  const updateTranslations = useUpdateProductTranslations();

  const selectedProduct = useMemo(
    () => products?.find((product) => product.code === selectedCode) ?? null,
    [products, selectedCode],
  );

  const filteredProducts = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q || !products) return products ?? [];
    return products.filter((p) => {
      const categoryName = categories?.find((c) => c.id === p.category_id)?.name ?? '';
      return (
        p.code.toLowerCase().includes(q) ||
        p.internal_name.toLowerCase().includes(q) ||
        categoryName.toLowerCase().includes(q) ||
        (p.asset_flow ?? '').toLowerCase().includes(q)
      );
    });
  }, [products, search, categories]);

  function openEditModal(product: Product) {
    setEditDraft({
      internal_name: product.internal_name,
      category_id: product.category_id,
      nrc: product.nrc,
      mrc: product.mrc,
      img_url: product.img_url,
      erp_sync: product.erp_sync,
      asset_flow: product.asset_flow,
      translations: ensureRequiredTranslations(product.translations),
    });
    setOriginalTranslations(ensureRequiredTranslations(product.translations));
    setSelectedCode(product.code);
    setEditOpen(true);
  }

  async function handleCreateProduct() {
    if (!newProduct.code.trim() || !newProduct.internal_name.trim() || newProduct.category_id <= 0) {
      toast('Compila codice, nome e categoria prima di creare il prodotto', 'error');
      return;
    }

    try {
      await createProduct.mutateAsync({
        ...newProduct,
        code: newProduct.code.trim(),
        internal_name: newProduct.internal_name.trim(),
        img_url: emptyToNull(newProduct.img_url),
      });
      setCreateOpen(false);
      setNewProduct({
        code: '',
        internal_name: '',
        category_id: 0,
        nrc: 0,
        mrc: 0,
        img_url: null,
        erp_sync: true,
        asset_flow: null,
        translations: emptyTranslations,
      });
      toast('Prodotto creato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile creare il prodotto'), 'error');
    }
  }

  async function handleSaveEdit() {
    if (!selectedProduct) return;
    if (editDraft.category_id <= 0) {
      toast('Seleziona una categoria valida', 'error');
      return;
    }

    try {
      await updateProduct.mutateAsync({
        code: selectedProduct.code,
        internal_name: editDraft.internal_name.trim(),
        category_id: editDraft.category_id,
        nrc: editDraft.nrc,
        mrc: editDraft.mrc,
        img_url: emptyToNull(editDraft.img_url),
        erp_sync: editDraft.erp_sync,
        asset_flow: editDraft.asset_flow,
      });

      const translationsChanged = JSON.stringify(editDraft.translations) !== JSON.stringify(originalTranslations);
      if (translationsChanged) {
        const result = await updateTranslations.mutateAsync({
          code: selectedProduct.code,
          translations: editDraft.translations,
        });
        if (result.warning) {
          toast(result.warning.message, 'warning');
        }
      }

      setEditOpen(false);
      toast(`Prodotto ${selectedProduct.code} aggiornato`, 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare il prodotto'), 'error');
    }
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={8} />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1>Catalogo prodotti</h1>
          <p className={styles.subtitle}>{products?.length ?? 0} prodotti</p>
        </div>
        <button type="button" className={styles.primaryButton} onClick={() => setCreateOpen(true)}>
          Nuovo prodotto
        </button>
      </header>

      <section className={styles.card}>
        <div className={styles.cardToolbar}>
          <div className={styles.toolbarGroup}>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={!selectedProduct}
              onClick={() => {
                if (selectedProduct) openEditModal(selectedProduct);
              }}
            >
              Modifica
            </button>
          </div>
          <div className={styles.searchWrap}>
            <svg className={styles.searchIcon} viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
              <path fillRule="evenodd" d="M9 3.5a5.5 5.5 0 1 0 0 11 5.5 5.5 0 0 0 0-11ZM2 9a7 7 0 1 1 12.452 4.391l3.328 3.329a.75.75 0 1 1-1.06 1.06l-3.329-3.328A7 7 0 0 1 2 9Z" clipRule="evenodd" />
            </svg>
            <input
              type="text"
              className={styles.searchInput}
              placeholder="Cerca prodotti..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
        </div>

        {error ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIconDanger}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Impossibile caricare i prodotti</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : filteredProducts.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0-3-3m3 3 3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>{search ? 'Nessun risultato' : 'Nessun prodotto disponibile'}</p>
            <p className={styles.emptyText}>{search ? `Nessun prodotto corrisponde a "${search}".` : 'Crea il primo prodotto per iniziare il catalogo.'}</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Codice</th>
                  <th>Nome</th>
                  <th>Categoria</th>
                  <th>Asset flow</th>
                  <th>NRC</th>
                  <th>MRC</th>
                  <th>ERP</th>
                </tr>
              </thead>
              <tbody>
                {filteredProducts.map((product, index) => {
                  const category = categories?.find((c) => c.id === product.category_id);
                  return (
                    <tr
                      key={product.code}
                      className={selectedCode === product.code ? styles.rowSelected : ''}
                      style={{ animationDelay: `${index * 0.03}s` }}
                      onClick={() => setSelectedCode(product.code)}
                      onDoubleClick={() => openEditModal(product)}
                    >
                      <td><span className={styles.code}>{product.code}</span></td>
                      <td>{product.internal_name}</td>
                      <td>
                        {category ? (
                          <span className={styles.categoryBadge}>
                            <span className={styles.categoryDot} style={{ background: category.color }} />
                            {category.name}
                          </span>
                        ) : '—'}
                      </td>
                      <td>{product.asset_flow ?? '—'}</td>
                      <td className={styles.mono}>{formatMoney(product.nrc)}</td>
                      <td className={styles.mono}>{formatMoney(product.mrc)}</td>
                      <td>{product.erp_sync ? 'On' : 'Off'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Create modal */}
      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Nuovo prodotto" wide>
        <div className={styles.modalForm}>
          <label className={styles.field}>
            <span>Codice</span>
            <input
              value={newProduct.code}
              maxLength={25}
              onChange={(e) => setNewProduct((c) => ({ ...c, code: e.target.value }))}
            />
          </label>
          <label className={styles.field}>
            <span>Nome</span>
            <input
              value={newProduct.internal_name}
              onChange={(e) => setNewProduct((c) => ({ ...c, internal_name: e.target.value }))}
            />
          </label>
          <div className={styles.formRow}>
            <label className={styles.field}>
              <span className={styles.labelWithDot}>
                Categoria
                {(() => { const c = categories?.find((x) => x.id === newProduct.category_id); return c ? <span className={styles.categoryDot} style={{ background: c.color }} /> : null; })()}
              </span>
              <select
                value={newProduct.category_id}
                onChange={(e) => setNewProduct((c) => ({ ...c, category_id: Number(e.target.value) }))}
              >
                <option value={0}>Seleziona</option>
                {categories?.map((cat) => (
                  <option key={cat.id} value={cat.id}>{cat.name}</option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span>Asset flow</span>
              <select
                value={newProduct.asset_flow ?? ''}
                onChange={(e) => setNewProduct((c) => ({ ...c, asset_flow: e.target.value || null }))}
              >
                <option value="">Nessuno</option>
                {assetFlows?.map((flow) => (
                  <option key={flow.name} value={flow.name}>{flow.label}</option>
                ))}
              </select>
            </label>
          </div>
          <div className={styles.formRow}>
            <label className={styles.field}>
              <span>NRC</span>
              <input type="number" step="0.01" value={newProduct.nrc} onChange={(e) => setNewProduct((c) => ({ ...c, nrc: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input type="number" step="0.01" value={newProduct.mrc} onChange={(e) => setNewProduct((c) => ({ ...c, mrc: Number(e.target.value) }))} />
            </label>
          </div>
          <div className={styles.formRowWide}>
            <label className={styles.field}>
              <span>Img URL</span>
              <input value={newProduct.img_url ?? ''} onChange={(e) => setNewProduct((c) => ({ ...c, img_url: e.target.value || null }))} />
            </label>
            <div className={styles.field}>
              <span>ERP</span>
              <div className={styles.toggleWrap}>
                <ToggleSwitch id="create-erp" checked={newProduct.erp_sync} onChange={(v) => setNewProduct((c) => ({ ...c, erp_sync: v }))} />
              </div>
            </div>
          </div>
          <TranslationFields
            translations={newProduct.translations}
            onChange={(translations) => setNewProduct((c) => ({ ...c, translations }))}
          />
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCreateOpen(false)}>Annulla</button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleCreateProduct()} disabled={createProduct.isPending}>Crea prodotto</button>
          </div>
        </div>
      </Modal>

      {/* Edit modal */}
      <Modal open={editOpen} onClose={() => setEditOpen(false)} title={`Modifica ${selectedProduct?.code ?? ''}`} wide>
        <div className={styles.modalForm}>
          <label className={styles.field}>
            <span>Nome</span>
            <input value={editDraft.internal_name} onChange={(e) => setEditDraft((c) => ({ ...c, internal_name: e.target.value }))} />
          </label>
          <div className={styles.formRow}>
            <label className={styles.field}>
              <span className={styles.labelWithDot}>
                Categoria
                {(() => { const c = categories?.find((x) => x.id === editDraft.category_id); return c ? <span className={styles.categoryDot} style={{ background: c.color }} /> : null; })()}
              </span>
              <select value={editDraft.category_id} onChange={(e) => setEditDraft((c) => ({ ...c, category_id: Number(e.target.value) }))}>
                <option value={0}>Seleziona</option>
                {categories?.map((cat) => (
                  <option key={cat.id} value={cat.id}>{cat.name}</option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span>Asset flow</span>
              <select value={editDraft.asset_flow ?? ''} onChange={(e) => setEditDraft((c) => ({ ...c, asset_flow: e.target.value || null }))}>
                <option value="">Nessuno</option>
                {assetFlows?.map((flow) => (
                  <option key={flow.name} value={flow.name}>{flow.label}</option>
                ))}
              </select>
            </label>
          </div>
          <div className={styles.formRow}>
            <label className={styles.field}>
              <span>NRC</span>
              <input type="number" step="0.01" value={editDraft.nrc} onChange={(e) => setEditDraft((c) => ({ ...c, nrc: Number(e.target.value) }))} />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input type="number" step="0.01" value={editDraft.mrc} onChange={(e) => setEditDraft((c) => ({ ...c, mrc: Number(e.target.value) }))} />
            </label>
          </div>
          <div className={styles.formRowWide}>
            <label className={styles.field}>
              <span>Img URL</span>
              <input value={editDraft.img_url ?? ''} onChange={(e) => setEditDraft((c) => ({ ...c, img_url: e.target.value || null }))} />
            </label>
            <div className={styles.field}>
              <span>ERP</span>
              <div className={styles.toggleWrap}>
                <ToggleSwitch id="edit-erp" checked={editDraft.erp_sync} onChange={(v) => setEditDraft((c) => ({ ...c, erp_sync: v }))} />
              </div>
            </div>
          </div>
          <TranslationFields
            translations={editDraft.translations}
            onChange={(translations) => setEditDraft((c) => ({ ...c, translations }))}
          />
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setEditOpen(false)}>Annulla</button>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => void handleSaveEdit()}
              disabled={updateProduct.isPending || updateTranslations.isPending}
            >
              Salva
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function TranslationFields({
  translations,
  onChange,
}: {
  translations: Translation[];
  onChange: (translations: Translation[]) => void;
}) {
  return (
    <div className={styles.translationBlock}>
      <h3>Descrizioni</h3>
      <div className={styles.translationGrid}>
        {(['it', 'en'] as const).map((language) => {
          const translation = translations.find((item) => item.language === language) ?? {
            language,
            short: '',
            long: '',
          };

          return (
            <div key={language} className={styles.translationCard}>
              <span className={styles.translationLabel}>{language.toUpperCase()}</span>
              <label className={styles.field}>
                <span>Short</span>
                <input
                  value={translation.short}
                  onChange={(event) =>
                    onChange(updateTranslationsArray(translations, language, {
                      ...translation,
                      short: event.target.value,
                    }))
                  }
                />
              </label>
              <label className={styles.field}>
                <span>Long</span>
                <textarea
                  value={translation.long}
                  onChange={(event) =>
                    onChange(updateTranslationsArray(translations, language, {
                      ...translation,
                      long: event.target.value,
                    }))
                  }
                />
              </label>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function updateTranslationsArray(translations: Translation[], language: 'it' | 'en', next: Translation) {
  const map = new Map(translations.map((translation) => [translation.language, translation]));
  map.set(language, next);
  return ensureRequiredTranslations(Array.from(map.values()));
}

function ensureRequiredTranslations(translations: Translation[]) {
  const map = new Map(
    translations.map((translation) => [
      translation.language,
      { ...translation },
    ]),
  );
  return [
    map.get('it') ?? { language: 'it', short: '', long: '' },
    map.get('en') ?? { language: 'en', short: '', long: '' },
  ];
}

function formatMoney(value: number) {
  return new Intl.NumberFormat('it-IT', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function emptyToNull(value: string | null) {
  if (value == null) return null;
  const trimmed = value.trim();
  return trimmed === '' ? null : trimmed;
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') return message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}
