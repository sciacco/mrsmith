import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCategories,
  useCreateCategory,
  useUpdateCategory,
} from '../../api/queries';
import styles from './SettingsPage.module.css';

export function CategoriesPage() {
  const { toast } = useToast();
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<'create' | 'edit'>('create');
  const [editingId, setEditingId] = useState<number | null>(null);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [draft, setDraft] = useState({ name: '', color: '#231F20' });

  const { data, isLoading, error } = useCategories();
  const createCategory = useCreateCategory();
  const updateCategory = useUpdateCategory();

  const categories = data ?? [];

  function openCreate() {
    setModalMode('create');
    setDraft({ name: '', color: '#231F20' });
    setEditingId(null);
    setModalOpen(true);
  }

  function openEdit(id: number) {
    const cat = categories.find((c) => c.id === id);
    if (!cat) return;
    setModalMode('edit');
    setDraft({ name: cat.name, color: cat.color });
    setEditingId(id);
    setModalOpen(true);
  }

  async function handleSave() {
    if (!draft.name.trim()) {
      toast('Il nome categoria e obbligatorio', 'error');
      return;
    }

    try {
      if (modalMode === 'create') {
        await createCategory.mutateAsync({ name: draft.name.trim(), color: draft.color });
        toast('Categoria creata', 'success');
      } else if (editingId != null) {
        await updateCategory.mutateAsync({ id: editingId, name: draft.name.trim(), color: draft.color });
        toast('Categoria aggiornata', 'success');
      }
      setModalOpen(false);
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile salvare la categoria'), 'error');
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
      <header className={styles.pageHeader}>
        <div>
          <h1>Categorie</h1>
          <p className={styles.subtitle}>{categories.length} categorie</p>
        </div>
        <button type="button" className={styles.primaryButton} onClick={openCreate}>
          Nuova categoria
        </button>
      </header>

      <section className={styles.card}>
        <div className={styles.cardToolbar}>
          <button
            type="button"
            className={styles.secondaryButton}
            disabled={selectedId == null}
            onClick={() => { if (selectedId != null) openEdit(selectedId); }}
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
            <p className={styles.emptyTitle}>Impossibile caricare le categorie</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : categories.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M9.568 3H5.25A2.25 2.25 0 0 0 3 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 0 0 5.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 0 0 9.568 3Z" />
                <path d="M6 6h.008v.008H6V6Z" />
              </svg>
            </div>
            <p className={styles.emptyTitle}>Nessuna categoria</p>
            <p className={styles.emptyText}>Crea la prima categoria per iniziare.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Colore</th>
                  <th>Nome</th>
                  <th>Preview</th>
                </tr>
              </thead>
              <tbody>
                {categories.map((category, index) => (
                  <tr
                    key={category.id}
                    className={selectedId === category.id ? styles.rowSelected : ''}
                    style={{ animationDelay: `${index * 0.03}s` }}
                    onClick={() => setSelectedId(category.id)}
                    onDoubleClick={() => openEdit(category.id)}
                  >
                    <td>
                      <span className={styles.colorDot} style={{ background: toColorValue(category.color) }} />
                    </td>
                    <td>{category.name}</td>
                    <td>
                      <span className={styles.colorChip} style={{ backgroundColor: toColorValue(category.color) }}>
                        {category.name}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={modalMode === 'create' ? 'Nuova categoria' : 'Modifica categoria'}>
        <div className={styles.modalBody}>
          <label className={styles.field}>
            <span>Nome</span>
            <input value={draft.name} onChange={(e) => setDraft((c) => ({ ...c, name: e.target.value }))} placeholder="Nome categoria" />
          </label>
          <label className={styles.field}>
            <span>Colore</span>
            <div className={styles.colorField}>
              <input type="color" value={draft.color} onChange={(e) => setDraft((c) => ({ ...c, color: e.target.value }))} />
              <code className={styles.mono}>{draft.color}</code>
              <span className={styles.colorChip} style={{ backgroundColor: draft.color }}>{draft.name || 'Preview'}</span>
            </div>
          </label>
          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setModalOpen(false)}>Annulla</button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleSave()} disabled={createCategory.isPending || updateCategory.isPending}>Salva</button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') return message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}

function toColorValue(value: string) {
  return /^#[0-9A-Fa-f]{6}$/.test(value) ? value : '#231F20';
}
