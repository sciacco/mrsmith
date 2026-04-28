import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, SearchInput, Skeleton, TabNav, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Outlet } from 'react-router-dom';
import {
  useArticleCategories,
  useCategories,
  useDocumentTypes,
  useFornitoriMutations,
  usePaymentMethods,
} from '../api/queries';
import type { ArticleCategory, Category, DocumentType } from '../api/types';

const settingsNavItems = [
  { label: 'Qualifica', path: '/impostazioni/qualifica' },
  { label: 'Tipi documento', path: '/impostazioni/tipi-documento' },
  { label: 'Pagamenti RDA', path: '/impostazioni/pagamenti-rda' },
  { label: 'Articoli-categorie', path: '/impostazioni/articoli-categorie' },
];

type CategorySelection = number | 'new' | null;
type DocumentTypeSelection = number | 'new' | null;
type DocumentRuleState = 'none' | 'optional' | 'required';

const documentRuleOptions: Array<{ value: DocumentRuleState; label: string; ariaLabel: string }> = [
  { value: 'none', label: 'Nessuno', ariaLabel: 'non assegnato' },
  { value: 'optional', label: 'Facoltativo', ariaLabel: 'facoltativo' },
  { value: 'required', label: 'Obbligatorio', ariaLabel: 'obbligatorio' },
];

export function SettingsLayout() {
  return (
    <main className="page settingsPage">
      <header className="settingsHeader">
        <div>
          <h1>Impostazioni</h1>
          <p>Regole di qualifica, tipi documento, pagamenti RDA e associazioni articolo-categoria.</p>
        </div>
      </header>
      <div className="settingsSubnav">
        <TabNav items={settingsNavItems} />
      </div>
      <div className="settingsContent">
        <Outlet />
      </div>
    </main>
  );
}

