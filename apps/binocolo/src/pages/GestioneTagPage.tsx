import { Button, Icon, Modal, Skeleton, useToast, VisuallyHidden } from '@mrsmith/ui';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import type { MATag } from '../api/types';
import { maTagErrorMessage, useMATagCatalog, useMATagCatalogMutations } from '../hooks/useMATags';
import { errorLabel } from './ricerche/helpers';
import styles from './GestioneTagPage.module.css';

/**
 * Strumenti → Gestione tag (issue #198, slice 6). Il catalogo condiviso del
 * team con le uniche due azioni GLOBALI: rinomina (preserva l'UUID, quindi
 * associazioni e selezioni restano valide) ed eliminazione (tag rimosso dal
 * catalogo e da tutte le aziende; le aziende restano). La creazione non esiste
 * qui per scelta di prodotto: nasce dalla scheda azienda (AddCompanyTagModal);
 * la rimozione dalla singola azienda resta sulla chip in testata.
 *
 * Tutte le letture e le mutazioni passano da useMATags: useMATagCatalog per la
 * policy di freschezza (mount/focus), useMATagCatalogMutations per le
 * invalidazioni centrali di catalogo, overview, elenco, board e pipeline.
 */
export function GestioneTagPage() {
  const catalog = useMATagCatalog();
  const { rename, remove } = useMATagCatalogMutations();
  const { toast } = useToast();
  const navigate = useNavigate();

  // Una sola riga in modifica alla volta: il nome è bozza locale finché non
  // passa da Salva (con le stesse regole del backend: trim, non vuoto,
  // equivalenza case/space-insensitive).
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [renameError, setRenameError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<MATag | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  // Focus dopo una mutazione riuscita o un annullamento: i comandi coinvolti
  // vengono smontati e il focus cadrebbe su <body>. Il tag rimosso sparisce
  // dall'elenco solo dopo il refetch dell'invalidazione: l'effetto più sotto
  // consume il riferimento quando l'aggiornamento arriva. Come in scheda, il
  // focus viene mosso solo se è davvero caduto su <body>.
  const pendingFocusRef = useRef<{ kind: 'rename' | 'delete'; tagId: string; index: number } | null>(null);
  const listRef = useRef<HTMLUListElement | null>(null);
  const emptyActionRef = useRef<HTMLButtonElement | null>(null);

  // Catalogo alfabetico: il backend ordina già per nome, il sort client tiene
  // l'ordine stabile anche con i mock e le ritardi di rete.
  const rows = useMemo(
    () => [...(catalog.data ?? [])].sort((a, b) => a.name.localeCompare(b.name, 'it')),
    [catalog.data],
  );
  const busy = rename.isPending || remove.isPending;

  const startRename = (tag: MATag) => {
    setEditingId(tag.id);
    setDraft(tag.name);
    setRenameError(null);
  };

  const cancelRename = () => {
    if (!editingId) return;
    pendingFocusRef.current = { kind: 'rename', tagId: editingId, index: -1 };
    setEditingId(null);
    setDraft('');
    setRenameError(null);
  };

  const submitRename = async (event: FormEvent) => {
    event.preventDefault();
    if (!editingId) return;
    const trimmed = draft.trim();
    if (!trimmed) {
      setRenameError('Inserisci un nome per il tag.');
      return;
    }
    // Stessa regola dell'indice univoco del backend: il confronto ignora spazi
    // esterni e maiuscole («Cloud» e « cloud » sono lo stesso tag). Il 409 del
    // server resta coperto come rete di sicurezza per le corse concorrenti.
    const equivalent = rows.some(
      (tag) => tag.id !== editingId && tag.name.trim().toLowerCase() === trimmed.toLowerCase(),
    );
    if (equivalent) {
      setRenameError('Esiste già un tag con questo nome.');
      return;
    }
    setRenameError(null);
    const tagId = editingId;
    try {
      await rename.mutateAsync({ tagId, name: trimmed });
      toast('Tag rinominato.', 'success');
      pendingFocusRef.current = { kind: 'rename', tagId, index: -1 };
      setEditingId(null);
      setDraft('');
    } catch (error) {
      // Superficie persistente nella riga (UI-UX §14.3): resta finché il nome
      // non cambia o la modifica non viene annullata; Salva è il comando di
      // riprova.
      setRenameError(maTagErrorMessage(error));
    }
  };

  const openDelete = (tag: MATag) => {
    setDeleting(tag);
    setDeleteError(null);
  };

  const closeDelete = () => {
    setDeleting(null);
    setDeleteError(null);
  };

  const confirmDelete = async () => {
    if (!deleting) return;
    setDeleteError(null);
    const index = rows.findIndex((tag) => tag.id === deleting.id);
    try {
      await remove.mutateAsync(deleting.id);
      toast('Tag eliminato.', 'success');
      pendingFocusRef.current = { kind: 'delete', tagId: deleting.id, index };
      setDeleting(null);
    } catch (error) {
      // Il modal resta aperto con l'errore persistente: Elimina è la riprova.
      setDeleteError(maTagErrorMessage(error));
    }
  };

  // Riposizionamento del focus (uso da tastiera): vedi pendingFocusRef. La
  // decisione è RIMANDATA a un task: la chiusura del <dialog> nativo e il
  // ripristino del focus avvengono dopo il commit, quindi ricampionare
  // activeElement durante il commit leggerebbe il focus ancora dentro il
  // modal (e il guard su <body> salterebbe a torto, lasciandolo cadere).
  // `deleting` tra le dipendenze: la chiusura del modal è essa stessa il
  // momento in cui il focus può cadere.
  useEffect(() => {
    const pending = pendingFocusRef.current;
    if (!pending) return;
    // Rinomina: attende che la riga sia tornata in visualizzazione.
    if (pending.kind === 'rename' && editingId === pending.tagId) return;
    // Eliminazione: attende che il tag spariscano dal catalogo.
    if (pending.kind === 'delete' && rows.some((tag) => tag.id === pending.tagId)) return;
    const handle = window.setTimeout(() => {
      if (pendingFocusRef.current !== pending) return;
      pendingFocusRef.current = null;
      if (document.activeElement !== document.body) return; // l'utente lo ha già mosso
      if (pending.kind === 'rename') {
        listRef.current?.querySelector<HTMLButtonElement>(`button[data-tag-rename="${pending.tagId}"]`)?.focus();
        return;
      }
      // Riga eliminata: il comando Rinomina subentrato alla stessa posizione;
      // se l'elenco è finito, l'ultima riga; senza righe, l'azione del vuoto.
      const renameButtons = listRef.current
        ? Array.from(listRef.current.querySelectorAll<HTMLButtonElement>('button[data-tag-rename]'))
        : [];
      const target =
        pending.index >= 0 && pending.index < renameButtons.length
          ? renameButtons[pending.index]
          : renameButtons[renameButtons.length - 1];
      if (target && !target.disabled) {
        target.focus();
        return;
      }
      emptyActionRef.current?.focus();
    }, 0);
    return () => window.clearTimeout(handle);
  }, [rows, editingId, deleting]);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <span className={styles.eyebrow}>Binocolo</span>
        <h1>Gestione tag</h1>
        <p>Catalogo condiviso da tutto il team: rinominare un tag aggiorna il nome in tutte le aziende associate, eliminarlo lo rimuove dal catalogo e dalle aziende.</p>
      </header>

      {catalog.isLoading ? (
        <div className={styles.card}>
          <Skeleton rows={6} />
        </div>
      ) : catalog.isError ? (
        <div className={styles.errorBox} role="alert">
          <Icon name="triangle-alert" size={16} aria-hidden="true" />
          <span>{errorLabel(catalog.error)}</span>
          <Button size="sm" variant="secondary" onClick={() => void catalog.refetch()}>
            Riprova
          </Button>
        </div>
      ) : rows.length === 0 ? (
        <div className={styles.emptyState}>
          <span className={styles.emptyIcon} aria-hidden="true">
            <Icon name="archive" size={32} />
          </span>
          <h2>Nessun tag nel catalogo</h2>
          <p>I tag si creano dalla scheda di un’azienda: da lì entrano nel catalogo condiviso e diventano disponibili per tutto il team.</p>
          <Button ref={emptyActionRef} className={styles.emptyAction} onClick={() => navigate('/aziende')}>
            Vai alle aziende
          </Button>
        </div>
      ) : (
        <div className={styles.card}>
          <ul ref={listRef} className={styles.list} aria-label="Catalogo dei tag">
            {rows.map((tag) => (
              <li key={tag.id} className={styles.row}>
                {editingId === tag.id ? (
                  <form className={styles.renameForm} onSubmit={(event) => void submitRename(event)} noValidate>
                    <label htmlFor={`ma-rename-${tag.id}`} className={styles.fieldLabel}>
                      <span className={styles.labelText}>
                        Nome del tag
                        <span className={styles.requiredDot} aria-hidden="true" />
                        <VisuallyHidden>obbligatorio</VisuallyHidden>
                      </span>
                      <input
                        id={`ma-rename-${tag.id}`}
                        type="text"
                        value={draft}
                        autoFocus
                        required
                        readOnly={rename.isPending}
                        aria-invalid={renameError ? true : undefined}
                        aria-describedby={renameError ? `ma-rename-error-${tag.id}` : undefined}
                        onChange={(event) => {
                          setDraft(event.target.value);
                          setRenameError(null);
                        }}
                      />
                    </label>
                    {renameError ? (
                      <p id={`ma-rename-error-${tag.id}`} className={styles.formError} role="alert">
                        {renameError}
                      </p>
                    ) : null}
                    <div className={styles.rowActions}>
                      <Button type="submit" size="sm" loading={rename.isPending} disabled={remove.isPending}>
                        Salva
                      </Button>
                      <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={cancelRename}>
                        Annulla
                      </Button>
                    </div>
                  </form>
                ) : (
                  <>
                    <span className={styles.tagName}>{tag.name}</span>
                    <div className={styles.rowActions}>
                      <Button
                        size="sm"
                        variant="ghost"
                        data-tag-rename={tag.id}
                        disabled={busy}
                        onClick={() => startRename(tag)}
                      >
                        Rinomina
                      </Button>
                      <Button size="sm" variant="ghost" disabled={busy} onClick={() => openDelete(tag)}>
                        Elimina
                      </Button>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      <Modal
        open={deleting !== null}
        onClose={closeDelete}
        title="Elimina tag"
        size="sm"
        dismissible={!busy}
      >
        <div className={styles.modalBody}>
          <p>Stai eliminando il tag {deleting ? <strong>«{deleting.name}»</strong> : ''}.</p>
          <p className={styles.modalConsequence}>Il tag sarà rimosso dal catalogo e da tutte le aziende associate</p>
          {deleteError ? (
            <p className={styles.formError} role="alert">
              {deleteError}
            </p>
          ) : null}
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={closeDelete} disabled={busy}>
              Annulla
            </Button>
            <Button variant="danger" loading={remove.isPending} onClick={() => void confirmDelete()}>
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </main>
  );
}
