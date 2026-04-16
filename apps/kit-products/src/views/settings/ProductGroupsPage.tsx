import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCreateProductGroup,
  useLanguages,
  useProductGroups,
  useUpdateProductGroup,
} from '../../api/queries';
import type {
  LanguageOption,
  ProductGroup,
  ProductGroupRenameConflict,
  ProductGroupTranslation,
} from '../../api/types';
import styles from './SettingsPage.module.css';

interface ProductGroupDraft {
  name: string;
  translations: ProductGroupTranslation[];
}

type ShortTouchedState = Record<string, boolean>;

export function ProductGroupsPage() {
  const { toast } = useToast();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<'create' | 'edit'>('create');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ProductGroupDraft>({ name: '', translations: [] });
  const [shortTouched, setShortTouched] = useState<ShortTouchedState>({});
  const [renameConflict, setRenameConflict] = useState<ProductGroupRenameConflict | null>(null);

  const {
    data: productGroups,
    isLoading: isGroupsLoading,
    error: groupsError,
  } = useProductGroups();
  const {
    data: languages,
    isLoading: isLanguagesLoading,
    error: languagesError,
  } = useLanguages();
  const createProductGroup = useCreateProductGroup();
  const updateProductGroup = useUpdateProductGroup();

  const groups = productGroups ?? [];
  const languageOptions = languages ?? [];
  const isLoading = isGroupsLoading || isLanguagesLoading;
  const error = groupsError ?? languagesError;
  const selectedGroup = useMemo(
    () => groups.find((group) => group.translation_uuid === selectedId) ?? null,
    [groups, selectedId],
  );
  const isSaving = createProductGroup.isPending || updateProductGroup.isPending;

  function openCreate() {
    setModalMode('create');
    setEditingId(null);
    setDraft({
      name: '',
      translations: buildEmptyTranslations(languageOptions),
    });
    setShortTouched({});
    setRenameConflict(null);
    setModalOpen(true);
  }

  function openEdit(translationUUID: string) {
    const group = groups.find((item) => item.translation_uuid === translationUUID);
    if (!group) return;

    setModalMode('edit');
    setEditingId(translationUUID);
    setDraft({
      name: group.name,
      translations: buildDraftTranslations(group, languageOptions),
    });
    setShortTouched(buildTouchedState(group, languageOptions));
    setRenameConflict(null);
    setModalOpen(true);
  }

  function handleNameChange(nextName: string) {
    setDraft((current) => ({
      ...current,
      name: nextName,
      translations: current.translations.map((translation) => (
        shortTouched[getLanguageKey(translation.language)]
          ? translation
          : { ...translation, short: nextName }
      )),
    }));
  }

  function handleShortChange(language: string, value: string) {
    const key = getLanguageKey(language);
    setShortTouched((current) => ({ ...current, [key]: true }));
    setDraft((current) => ({
      ...current,
      translations: current.translations.map((translation) => (
        getLanguageKey(translation.language) === key
          ? { ...translation, short: value }
          : translation
      )),
    }));
  }

  function handleLongChange(language: string, value: string) {
    const key = getLanguageKey(language);
    setDraft((current) => ({
      ...current,
      translations: current.translations.map((translation) => (
        getLanguageKey(translation.language) === key
          ? { ...translation, long: value }
          : translation
      )),
    }));
  }

  async function handleSave(confirmPropagation = false) {
    const name = draft.name.trim();
    if (!name) {
      toast('Il nome del raggruppamento e obbligatorio', 'error');
      return;
    }

    const translations = draft.translations.map((translation) => ({
      language: translation.language,
      short: translation.short.trim(),
      long: translation.long.trim(),
    }));

    if (translations.some((translation) => translation.short.length === 0)) {
      toast('Compila la descrizione breve per tutte le lingue', 'error');
      return;
    }

    try {
      if (modalMode === 'create') {
        const created = await createProductGroup.mutateAsync({ name, translations });
        setSelectedId(created.translation_uuid);
        toast('Raggruppamento creato', 'success');
      } else if (editingId != null) {
        const updated = await updateProductGroup.mutateAsync({
          translationUUID: editingId,
          name,
          translations,
          confirm_propagation: confirmPropagation,
        });
        setSelectedId(updated.translation_uuid);
        toast('Raggruppamento aggiornato', 'success');
      }

      setRenameConflict(null);
      setModalOpen(false);
    } catch (err) {
      const conflict = getRenameConflict(err);
      if (!confirmPropagation && conflict) {
        setRenameConflict(conflict);
        return;
      }
      toast(getErrorMessage(err, 'Impossibile salvare il raggruppamento'), 'error');
    }
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={6} />
      </section>
    );
  }

  if (languageOptions.length === 0) {
    return (
      <section className={styles.page}>
        <header className={styles.pageHeader}>
          <div>
            <h1>Raggruppamento Prodotti</h1>
            <p className={styles.subtitle}>Nessuna lingua configurata</p>
          </div>
        </header>
        <section className={styles.card}>
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 18.75A6.75 6.75 0 1 0 12 5.25a6.75 6.75 0 0 0 0 13.5Z" />
                <path d="M2.25 12h19.5M12 2.25c1.9 2.02 3 4.69 3 7.5s-1.1 5.48-3 7.5c-1.9-2.02-3-4.69-3-7.5s1.1-5.48 3-7.5Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Lingue non disponibili</p>
            <p className={styles.emptyText}>Configura almeno una lingua in Mistra per gestire i raggruppamenti.</p>
          </div>
        </section>
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1>Raggruppamento Prodotti</h1>
          <p className={styles.subtitle}>{groups.length} raggruppamenti</p>
        </div>
        <button type="button" className={styles.primaryButton} onClick={openCreate}>
          Nuovo raggruppamento
        </button>
      </header>

      <section className={styles.card}>
        <div className={styles.cardToolbar}>
          <button
            type="button"
            className={styles.secondaryButton}
            disabled={selectedGroup == null}
            onClick={() => {
              if (selectedGroup) openEdit(selectedGroup.translation_uuid);
            }}
          >
            Modifica
          </button>
        </div>

        {error ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Impossibile caricare i raggruppamenti</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : groups.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M4.5 6.75h15m-15 5.25h15m-15 5.25h15" />
                <path d="M8.25 3v18" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessun raggruppamento</p>
            <p className={styles.emptyText}>Crea il primo raggruppamento da usare nella composizione dei kit.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Usato in kit</th>
                  <th>Traduzioni</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((group, index) => (
                  <tr
                    key={group.translation_uuid}
                    className={selectedId === group.translation_uuid ? styles.rowSelected : ''}
                    style={{ animationDelay: `${index * 0.03}s` }}
                    onClick={() => setSelectedId(group.translation_uuid)}
                    onDoubleClick={() => openEdit(group.translation_uuid)}
                  >
                    <td>{group.name}</td>
                    <td><span className={styles.mono}>{group.usage_count}</span></td>
                    <td>
                      <span className={styles.translationSummary}>
                        {countCompleteTranslations(group, languageOptions)}/{languageOptions.length} lingue complete
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Modal
        open={modalOpen}
        onClose={() => {
          if (isSaving) return;
          setModalOpen(false);
          setRenameConflict(null);
        }}
        title={modalMode === 'create' ? 'Nuovo raggruppamento' : 'Modifica raggruppamento'}
        size="wide"
        dismissible={!isSaving}
      >
        <div className={styles.modalBody}>
          <label className={styles.field}>
            <span>Nome</span>
            <input
              value={draft.name}
              onChange={(event) => handleNameChange(event.target.value)}
              placeholder="Nome raggruppamento"
            />
          </label>

          <div className={styles.translationGrid}>
            {languageOptions.map((language) => {
              const translation = getDraftTranslation(draft.translations, language.iso);
              return (
                <section key={language.iso} className={styles.translationCard}>
                  <div className={styles.translationHeader}>
                    <strong>{language.name}</strong>
                    <span>{language.iso.toUpperCase()}</span>
                  </div>
                  <label className={styles.field}>
                    <span>Short</span>
                    <input
                      value={translation.short}
                      onChange={(event) => handleShortChange(language.iso, event.target.value)}
                      placeholder={`Short ${language.name}`}
                    />
                  </label>
                  <label className={styles.field}>
                    <span>Long</span>
                    <textarea
                      value={translation.long}
                      onChange={(event) => handleLongChange(language.iso, event.target.value)}
                      placeholder={`Descrizione estesa ${language.name}`}
                    />
                  </label>
                </section>
              );
            })}
          </div>

          <div className={styles.modalActions}>
            <button
              type="button"
              className={styles.secondaryButton}
              onClick={() => {
                setModalOpen(false);
                setRenameConflict(null);
              }}
              disabled={isSaving}
            >
              Annulla
            </button>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => void handleSave()}
              disabled={isSaving}
            >
              Salva
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={renameConflict != null}
        onClose={() => {
          if (isSaving) return;
          setRenameConflict(null);
        }}
        title="Conferma rinomina"
        dismissible={!isSaving}
      >
        <div className={styles.modalBody}>
          <div className={styles.confirmCopy}>
            <p>
              La modifica del nome aggiornera <strong>{renameConflict?.impacted_kit_products ?? 0}</strong>{' '}
              righe prodotto gia presenti nei kit che usano questo raggruppamento.
            </p>
            <p>Le quote storiche non verranno toccate e manterranno il nome salvato come snapshot.</p>
          </div>
          <div className={styles.modalActions}>
            <button
              type="button"
              className={styles.secondaryButton}
              onClick={() => setRenameConflict(null)}
              disabled={isSaving}
            >
              Annulla
            </button>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => void handleSave(true)}
              disabled={isSaving}
            >
              Conferma modifica
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function buildEmptyTranslations(languages: LanguageOption[]): ProductGroupTranslation[] {
  return languages.map((language) => ({
    language: language.iso,
    short: '',
    long: '',
  }));
}

function buildDraftTranslations(group: ProductGroup, languages: LanguageOption[]): ProductGroupTranslation[] {
  return languages.map((language) => {
    const existing = findTranslation(group.translations, language.iso);
    return {
      language: language.iso,
      short: existing?.short?.trim() || group.name,
      long: existing?.long ?? '',
    };
  });
}

function buildTouchedState(group: ProductGroup, languages: LanguageOption[]): ShortTouchedState {
  const state: ShortTouchedState = {};
  for (const language of languages) {
    const existing = findTranslation(group.translations, language.iso);
    state[getLanguageKey(language.iso)] = (existing?.short?.trim().length ?? 0) > 0;
  }
  return state;
}

function getDraftTranslation(translations: ProductGroupTranslation[], language: string): ProductGroupTranslation {
  return findTranslation(translations, language) ?? {
    language,
    short: '',
    long: '',
  };
}

function findTranslation(translations: ProductGroupTranslation[], language: string) {
  const key = getLanguageKey(language);
  return translations.find((translation) => getLanguageKey(translation.language) === key);
}

function countCompleteTranslations(group: ProductGroup, languages: LanguageOption[]) {
  return languages.reduce((count, language) => {
    const translation = findTranslation(group.translations, language.iso);
    return count + (translation?.short?.trim() ? 1 : 0);
  }, 0);
}

function getRenameConflict(error: unknown): ProductGroupRenameConflict | null {
  if (
    error instanceof ApiError &&
    typeof error.body === 'object' &&
    error.body != null &&
    'error' in error.body &&
    error.body.error === 'rename_confirmation_required'
  ) {
    const impacted = 'impacted_kit_products' in error.body ? error.body.impacted_kit_products : 0;
    const quotesUnchanged = 'quotes_unchanged' in error.body ? error.body.quotes_unchanged : false;
    if (typeof impacted === 'number' && typeof quotesUnchanged === 'boolean') {
      return {
        error: 'rename_confirmation_required',
        impacted_kit_products: impacted,
        quotes_unchanged: quotesUnchanged,
      };
    }
  }
  return null;
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') return message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}

function getLanguageKey(language: string) {
  return language.trim().toLowerCase();
}