export function QualificationSettingsPage() {
  const { toast } = useToast();
  const categories = useCategories();
  const documentTypes = useDocumentTypes();
  const mutations = useFornitoriMutations();
  const [selectedCategoryId, setSelectedCategoryId] = useState<CategorySelection>(null);
  const [categoryName, setCategoryName] = useState('');
  const [required, setRequired] = useState<number[]>([]);
  const [optional, setOptional] = useState<number[]>([]);
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);

  const categoryItems = useMemo(
    () => [...(categories.data ?? [])].sort(compareCategoryByName),
    [categories.data],
  );
  const selectedCategory = selectedCategoryId === 'new'
    ? null
    : categoryItems.find((item) => item.id === selectedCategoryId) ?? null;
  const creatingCategory = selectedCategoryId === 'new' || (!categories.isLoading && !categories.error && categoryItems.length === 0);
  const documentTypeItems = useMemo(
    () => [...(documentTypes.data ?? [])].sort(compareDocumentTypeByName),
    [documentTypes.data],
  );

  useEffect(() => {
    if (selectedCategoryId !== null) return;
    const first = categoryItems[0];
    if (first) loadCategory(first);
  }, [categoryItems, selectedCategoryId]);

  function loadCategory(item: Category) {
    const draft = categoryDraft(item);
    setSelectedCategoryId(item.id);
    setCategoryName(item.name);
    setRequired(draft.required);
    setOptional(draft.optional);
    setConfirmDeleteOpen(false);
  }

  function startNewCategory() {
    setSelectedCategoryId('new');
    setCategoryName('');
    setRequired([]);
    setOptional([]);
    setConfirmDeleteOpen(false);
  }

  function setDocumentRule(documentTypeId: number, rule: DocumentRuleState) {
    setRequired((current) => {
      const withoutDocumentType = current.filter((id) => id !== documentTypeId);
      return rule === 'required' ? [...withoutDocumentType, documentTypeId] : withoutDocumentType;
    });
    setOptional((current) => {
      const withoutDocumentType = current.filter((id) => id !== documentTypeId);
      return rule === 'optional' ? [...withoutDocumentType, documentTypeId] : withoutDocumentType;
    });
  }

  async function saveCategory(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = categoryName.trim();
    if (!name) {
      toast('Inserisci il nome categoria', 'warning');
      return;
    }

    const overlap = required.find((id) => optional.includes(id));
    if (overlap) {
      toast('Lo stesso tipo documento non puo essere obbligatorio e facoltativo', 'warning');
      return;
    }

    const uniqueRequired = [...new Set(required)];
    const uniqueOptional = [...new Set(optional)];
    const body = {
      name,
      document_types: [
        ...uniqueRequired.map((id) => ({ id, required: true })),
        ...uniqueOptional.map((id) => ({ id, required: false })),
      ],
    };

    try {
      const saved = selectedCategory && !creatingCategory
        ? await mutations.updateCategory.mutateAsync({ id: selectedCategory.id, body })
        : await mutations.createCategory.mutateAsync(body);
      setSelectedCategoryId(saved.id);
      setCategoryName(name);
      setRequired(uniqueRequired);
      setOptional(uniqueOptional);
      toast('Categoria salvata', 'success');
    } catch (error) {
      toast(apiErrorMessage(error, 'Impossibile salvare la categoria.'), 'error');
    }
  }

  async function deleteCategory() {
    if (!selectedCategory) return;
    try {
      await mutations.deleteCategory.mutateAsync(selectedCategory.id);
      setConfirmDeleteOpen(false);
      startNewCategory();
      toast('Categoria eliminata', 'success');
    } catch (error) {
      toast(apiErrorMessage(error, 'Impossibile eliminare la categoria.'), 'error');
    }
  }

  return (
    <section className="settingsSection" aria-label="Qualifica">
      <div className="settingsWorkspace">
        <section className="master panel settingsMasterPanel" aria-label="Categorie qualifica">
          <div className="settingsPanelHeader">
            <div>
              <h2>Categorie qualifica</h2>
              <span className="settingsPanelMeta">{countLabel(categoryItems.length, 'categoria', 'categorie')}</span>
            </div>
            <Button size="sm" leftIcon={<Icon name="plus" />} onClick={startNewCategory}>
              Nuova categoria
            </Button>
          </div>

          {categories.isLoading ? (
            <div className="settingsPanelBody">
              <Skeleton rows={8} />
            </div>
          ) : categories.error ? (
            stateBlock(errorTitle(categories.error), 'Le categorie non possono essere caricate.', 'triangle-alert')
          ) : categoryItems.length === 0 ? (
            stateBlock('Nessuna categoria', 'Crea una categoria per definire i documenti richiesti.', 'file-plus')
          ) : (
            <div className="settingsList">
              {categoryItems.map((item) => {
                const counts = categoryDocumentCounts(item);
                const selected = selectedCategoryId === item.id;
                return (
                  <button
                    key={item.id}
                    type="button"
                    className={`settingsListRow settingsCategoryRow ${selected ? 'selected' : ''}`}
                    aria-current={selected ? 'true' : undefined}
                    onClick={() => loadCategory(item)}
                  >
                    <span className="settingsListMain settingsCategoryMain">
                      <span className="settingsCategoryName">{item.name}</span>
                      <span className="settingsDocCounts">
                        {counts.required === 0 && counts.optional === 0 ? (
                          <span className="settingsDocBadge settingsDocBadge--empty">Nessuna regola documentale</span>
                        ) : (
                          <>
                            {counts.required > 0 && (
                              <span className="settingsDocBadge settingsDocBadge--required">
                                <Icon name="file-warning" size={11} />
                                {counts.required} obbligator{counts.required === 1 ? 'io' : 'i'}
                              </span>
                            )}
                            {counts.optional > 0 && (
                              <span className="settingsDocBadge settingsDocBadge--optional">
                                {counts.optional} facoltativ{counts.optional === 1 ? 'o' : 'i'}
                              </span>
                            )}
                          </>
                        )}
                      </span>
                    </span>
                    <Icon name="chevron-right" size={16} />
                  </button>
                );
              })}
            </div>
          )}
        </section>

        <section className="detail panel settingsDetailPanel" aria-label="Regola categoria">
          <div className="settingsDetailHeader">
            <div>
              <h2>{creatingCategory ? 'Nuova categoria' : selectedCategory?.name ?? 'Categoria'}</h2>
              <span className="settingsPanelMeta">Regola documentale</span>
            </div>
          </div>

          {!creatingCategory && !selectedCategory ? (
            stateBlock('Seleziona una categoria', 'Scegli una categoria dalla lista.', 'file-text')
          ) : (
            <form className="settingsDetailForm" onSubmit={(event) => void saveCategory(event)}>
              <label className="field">
                <span>Nome categoria</span>
                <input value={categoryName} onChange={(event) => setCategoryName(event.target.value)} />
              </label>
              <DocumentRuleMatrix
                documentTypes={documentTypeItems}
                loading={documentTypes.isLoading}
                error={documentTypes.error}
                required={required}
                optional={optional}
                onChange={setDocumentRule}
              />
              <div className="settingsFormActions">
                {selectedCategory ? (
                  <Button
                    type="button"
                    variant="danger"
                    leftIcon={<Icon name="trash" />}
                    onClick={() => setConfirmDeleteOpen(true)}
                  >
                    Elimina
                  </Button>
                ) : null}
                <Button
                  type="submit"
                  leftIcon={<Icon name="check" />}
                  loading={mutations.createCategory.isPending || mutations.updateCategory.isPending}
                  disabled={documentTypes.isLoading || Boolean(documentTypes.error)}
                >
                  Salva
                </Button>
              </div>
            </form>
          )}
        </section>
      </div>

      <Modal open={confirmDeleteOpen && Boolean(selectedCategory)} onClose={() => setConfirmDeleteOpen(false)} title="Elimina categoria" size="sm">
        <div className="settingsConfirm">
          <p>La categoria selezionata verra rimossa dalle regole di qualifica.</p>
          <div className="modalActions">
            <Button variant="secondary" onClick={() => setConfirmDeleteOpen(false)}>Annulla</Button>
            <Button
              variant="danger"
              leftIcon={<Icon name="trash" />}
              loading={mutations.deleteCategory.isPending}
              onClick={() => void deleteCategory()}
            >
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function DocumentRuleMatrix({
  documentTypes,
  loading,
  error,
  required,
  optional,
  onChange,
}: {
  documentTypes: DocumentType[];
  loading: boolean;
  error: unknown;
  required: number[];
  optional: number[];
  onChange: (documentTypeId: number, rule: DocumentRuleState) => void;
}) {
  const assignedDocumentIds = new Set([...required, ...optional]);
  const unassignedCount = Math.max(documentTypes.length - assignedDocumentIds.size, 0);

  function currentRule(documentTypeId: number): DocumentRuleState {
    if (required.includes(documentTypeId)) return 'required';
    if (optional.includes(documentTypeId)) return 'optional';
    return 'none';
  }

  return (
    <section className="settingsDocumentRuleMatrix" aria-label="Regola documentale">
      <div className="settingsDocumentRuleHeader">
        <span className="fieldLabel">Documenti categoria</span>
        <div className="settingsDocumentRuleSummary" aria-label="Riepilogo regola documentale">
          <span className="settingsDocBadge settingsDocBadge--required">
            <Icon name="file-warning" size={11} />
            {required.length} obbligator{required.length === 1 ? 'io' : 'i'}
          </span>
          <span className="settingsDocBadge settingsDocBadge--optional">
            {optional.length} facoltativ{optional.length === 1 ? 'o' : 'i'}
          </span>
          <span className="settingsDocBadge settingsDocBadge--empty">
            {unassignedCount} non assegnat{unassignedCount === 1 ? 'o' : 'i'}
          </span>
        </div>
      </div>

      {loading ? (
        <div className="settingsPanelBody">
          <Skeleton rows={6} />
        </div>
      ) : error ? (
        stateBlock(errorTitle(error), 'I tipi documento non possono essere caricati.', 'triangle-alert')
      ) : documentTypes.length === 0 ? (
        stateBlock('Nessun tipo documento', 'Crea i tipi documento nella tab Tipi documento per definire le regole.', 'file-plus')
      ) : (
        <div className="settingsDocumentRuleRows">
          {documentTypes.map((documentType) => {
            const rule = currentRule(documentType.id);
            return (
              <div key={documentType.id} className={`settingsDocumentRuleRow settingsDocumentRuleRow--${rule}`}>
                <span className="settingsDocumentRuleName">{documentType.name}</span>
                <div className="settingsRuleSegment" aria-label={`Regola per ${documentType.name}`}>
                  {documentRuleOptions.map((option) => {
                    const active = rule === option.value;
                    return (
                      <button
                        key={option.value}
                        type="button"
                        className={`settingsRuleSegmentButton settingsRuleSegmentButton--${option.value} ${active ? 'active' : ''}`}
                        aria-pressed={active}
                        aria-label={`Imposta ${documentType.name} come ${option.ariaLabel}`}
                        onClick={() => onChange(documentType.id, option.value)}
                      >
                        {option.value === 'required' ? <Icon name="file-warning" size={12} /> : null}
                        <span>{option.label}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

export function PaymentMethodsPage() {
  const { toast } = useToast();
  const methods = usePaymentMethods();
  const mutations = useFornitoriMutations();
  const [query, setQuery] = useState('');
  const [pendingCode, setPendingCode] = useState<string | null>(null);

  const methodItems = methods.data ?? [];
  const filteredMethods = useMemo(
    () => methodItems.filter((item) => matchesPaymentMethodQuery(item.code, item.description, query)),
    [methodItems, query],
  );

  async function toggle(code: string, checked: boolean) {
    setPendingCode(code);
    try {
      await mutations.setPaymentRda.mutateAsync({ code, rda_available: checked });
      toast('Metodo di pagamento aggiornato', 'success');
    } catch (error) {
      toast(apiErrorMessage(error, 'Impossibile aggiornare il metodo di pagamento.'), 'error');
    } finally {
      setPendingCode(null);
    }
  }

  return (
    <section className="settingsSection" aria-label="Pagamenti RDA">
      <section className="panel settingsTablePanel">
        <div className="settingsPanelHeader settingsPanelHeader--toolbar">
          <div>
            <h2>Pagamenti RDA</h2>
            <span className="settingsPanelMeta">{countLabel(methodItems.length, 'metodo', 'metodi')}</span>
          </div>
          <SearchInput value={query} onChange={setQuery} placeholder="Cerca codice o descrizione" className="settingsSearch" />
        </div>

        {methods.isLoading ? (
          <div className="settingsPanelBody">
            <Skeleton rows={8} />
          </div>
        ) : methods.error ? (
          stateBlock(errorTitle(methods.error), 'I metodi di pagamento non possono essere caricati.', 'triangle-alert')
        ) : methodItems.length === 0 ? (
          stateBlock('Nessun metodo di pagamento', 'Non ci sono metodi disponibili.', 'file-text')
        ) : filteredMethods.length === 0 ? (
          stateBlock('Nessun risultato', 'La ricerca non corrisponde ad alcun metodo di pagamento.', 'search')
        ) : (
          <div className="tableScroll">
            <table className="table settingsTable">
              <thead>
                <tr>
                  <th>Codice</th>
                  <th>Descrizione</th>
                  <th>Disponibile per RDA</th>
                </tr>
              </thead>
              <tbody>
                {filteredMethods.map((item) => (
                  <tr key={item.code}>
                    <td className="monoCell">{item.code}</td>
                    <td>{item.description}</td>
                    <td>
                      <ToggleSwitch
                        id={`rda-${item.code}`}
                        checked={Boolean(item.rda_available)}
                        disabled={pendingCode === item.code}
                        aria-label={`Disponibile per RDA ${item.code}`}
                        onChange={(checked) => void toggle(item.code, checked)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
}

export function ArticleCategoriesPage() {
  const { toast } = useToast();
  const articles = useArticleCategories();
  const categories = useCategories();
  const mutations = useFornitoriMutations();
  const [query, setQuery] = useState('');
  const [selectedArticleCode, setSelectedArticleCode] = useState<string | null>(null);
  const [categoryDraft, setCategoryDraft] = useState('');
  const [categoryBaseline, setCategoryBaseline] = useState('');

  const articleItems = articles.data ?? [];
  const filteredArticles = useMemo(
    () => articleItems.filter((item) => matchesArticleQuery(item, query)),
    [articleItems, query],
  );
  const selectedArticle = selectedArticleCode
    ? articleItems.find((item) => item.article_code === selectedArticleCode) ?? null
    : null;
  const categoryOptions = useMemo(
    () => categorySelectOptions(categories.data ?? [], selectedArticle),
    [categories.data, selectedArticle],
  );
  const dirty = Boolean(selectedArticle) && categoryDraft !== categoryBaseline;

  useEffect(() => {
    if (articles.isLoading || articles.error) return;
    if (filteredArticles.length === 0) {
      setSelectedArticleCode(null);
      return;
    }
    if (!selectedArticleCode || !filteredArticles.some((item) => item.article_code === selectedArticleCode)) {
      setSelectedArticleCode(filteredArticles[0]?.article_code ?? null);
    }
  }, [articles.error, articles.isLoading, filteredArticles, selectedArticleCode]);

  useEffect(() => {
    if (!selectedArticle) {
      setCategoryDraft('');
      setCategoryBaseline('');
      return;
    }
    const next = String(selectedArticle.category_id);
    setCategoryDraft(next);
    setCategoryBaseline(next);
  }, [selectedArticle?.article_code, selectedArticle?.category_id, selectedArticle]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedArticle || !categoryDraft) return;
    const categoryId = Number(categoryDraft);
    if (!Number.isInteger(categoryId) || categoryId <= 0) {
      toast('Seleziona una categoria valida', 'warning');
      return;
    }

    try {
      await mutations.setArticleCategory.mutateAsync({ articleCode: selectedArticle.article_code, categoryId });
      setCategoryBaseline(categoryDraft);
      toast('Associazione aggiornata', 'success');
    } catch (error) {
      toast(apiErrorMessage(error, "Impossibile aggiornare l'associazione."), 'error');
    }
  }

  return (
    <section className="settingsSection" aria-label="Articoli-categorie">
      <div className="settingsWorkspace">
        <section className="master panel settingsMasterPanel" aria-label="Articoli associati">
          <div className="settingsPanelHeader settingsPanelHeader--toolbar">
            <div>
              <h2>Articoli-categorie</h2>
              <span className="settingsPanelMeta">{countLabel(articleItems.length, 'articolo', 'articoli')}</span>
            </div>
            <SearchInput value={query} onChange={setQuery} placeholder="Cerca articolo o categoria" className="settingsSearch" />
          </div>

          {articles.isLoading ? (
            <div className="settingsPanelBody">
              <Skeleton rows={8} />
            </div>
          ) : articles.error ? (
            stateBlock(errorTitle(articles.error), 'Gli articoli non possono essere caricati.', 'triangle-alert')
          ) : articleItems.length === 0 ? (
            stateBlock('Nessun articolo associato', 'Non ci sono associazioni articolo-categoria.', 'package')
          ) : filteredArticles.length === 0 ? (
            stateBlock('Nessun risultato', 'La ricerca non corrisponde ad alcuna associazione.', 'search')
          ) : (
            <div className="tableScroll settingsMappedRows">
              <table className="table settingsTable">
                <thead>
                  <tr>
                    <th>Articolo</th>
                    <th>Categoria</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredArticles.map((item) => {
                    const selected = selectedArticleCode === item.article_code;
                    return (
                      <tr
                        key={item.article_code}
                        className={selected ? 'selectedRow' : ''}
                        onClick={() => setSelectedArticleCode(item.article_code)}
                      >
                        <td>
                          <span className="monoCell">{item.article_code}</span>
                          <small>{item.description || '-'}</small>
                        </td>
                        <td>{item.category_name || '-'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="detail panel settingsDetailPanel settingsArticleDetail" aria-label="Associazione articolo-categoria">
          <div className="settingsDetailHeader">
            <div>
              <h2>Associazione articolo-categoria</h2>
              <span className="settingsPanelMeta">{selectedArticle?.article_code ?? 'Nessun articolo'}</span>
            </div>
          </div>

          {!selectedArticle ? (
            stateBlock('Seleziona un articolo', 'La categoria potra essere modificata dopo la selezione.', 'package')
          ) : categories.error ? (
            stateBlock(errorTitle(categories.error), 'Le categorie non possono essere caricate.', 'triangle-alert')
          ) : (
            <form className="settingsDetailForm" onSubmit={(event) => void submit(event)}>
              <div className="settingsArticleSummary">
                <span className="monoCell">{selectedArticle.article_code}</span>
                <strong>{selectedArticle.description || '-'}</strong>
              </div>
              <label className="field">
                <span>Categoria</span>
                <select
                  value={categoryDraft}
                  onChange={(event) => setCategoryDraft(event.target.value)}
                  disabled={categories.isLoading}
                >
                  {categoryOptions.map((item) => (
                    <option key={item.value} value={item.value}>{item.label}</option>
                  ))}
                </select>
              </label>
              <div className="settingsFormActions">
                <Button
                  type="submit"
                  leftIcon={<Icon name="check" />}
                  loading={mutations.setArticleCategory.isPending}
                  disabled={!dirty || categories.isLoading || categoryOptions.length === 0}
                >
                  Salva
                </Button>
              </div>
            </form>
          )}
        </section>
      </div>
    </section>
  );
}

export function DocumentTypesPage() {
  const { toast } = useToast();
  const documentTypes = useDocumentTypes();
  const mutations = useFornitoriMutations();
  const [selectedId, setSelectedId] = useState<DocumentTypeSelection>(null);
  const [name, setName] = useState('');
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);

  const documentTypeItems = useMemo(() => documentTypes.data ?? [], [documentTypes.data]);
  const selectedDocumentType = selectedId === 'new'
    ? null
    : documentTypeItems.find((item) => item.id === selectedId) ?? null;
  const editingDocumentType = typeof selectedId === 'number';
  const saving = mutations.createDocumentType.isPending || mutations.updateDocumentType.isPending;

  function selectDocumentType(item: DocumentType) {
    setSelectedId(item.id);
    setName(item.name);
    setConfirmDeleteOpen(false);
  }

  function startNewDocumentType() {
    setSelectedId('new');
    setName('');
    setConfirmDeleteOpen(false);
  }

  async function saveDocumentType(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      toast('Inserisci il nome tipo documento', 'warning');
      return;
    }

    try {
      const saved = editingDocumentType
        ? await mutations.updateDocumentType.mutateAsync({ id: selectedId, name: trimmed })
        : await mutations.createDocumentType.mutateAsync({ name: trimmed });
      setSelectedId(saved.id);
      setName(saved.name);
      toast('Tipo documento salvato', 'success');
    } catch (saveError) {
      toast(apiErrorMessage(saveError, 'Impossibile salvare il tipo documento.'), 'error');
    }
  }

  async function deleteDocumentType() {
    if (!selectedDocumentType) return;
    try {
      await mutations.deleteDocumentType.mutateAsync(selectedDocumentType.id);
      setConfirmDeleteOpen(false);
      setSelectedId(null);
      setName('');
      toast('Tipo documento eliminato', 'success');
    } catch (deleteError) {
      toast(apiErrorMessage(deleteError, 'Impossibile eliminare il tipo documento.'), 'error');
    }
  }

  return (
    <section className="settingsSection" aria-label="Tipi documento">
      <div className="settingsWorkspace">
        <section className="master panel settingsMasterPanel" aria-label="Tipi documento">
          <div className="settingsPanelHeader">
            <div>
              <h2>Tipi documento</h2>
              <span className="settingsPanelMeta">{countLabel(documentTypeItems.length, 'tipo documento', 'tipi documento')}</span>
            </div>
            <Button size="sm" leftIcon={<Icon name="plus" />} onClick={startNewDocumentType}>
              Nuovo tipo documento
            </Button>
          </div>

          {documentTypes.isLoading ? (
            <div className="settingsPanelBody">
              <Skeleton rows={8} />
            </div>
          ) : documentTypes.error ? (
            stateBlock(errorTitle(documentTypes.error), 'I tipi documento non possono essere caricati.', 'triangle-alert')
          ) : documentTypeItems.length === 0 ? (
            stateBlock('Nessun tipo documento', 'Aggiungi un tipo documento per usarlo nelle regole.', 'file-plus')
          ) : (
            <div className="settingsList">
              {documentTypeItems.map((item) => {
                const selected = selectedId === item.id;
                return (
                  <button
                    key={item.id}
                    type="button"
                    className={`settingsListRow ${selected ? 'selected' : ''}`}
                    aria-current={selected ? 'true' : undefined}
                    onClick={() => selectDocumentType(item)}
                  >
                    <span className="settingsListMain">
                      <strong>{item.name}</strong>
                    </span>
                    <Icon name="chevron-right" size={16} />
                  </button>
                );
              })}
            </div>
          )}
        </section>

        <section className="detail panel settingsDetailPanel" aria-label="Dettaglio tipo documento">
          <div className="settingsDetailHeader">
            <div>
              <h2>{selectedId === 'new' ? 'Nuovo tipo documento' : selectedDocumentType?.name ?? 'Tipi documento'}</h2>
              <span className="settingsPanelMeta">{selectedId ? 'Catalogo documentale' : 'Nessuna selezione'}</span>
            </div>
          </div>

          {selectedId === null ? (
            stateBlock('Seleziona un tipo documento', 'Scegli un tipo documento dalla lista oppure creane uno nuovo.', 'file-text')
          ) : (
            <form className="settingsDetailForm" onSubmit={(event) => void saveDocumentType(event)}>
              <label className="field">
                <span>Nome tipo documento</span>
                <input value={name} onChange={(event) => setName(event.target.value)} />
              </label>
              <div className="settingsFormActions">
                {selectedDocumentType ? (
                  <Button
                    type="button"
                    variant="danger"
                    leftIcon={<Icon name="trash" />}
                    onClick={() => setConfirmDeleteOpen(true)}
                  >
                    Elimina
                  </Button>
                ) : null}
                <Button
                  type="submit"
                  leftIcon={<Icon name="check" />}
                  disabled={saving}
                  loading={saving}
                >
                  Salva
                </Button>
              </div>
            </form>
          )}
        </section>
      </div>

      <Modal open={confirmDeleteOpen && Boolean(selectedDocumentType)} onClose={() => setConfirmDeleteOpen(false)} title="Elimina tipo documento" size="sm">
        <div className="settingsConfirm">
          <p>Il tipo documento selezionato verra rimosso dal catalogo.</p>
          <div className="modalActions">
            <Button variant="secondary" onClick={() => setConfirmDeleteOpen(false)}>Annulla</Button>
            <Button
              variant="danger"
              leftIcon={<Icon name="trash" />}
              loading={mutations.deleteDocumentType.isPending}
              onClick={() => void deleteDocumentType()}
            >
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function stateBlock(title: string, message: string, iconName: 'file-plus' | 'file-text' | 'package' | 'search' | 'triangle-alert') {
  return (
    <div className="emptyState">
      <span className="emptyIcon" aria-hidden="true">
        <Icon name={iconName} size={28} />
      </span>
      <p className="emptyTitle">{title}</p>
      <p className="emptyText">{message}</p>
    </div>
  );
}

function errorTitle(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Accesso non consentito';
    if (error.status === 503) return 'Servizio temporaneamente non disponibile';
  }
  return 'Dati non disponibili';
}

function apiErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
    return error.message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}

function categoryDocumentCounts(category: Category) {
  const counts = { required: 0, optional: 0 };
  for (const entry of category.document_types ?? []) {
    if (entry.required) counts.required += 1;
    else counts.optional += 1;
  }
  return counts;
}

function categoryDraft(category: Category) {
  const draft = { required: [] as number[], optional: [] as number[] };
  for (const entry of category.document_types ?? []) {
    const id = entry.document_type?.id;
    if (typeof id !== 'number') continue;
    if (entry.required) draft.required.push(id);
    else draft.optional.push(id);
  }
  return draft;
}

function compareCategoryByName(left: Category, right: Category) {
  const byName = left.name.localeCompare(right.name, 'it', { sensitivity: 'base' });
  return byName || left.id - right.id;
}

function compareDocumentTypeByName(left: DocumentType, right: DocumentType) {
  const byName = left.name.localeCompare(right.name, 'it', { sensitivity: 'base' });
  return byName || left.id - right.id;
}

function countLabel(count: number, singular: string, plural: string) {
  return count === 1 ? `1 ${singular}` : `${count} ${plural}`;
}

function normalizedQuery(value: string) {
  return value.toLocaleLowerCase('it').trim();
}

function matchesPaymentMethodQuery(code: string, description: string, query: string) {
  const term = normalizedQuery(query);
  if (!term) return true;
  return `${code} ${description}`.toLocaleLowerCase('it').includes(term);
}

function matchesArticleQuery(item: ArticleCategory, query: string) {
  const term = normalizedQuery(query);
  if (!term) return true;
  return [
    item.article_code,
    item.description ?? '',
    item.category_name ?? '',
  ].join(' ').toLocaleLowerCase('it').includes(term);
}

function categorySelectOptions(categories: Category[], selectedArticle: ArticleCategory | null) {
  const options = categories.map((item) => ({ value: String(item.id), label: item.name }));
  if (!selectedArticle) return options;
  const selectedValue = String(selectedArticle.category_id);
  if (options.some((item) => item.value === selectedValue)) return options;
  return [
    { value: selectedValue, label: selectedArticle.category_name || selectedValue },
    ...options,
  ];
}
