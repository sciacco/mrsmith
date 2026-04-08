import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Skeleton, useToast } from '@mrsmith/ui';
import {
  useCategories,
  useCreateCategory,
  useUpdateCategory,
} from '../../api/queries';
import styles from './SettingsPage.module.css';

type Drafts = Record<number, { name: string; color: string }>;

export function CategoriesPage() {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<Drafts>({});
  const [newRow, setNewRow] = useState({ name: '', color: '#231F20' });

  const { data, isLoading, error } = useCategories();
  const createCategory = useCreateCategory();
  const updateCategory = useUpdateCategory();

  const categories = data ?? [];
  const dirtyIds = useMemo(
    () =>
      categories
        .filter((category) => {
          const draft = drafts[category.id];
          return draft != null && (
            draft.name.trim() !== category.name ||
            draft.color !== category.color
          );
        })
        .map((category) => category.id),
    [categories, drafts],
  );

  async function handleSave(id: number) {
    const category = categories.find((item) => item.id === id);
    const draft = drafts[id];
    if (!category || !draft) {
      return;
    }

    try {
      await updateCategory.mutateAsync({
        id,
        name: draft.name.trim(),
        color: draft.color,
      });
      setDrafts((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      toast('Categoria aggiornata', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile aggiornare la categoria'), 'error');
    }
  }

  async function handleCreate() {
    if (!newRow.name.trim()) {
      toast('Il nome categoria e obbligatorio', 'error');
      return;
    }

    try {
      await createCategory.mutateAsync({
        name: newRow.name.trim(),
        color: newRow.color,
      });
      setNewRow({ name: '', color: '#231F20' });
      toast('Categoria creata', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile creare la categoria'), 'error');
    }
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={6} />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Settings</p>
          <h1>Categorie</h1>
          <p className={styles.lead}>
            Mantieni allineate le categorie usate da kit e prodotti, con colore e naming coerenti.
          </p>
        </div>
        <div className={styles.highlight}>
          <span>Lookup condiviso</span>
          <strong>{categories.length}</strong>
          <p>categorie attive disponibili nelle viste di editing.</p>
        </div>
      </header>

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <div>
            <h2>Nuova categoria</h2>
            <p>Crea la voce e riusala subito negli editor di kit e prodotti.</p>
          </div>
        </div>

        <div className={styles.inlineForm}>
          <label className={styles.field}>
            <span>Nome</span>
            <input
              value={newRow.name}
              onChange={(event) => setNewRow((current) => ({ ...current, name: event.target.value }))}
              placeholder="Nuova categoria"
            />
          </label>
          <label className={styles.field}>
            <span>Colore</span>
            <div className={styles.colorField}>
              <input
                type="color"
                value={newRow.color}
                onChange={(event) => setNewRow((current) => ({ ...current, color: event.target.value }))}
              />
              <code>{newRow.color}</code>
            </div>
          </label>
          <button
            type="button"
            className={styles.primaryButton}
            onClick={() => void handleCreate()}
            disabled={createCategory.isPending}
          >
            Aggiungi
          </button>
        </div>
      </section>

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <div>
            <h2>Catalogo categorie</h2>
            <p>Modifica nome e colore direttamente in tabella, con salvataggio per singola riga.</p>
          </div>
          <div className={styles.meta}>{dirtyIds.length} righe con modifiche locali</div>
        </div>

        {error ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Impossibile caricare le categorie</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : categories.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nessuna categoria presente</p>
            <p className={styles.emptyText}>Crea la prima categoria per iniziare.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Colore</th>
                  <th>Preview</th>
                  <th className={styles.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {categories.map((category) => {
                  const draft = drafts[category.id] ?? category;
                  const safeColor = toColorValue(draft.color);
                  const dirty = dirtyIds.includes(category.id);

                  return (
                    <tr key={category.id}>
                      <td>
                        <input
                          value={draft.name}
                          onChange={(event) => {
                            const value = event.target.value;
                            setDrafts((current) => ({
                              ...current,
                              [category.id]: {
                                name: value,
                                color: current[category.id]?.color ?? category.color,
                              },
                            }));
                          }}
                        />
                      </td>
                      <td>
                        <div className={styles.colorField}>
                          <input
                            type="color"
                            value={safeColor}
                            onChange={(event) => {
                              const value = event.target.value;
                              setDrafts((current) => ({
                                ...current,
                                [category.id]: {
                                  name: current[category.id]?.name ?? category.name,
                                  color: value,
                                },
                              }));
                            }}
                          />
                          <code>{draft.color}</code>
                        </div>
                      </td>
                      <td>
                        <span
                          className={styles.colorChip}
                          style={{ backgroundColor: safeColor }}
                        >
                          {draft.name || 'Preview'}
                        </span>
                      </td>
                      <td className={styles.actionsCell}>
                        <button
                          type="button"
                          className={styles.secondaryButton}
                          disabled={!dirty || updateCategory.isPending}
                          onClick={() => void handleSave(category.id)}
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
    </section>
  );
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

function toColorValue(value: string) {
  return /^#[0-9A-Fa-f]{6}$/.test(value) ? value : '#231F20';
}
