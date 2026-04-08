import { useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, useToast } from '@mrsmith/ui';
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

type ProductDrafts = Record<string, ProductUpdateRequest>;

const emptyTranslations: Translation[] = [
  { language: 'it', short: '', long: '' },
  { language: 'en', short: '', long: '' },
];

export function ProductsPage() {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<ProductDrafts>({});
  const [selectedCode, setSelectedCode] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [descriptionOpen, setDescriptionOpen] = useState(false);
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
  const [translationDrafts, setTranslationDrafts] = useState<Record<string, Translation[]>>({});

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

  async function handleSaveRow(product: Product) {
    const draft = drafts[product.code];
    if (!draft) {
      return;
    }
    if (draft.category_id <= 0) {
      toast('Seleziona una categoria valida prima di salvare il prodotto', 'error');
      return;
    }

    try {
      await updateProduct.mutateAsync({
        code: product.code,
        ...draft,
        internal_name: draft.internal_name.trim(),
        img_url: emptyToNull(draft.img_url),
      });
      setDrafts((current) => {
        const next = { ...current };
        delete next[product.code];
        return next;
      });
      toast(`Prodotto ${product.code} aggiornato`, 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare il prodotto'), 'error');
    }
  }

  async function handleSaveDescriptions() {
    if (!selectedProduct) {
      return;
    }

    const translations = translationDrafts[selectedProduct.code] ?? selectedProduct.translations;
    try {
      const result = await updateTranslations.mutateAsync({
        code: selectedProduct.code,
        translations,
      });
      setDescriptionOpen(false);
      if (result.warning) {
        toast(result.warning.message, 'warning');
      } else {
        toast('Descrizioni aggiornate', 'success');
      }
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare le descrizioni'), 'error');
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
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Phase 3</p>
          <h1>Catalogo prodotti</h1>
          <p className={styles.lead}>
            Gestisci il master prodotti con traduzioni IT/EN, asset flow e sincronizzazione ERP best effort.
          </p>
        </div>
        <div className={styles.toolbar}>
          <button
            type="button"
            className={styles.primaryButton}
            onClick={() => setCreateOpen(true)}
          >
            Nuovo prodotto
          </button>
          <button
            type="button"
            className={styles.secondaryButton}
            disabled={!selectedProduct}
            onClick={() => {
              if (!selectedProduct) {
                return;
              }
              setTranslationDrafts((current) => ({
                ...current,
                [selectedProduct.code]: selectedProduct.translations.length > 0
                  ? selectedProduct.translations
                  : emptyTranslations,
              }));
              setDescriptionOpen(true);
            }}
          >
            Edit descriptions
          </button>
        </div>
      </header>

      <section className={styles.card}>
        {error ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Impossibile caricare i prodotti</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : !products || products.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nessun prodotto disponibile</p>
            <p className={styles.emptyText}>Crea il primo prodotto per iniziare il catalogo.</p>
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
                  <th>Img URL</th>
                  <th className={styles.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {products.map((product) => {
                  const draft = drafts[product.code] ?? toProductDraft(product);
                  const dirty = isProductDraftDirty(product, draft);

                  return (
                    <tr
                      key={product.code}
                      className={selectedCode === product.code ? styles.rowSelected : ''}
                      onClick={() => setSelectedCode(product.code)}
                    >
                      <td>
                        <span className={styles.code}>{product.code}</span>
                      </td>
                      <td>
                        <input
                          value={draft.internal_name}
                          onChange={(event) => updateDraft(product, setDrafts, { internal_name: event.target.value })}
                        />
                      </td>
                      <td>
                        <select
                          value={draft.category_id}
                          onChange={(event) => updateDraft(product, setDrafts, { category_id: Number(event.target.value) })}
                        >
                          <option value={0}>Seleziona</option>
                          {categories?.map((category) => (
                            <option key={category.id} value={category.id}>
                              {category.name}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td>
                        <select
                          value={draft.asset_flow ?? ''}
                          onChange={(event) => updateDraft(product, setDrafts, { asset_flow: event.target.value || null })}
                        >
                          <option value="">Nessuno</option>
                          {assetFlows?.map((flow) => (
                            <option key={flow.name} value={flow.name}>
                              {flow.label}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.01"
                          value={draft.nrc}
                          onChange={(event) => updateDraft(product, setDrafts, { nrc: Number(event.target.value) })}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.01"
                          value={draft.mrc}
                          onChange={(event) => updateDraft(product, setDrafts, { mrc: Number(event.target.value) })}
                        />
                      </td>
                      <td>
                        <label className={styles.checkboxInline}>
                          <input
                            type="checkbox"
                            checked={draft.erp_sync}
                            onChange={(event) => updateDraft(product, setDrafts, { erp_sync: event.target.checked })}
                          />
                          <span>{draft.erp_sync ? 'On' : 'Off'}</span>
                        </label>
                      </td>
                      <td>
                        <input
                          value={draft.img_url ?? ''}
                          onChange={(event) => updateDraft(product, setDrafts, { img_url: event.target.value || null })}
                          placeholder="https://..."
                        />
                      </td>
                      <td className={styles.actionsCell}>
                        <button
                          type="button"
                          className={styles.secondaryButton}
                          disabled={!dirty || updateProduct.isPending}
                          onClick={(event) => {
                            event.stopPropagation();
                            void handleSaveRow(product);
                          }}
                        >
                          Salva
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

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Nuovo prodotto">
        <div className={styles.modalForm}>
          <label className={styles.field}>
            <span>Codice</span>
            <input
              value={newProduct.code}
              maxLength={25}
              onChange={(event) => setNewProduct((current) => ({ ...current, code: event.target.value }))}
            />
          </label>
          <label className={styles.field}>
            <span>Nome</span>
            <input
              value={newProduct.internal_name}
              onChange={(event) => setNewProduct((current) => ({ ...current, internal_name: event.target.value }))}
            />
          </label>
          <label className={styles.field}>
            <span>Categoria</span>
            <select
              value={newProduct.category_id}
              onChange={(event) => setNewProduct((current) => ({ ...current, category_id: Number(event.target.value) }))}
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
            <span>Asset flow</span>
            <select
              value={newProduct.asset_flow ?? ''}
              onChange={(event) => setNewProduct((current) => ({ ...current, asset_flow: event.target.value || null }))}
            >
              <option value="">Nessuno</option>
              {assetFlows?.map((flow) => (
                <option key={flow.name} value={flow.name}>
                  {flow.label}
                </option>
              ))}
            </select>
          </label>
          <div className={styles.numberGrid}>
            <label className={styles.field}>
              <span>NRC</span>
              <input
                type="number"
                step="0.01"
                value={newProduct.nrc}
                onChange={(event) => setNewProduct((current) => ({ ...current, nrc: Number(event.target.value) }))}
              />
            </label>
            <label className={styles.field}>
              <span>MRC</span>
              <input
                type="number"
                step="0.01"
                value={newProduct.mrc}
                onChange={(event) => setNewProduct((current) => ({ ...current, mrc: Number(event.target.value) }))}
              />
            </label>
          </div>
          <label className={styles.field}>
            <span>Img URL</span>
            <input
              value={newProduct.img_url ?? ''}
              onChange={(event) => setNewProduct((current) => ({ ...current, img_url: event.target.value || null }))}
            />
          </label>
          <label className={styles.checkboxInline}>
            <input
              type="checkbox"
              checked={newProduct.erp_sync}
              onChange={(event) => setNewProduct((current) => ({ ...current, erp_sync: event.target.checked }))}
            />
            <span>ERP sync abilitato</span>
          </label>
          <TranslationFields
            title="Descrizioni iniziali"
            translations={newProduct.translations}
            onChange={(translations) => setNewProduct((current) => ({ ...current, translations }))}
          />
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setCreateOpen(false)}>
              Annulla
            </button>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => void handleCreateProduct()}
              disabled={createProduct.isPending}
            >
              Crea prodotto
            </button>
          </div>
        </div>
      </Modal>

      <Modal open={descriptionOpen} onClose={() => setDescriptionOpen(false)} title="Edit descriptions">
        {selectedProduct ? (
          <div className={styles.modalForm}>
            <p className={styles.modalLead}>
              Aggiorni le descrizioni IT/EN di <strong>{selectedProduct.code}</strong>. Le short description vengono sincronizzate verso Alyante solo in questa operazione.
            </p>
            <TranslationFields
              title="Descrizioni"
              translations={translationDrafts[selectedProduct.code] ?? selectedProduct.translations}
              onChange={(translations) => {
                setTranslationDrafts((current) => ({
                  ...current,
                  [selectedProduct.code]: translations,
                }));
              }}
            />
            <div className={styles.modalActions}>
              <button type="button" className={styles.secondaryButton} onClick={() => setDescriptionOpen(false)}>
                Annulla
              </button>
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => void handleSaveDescriptions()}
                disabled={updateTranslations.isPending}
              >
                Salva descrizioni
              </button>
            </div>
          </div>
        ) : null}
      </Modal>
    </section>
  );
}

function TranslationFields({
  title,
  translations,
  onChange,
}: {
  title: string;
  translations: Translation[];
  onChange: (translations: Translation[]) => void;
}) {
  return (
    <div className={styles.translationBlock}>
      <h3>{title}</h3>
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
  return [
    map.get('it') ?? { language: 'it', short: '', long: '' },
    map.get('en') ?? { language: 'en', short: '', long: '' },
  ];
}

function toProductDraft(product: Product): ProductUpdateRequest {
  return {
    internal_name: product.internal_name,
    category_id: product.category_id,
    nrc: product.nrc,
    mrc: product.mrc,
    img_url: product.img_url,
    erp_sync: product.erp_sync,
    asset_flow: product.asset_flow,
  };
}

function updateDraft(
  product: Product,
  setDrafts: Dispatch<SetStateAction<ProductDrafts>>,
  update: Partial<ProductUpdateRequest>,
) {
  setDrafts((current) => ({
    ...current,
    [product.code]: {
      ...toProductDraft(product),
      ...current[product.code],
      ...update,
    },
  }));
}

function isProductDraftDirty(product: Product, draft: ProductUpdateRequest) {
  return JSON.stringify(toProductDraft(product)) !== JSON.stringify(draft);
}

function emptyToNull(value: string | null) {
  if (value == null) {
    return null;
  }
  const trimmed = value.trim();
  return trimmed === '' ? null : trimmed;
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
